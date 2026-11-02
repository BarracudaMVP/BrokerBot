package flows

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/jackc/pgx/v5"

	"brokerbot/internal/bot/keyboards"
	"brokerbot/internal/bot/texts"
	"brokerbot/internal/bot/tg"
	"brokerbot/internal/db"
	"brokerbot/internal/domain"
)

// errEventNotStarted — покупатель нажал «Прошёл» до начала мероприятия.
var errEventNotStarted = errors.New("мероприятие ещё не началось")

// Paid — гарант подтвердил получение денег.
func (f *Deals) Paid(ctx context.Context, b *bot.Bot, cq *models.CallbackQuery, dealID int64) {
	f.act(ctx, b, cq, dealID, domain.StatusPaid, "оплата получена", nil)
}

// TicketsGot — покупатель подтвердил получение билетов.
func (f *Deals) TicketsGot(ctx context.Context, b *bot.Bot, cq *models.CallbackQuery, dealID int64) {
	f.act(ctx, b, cq, dealID, domain.StatusTicketsSent, "билеты получены", nil)
}

// TicketsAll — продавец сказал, что отправил всё. Статус не меняется: сделку
// двигает покупатель, когда проверит билеты.
func (f *Deals) TicketsAll(ctx context.Context, b *bot.Bot, cq *models.CallbackQuery, dealID int64) {
	deal, err := f.db.GetDeal(ctx, dealID)
	if err != nil {
		tg.Answer(ctx, b, cq.ID, texts.DealNotFound)
		return
	}
	if cq.From.ID != deal.SellerID {
		tg.Answer(ctx, b, cq.ID, texts.OnlySeller)
		return
	}
	if deal.Status != domain.StatusWaitingTickets {
		tg.Answer(ctx, b, cq.ID, texts.StageChanged)
		return
	}
	tg.Answer(ctx, b, cq.ID, "")
	tg.Reply(ctx, b, deal.ChatID, texts.TicketsAllSent, keyboards.ForStatus(deal))
}

// EntryOK — покупатель подтвердил, что прошёл на мероприятие.
func (f *Deals) EntryOK(ctx context.Context, b *bot.Bot, cq *models.CallbackQuery, dealID int64) {
	// Кнопка приходит заранее, но работает только после начала мероприятия —
	// иначе покупателя можно уговорить отпустить деньги до прохода.
	notBefore := func(ctx context.Context, _ pgx.Tx, d *domain.Deal) error {
		if time.Now().Before(d.EventAt) {
			return errEventNotStarted
		}
		return nil
	}
	f.act(ctx, b, cq, dealID, domain.StatusEntryOK, "проход подтверждён", notBefore)
}

// EntryFail — покупателя не пустили: открываем спор.
func (f *Deals) EntryFail(ctx context.Context, b *bot.Bot, cq *models.CallbackQuery, dealID int64) {
	f.openDispute(ctx, b, cq, dealID, texts.DisputeReasonEntryFail)
}

// OpenDispute — кнопка «Открыть спор» в группе.
func (f *Deals) OpenDispute(ctx context.Context, b *bot.Bot, cq *models.CallbackQuery, dealID int64) {
	f.openDispute(ctx, b, cq, dealID, texts.DisputeReasonManual)
}

func (f *Deals) openDispute(ctx context.Context, b *bot.Bot, cq *models.CallbackQuery,
	dealID int64, reason string) {

	opener := cq.From.ID
	open := func(ctx context.Context, tx pgx.Tx, d *domain.Deal) error {
		return db.OpenDisputeTx(ctx, tx, d.ID, &opener, reason)
	}
	deal := f.act(ctx, b, cq, dealID, domain.StatusDispute, reason, open)
	if deal == nil {
		return
	}
	by := texts.UserRef(cq.From.FirstName, cq.From.LastName, cq.From.Username, cq.From.ID)
	f.notifyModerators(ctx, b, texts.DisputeForModerators(deal.ID, by, reason))
}

// PayoutDone — гарант выплатил продавцу. Если по сделке был спор, она закрывается
// как решённая модератором, иначе — как успешная.
func (f *Deals) PayoutDone(ctx context.Context, b *bot.Bot, cq *models.CallbackQuery, dealID int64) {
	disputed, err := f.db.HasDispute(ctx, dealID)
	if err != nil {
		slog.Error("не удалось проверить споры сделки", "deal_id", dealID, "err", err)
		tg.Answer(ctx, b, cq.ID, texts.InternalErr)
		return
	}
	to := domain.StatusClosed
	if disputed {
		to = domain.StatusResolved
	}
	f.act(ctx, b, cq, dealID, to, "выплачено продавцу", nil)
}

// RefundDone — гарант вернул деньги покупателю по решению модератора.
func (f *Deals) RefundDone(ctx context.Context, b *bot.Bot, cq *models.CallbackQuery, dealID int64) {
	f.act(ctx, b, cq, dealID, domain.StatusResolved, "возврат покупателю сделан", nil)
}

// act выполняет переход по нажатию кнопки. Права не проверяются здесь: кто какой
// переход может сделать, решает таблица переходов в domain.
func (f *Deals) act(ctx context.Context, b *bot.Bot, cq *models.CallbackQuery, dealID int64,
	to, comment string, extra func(context.Context, pgx.Tx, *domain.Deal) error) *domain.Deal {

	role := domain.RoleUser
	if u, err := f.db.GetUser(ctx, cq.From.ID); err == nil {
		role = u.Role
	}

	deal, err := domain.Transition(ctx, f.db, dealID, to,
		&domain.Actor{TgID: cq.From.ID, Role: role}, comment, extra)
	switch {
	case errors.Is(err, errEventNotStarted):
		tg.Answer(ctx, b, cq.ID, texts.EntryTooEarly)
		return nil
	case errors.Is(err, domain.ErrDealNotFound):
		tg.Answer(ctx, b, cq.ID, texts.DealNotFound)
		return nil
	case errors.Is(err, domain.ErrForbidden):
		tg.Answer(ctx, b, cq.ID, texts.NoRights)
		return nil
	case errors.Is(err, domain.ErrAlreadyInStatus), errors.Is(err, domain.ErrBadTransition):
		tg.Answer(ctx, b, cq.ID, texts.StageChanged)
		return nil
	case err != nil:
		slog.Error("переход статуса не удался", "deal_id", dealID, "to", to, "err", err)
		tg.Answer(ctx, b, cq.ID, texts.InternalErr)
		return nil
	}

	tg.Answer(ctx, b, cq.ID, "")
	f.enterStage(ctx, b, deal, comment)
	return deal
}

// advance — автоматический переход, который делает сам бот (actor = планировщик).
func (f *Deals) advance(ctx context.Context, b *bot.Bot, dealID int64, to string) {
	deal, err := domain.Transition(ctx, f.db, dealID, to, nil, "автоматически", nil)
	if err != nil {
		slog.Error("автоматический переход не удался", "deal_id", dealID, "to", to, "err", err)
		return
	}
	f.enterStage(ctx, b, deal, "")
}

// enterStage объявляет новый этап в группе и запускает его побочные действия:
// личные запросы сторонам и автоматические переходы транзитных статусов.
// reason нужен только спору — это причина, с которой его открыли.
func (f *Deals) enterStage(ctx context.Context, b *bot.Bot, deal *domain.Deal, reason string) {
	switch deal.Status {
	case domain.StatusWaitingPayment:
		tg.Reply(ctx, b, deal.ChatID, texts.StageWaitingPayment(deal), keyboards.ForStatus(deal))
		tg.Reply(ctx, b, deal.GuarantorID, texts.GuarantorWaitingPayment(deal.ID), nil)
		f.askPayoutDetails(ctx, b, deal)

	case domain.StatusPaid:
		f.advance(ctx, b, deal.ID, domain.StatusWaitingTickets)

	case domain.StatusWaitingTickets:
		tg.Reply(ctx, b, deal.ChatID, texts.StageWaitingTickets(deal), keyboards.ForStatus(deal))

	case domain.StatusTicketsSent:
		f.advance(ctx, b, deal.ID, domain.StatusWaitingEntry)

	case domain.StatusWaitingEntry:
		tg.Reply(ctx, b, deal.ChatID, texts.StageWaitingEntry(deal, f.cfg.Location), keyboards.ForStatus(deal))
		tg.Reply(ctx, b, deal.BuyerID, texts.EntryPrompt(deal, f.cfg.Location), keyboards.Entry(deal.ID))

	case domain.StatusEntryOK:
		f.advance(ctx, b, deal.ID, domain.StatusPayoutPending)

	case domain.StatusPayoutPending:
		tg.Reply(ctx, b, deal.ChatID, texts.StagePayoutPending(deal), keyboards.ForStatus(deal))
		tg.Reply(ctx, b, deal.GuarantorID, texts.PayoutRequest(deal), nil)

	case domain.StatusDispute:
		if reason == "" {
			reason = texts.DisputeReasonManual
		}
		tg.Reply(ctx, b, deal.ChatID, texts.DisputeOpened(deal.ID, reason), nil)

	case domain.StatusClosed:
		tg.Reply(ctx, b, deal.ChatID, texts.DealClosed(deal), nil)

	case domain.StatusResolved:
		tg.Reply(ctx, b, deal.ChatID, texts.DealResolved(deal.ID), nil)

	case domain.StatusCancelled:
		tg.Reply(ctx, b, deal.ChatID, texts.DealCancelled(deal.ID), nil)
	}
}

// OnGroupFile записывает вложение как файл сделки, если оно уместно на текущем этапе.
func (f *Deals) OnGroupFile(ctx context.Context, b *bot.Bot, m *models.Message) {
	file := tg.FileOf(m)
	if file == nil {
		return
	}
	deal, err := f.db.ActiveDealByChat(ctx, m.Chat.ID)
	if err != nil {
		return
	}

	kind := fileKind(deal, m.From.ID)
	if kind == "" {
		return
	}
	n, err := f.db.AddDealFile(ctx, db.DealFile{
		DealID: deal.ID, Kind: kind, FileID: file.FileID, FileUniqueID: file.UniqueID,
		FileName: file.Name, MimeType: file.MimeType, UploadedBy: m.From.ID,
	})
	if err != nil {
		slog.Error("не удалось записать файл сделки", "deal_id", deal.ID, "err", err)
		return
	}

	switch kind {
	case db.FileTicket:
		tg.Reply(ctx, b, deal.ChatID, texts.TicketRecorded(n, deal.TicketsCount),
			keyboards.ForStatus(deal))
	case db.FilePaymentProof:
		tg.Reply(ctx, b, deal.ChatID, texts.PaymentProofRecorded(deal.ID), nil)
	}
}

// fileKind решает, чем считать вложение: зависит от этапа и от того, кто его прислал.
func fileKind(deal *domain.Deal, from int64) string {
	switch deal.Status {
	case domain.StatusWaitingPayment:
		if from == deal.BuyerID {
			return db.FilePaymentProof
		}
	case domain.StatusWaitingTickets:
		if from == deal.SellerID {
			return db.FileTicket
		}
	case domain.StatusDispute:
		if deal.Party(from) != "" || from == deal.GuarantorID {
			return db.FileDisputeProof
		}
	}
	return ""
}
