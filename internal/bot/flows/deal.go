// Package flows — пошаговые диалоги. Состояние живёт в БД (conversation_state),
// чтобы бот переживал перезапуск, не теряя наполовину заполненную сделку.
package flows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"brokerbot/internal/bot/keyboards"
	"brokerbot/internal/bot/texts"
	"brokerbot/internal/bot/tg"
	"brokerbot/internal/config"
	"brokerbot/internal/db"
	"brokerbot/internal/domain"
)

// FlowDeal — имя мастера создания сделки в conversation_state.
const FlowDeal = "deal"

const (
	stepEventName = "event_name"
	stepEventAt   = "event_at"
	stepTickets   = "tickets"
	stepAmount    = "amount"
	stepPercent   = "percent"
	stepPayer     = "payer"
)

// dealDraft — черновик сделки между шагами мастера (поле data в conversation_state).
type dealDraft struct {
	EventName string    `json:"event_name,omitempty"`
	EventAt   time.Time `json:"event_at,omitempty"`
	Tickets   int       `json:"tickets,omitempty"`
	Amount    int64     `json:"amount,omitempty"`
	Percent   int64     `json:"percent,omitempty"`
}

type Deals struct {
	db  *db.DB
	cfg *config.Config
}

func NewDeals(d *db.DB, cfg *config.Config) *Deals {
	return &Deals{db: d, cfg: cfg}
}

// Start запускает мастер по /deal. Права гаранта проверяет вызывающий.
func (f *Deals) Start(ctx context.Context, b *bot.Bot, m *models.Message) {
	if m.Chat.Type == models.ChatTypePrivate {
		tg.Reply(ctx, b, m.Chat.ID, texts.DealGroupOnly, nil)
		return
	}

	_, err := f.db.ActiveDealByChat(ctx, m.Chat.ID)
	switch {
	case err == nil:
		tg.Reply(ctx, b, m.Chat.ID, texts.DealAlreadyHere, nil)
		return
	case !errors.Is(err, domain.ErrDealNotFound):
		slog.Error("не удалось проверить сделки группы", "chat_id", m.Chat.ID, "err", err)
		tg.Reply(ctx, b, m.Chat.ID, texts.InternalErr, nil)
		return
	}

	if !f.save(ctx, b, m.From.ID, m.Chat.ID, stepEventName, dealDraft{}) {
		return
	}
	tg.Reply(ctx, b, m.Chat.ID, texts.AskEventName, nil)
}

// Feed отдаёт сообщение текущему шагу мастера. Возвращает false, если этот
// пользователь в этом чате мастер не ведёт — тогда сообщение обрабатывается обычно.
func (f *Deals) Feed(ctx context.Context, b *bot.Bot, m *models.Message) bool {
	state, err := f.db.GetState(ctx, m.From.ID, m.Chat.ID)
	if errors.Is(err, db.ErrNotFound) {
		return false
	}
	if err != nil {
		slog.Error("не удалось прочитать состояние диалога", "tg_id", m.From.ID, "err", err)
		return false
	}
	switch state.Flow {
	case FlowDeal:
	case FlowPayout:
		f.feedPayout(ctx, b, m, state)
		return true
	default:
		return false
	}

	if m.Text == "/cancel" {
		f.clear(ctx, m.From.ID, m.Chat.ID)
		tg.Reply(ctx, b, m.Chat.ID, texts.DealWizardCancel, nil)
		return true
	}

	var draft dealDraft
	if err := json.Unmarshal(state.Data, &draft); err != nil {
		slog.Error("битый черновик сделки", "tg_id", m.From.ID, "err", err)
		f.clear(ctx, m.From.ID, m.Chat.ID)
		tg.Reply(ctx, b, m.Chat.ID, texts.InternalErr, nil)
		return true
	}

	next, prompt, err := f.step(state.Step, m.Text, &draft)
	if err != nil {
		tg.Reply(ctx, b, m.Chat.ID, texts.WizardStepError(err), nil)
		return true
	}

	if f.save(ctx, b, m.From.ID, m.Chat.ID, next, draft) {
		var markup models.ReplyMarkup
		if next == stepPayer {
			markup = keyboards.CommissionPayer()
		}
		tg.Reply(ctx, b, m.Chat.ID, prompt, markup)
	}
	return true
}

// step применяет ввод к черновику и возвращает следующий шаг с его вопросом.
// При ошибке черновик не меняется: парсим в промежуточную переменную и только
// потом записываем — иначе неудачный ввод затирал бы уже введённое значение.
func (f *Deals) step(step, input string, draft *dealDraft) (next, prompt string, err error) {
	switch step {
	case stepEventName:
		if input == "" {
			return "", "", fmt.Errorf("название не может быть пустым")
		}
		draft.EventName = input
		return stepEventAt, texts.AskEventAt, nil

	case stepEventAt:
		at, err := domain.ParseEventTime(input, f.cfg.Location)
		if err != nil {
			return "", "", err
		}
		draft.EventAt = at
		return stepTickets, texts.AskTickets, nil

	case stepTickets:
		n, err := domain.ParseCount(input)
		if err != nil {
			return "", "", err
		}
		draft.Tickets = n
		return stepAmount, texts.AskAmount, nil

	case stepAmount:
		amount, err := domain.ParseAmount(input)
		if err != nil {
			return "", "", err
		}
		draft.Amount = amount
		return stepPercent, texts.AskPercent, nil

	case stepPercent:
		pct, err := domain.ParsePercent(input)
		if err != nil {
			return "", "", err
		}
		draft.Percent = pct
		return stepPayer, texts.AskPayer, nil

	case stepPayer:
		// Последний шаг — только кнопкой, текст тут не ждём.
		return stepPayer, texts.AskPayer, nil
	}
	return "", "", fmt.Errorf("неизвестный шаг %s", step)
}

// Payer завершает мастер: считает комиссию, создаёт сделку и выдаёт invite-ссылки.
func (f *Deals) Payer(ctx context.Context, b *bot.Bot, cq *models.CallbackQuery, payer string) {
	msg := cq.Message.Message
	if msg == nil {
		tg.Answer(ctx, b, cq.ID, texts.InternalErr)
		return
	}
	chatID := msg.Chat.ID

	state, err := f.db.GetState(ctx, cq.From.ID, chatID)
	if err != nil || state.Flow != FlowDeal || state.Step != stepPayer {
		tg.Answer(ctx, b, cq.ID, texts.NoRights)
		return
	}
	var draft dealDraft
	if err := json.Unmarshal(state.Data, &draft); err != nil {
		tg.Answer(ctx, b, cq.ID, texts.InternalErr)
		return
	}
	tg.Answer(ctx, b, cq.ID, "")

	commission, buyerPays, sellerReceives := domain.Commission(draft.Amount, draft.Percent, payer)
	deal, err := f.db.CreateDeal(ctx, &domain.Deal{
		ChatID:           chatID,
		GuarantorID:      cq.From.ID,
		EventName:        draft.EventName,
		EventAt:          draft.EventAt,
		TicketsCount:     draft.Tickets,
		Currency:         domain.DefaultCurrency,
		Amount:           draft.Amount,
		CommissionPct:    draft.Percent,
		CommissionPayer:  payer,
		CommissionAmount: commission,
		BuyerPays:        buyerPays,
		SellerReceives:   sellerReceives,
	})
	if err != nil {
		slog.Error("не удалось создать сделку", "chat_id", chatID, "err", err)
		tg.Reply(ctx, b, chatID, texts.InternalErr, nil)
		return
	}
	f.clear(ctx, cq.From.ID, chatID)

	actor := &domain.Actor{TgID: cq.From.ID, Role: domain.RoleGuarantor}

	sellerLink, buyerLink, err := f.inviteLinks(ctx, b, deal)
	if err != nil {
		// Без ссылок сделка бесполезна и блокирует группу — отменяем, чтобы можно
		// было повторить /deal.
		slog.Error("не удалось создать invite-ссылки", "deal_id", deal.ID, "err", err)
		if _, err := domain.Transition(ctx, f.db, deal.ID, domain.StatusCancelled, actor,
			"не удалось создать invite-ссылки", nil); err != nil {
			slog.Error("не удалось отменить сделку", "deal_id", deal.ID, "err", err)
		}
		tg.Reply(ctx, b, chatID, texts.InviteLinksFailed, nil)
		return
	}
	if err := f.db.SetInviteLinks(ctx, deal.ID, sellerLink, buyerLink); err != nil {
		slog.Error("не удалось сохранить invite-ссылки", "deal_id", deal.ID, "err", err)
		tg.Reply(ctx, b, chatID, texts.InternalErr, nil)
		return
	}

	if _, err := domain.Transition(ctx, f.db, deal.ID, domain.StatusWaitingMembers, actor, "", nil); err != nil {
		slog.Error("не удалось перевести сделку в ожидание участников", "deal_id", deal.ID, "err", err)
		tg.Reply(ctx, b, chatID, texts.InternalErr, nil)
		return
	}

	deal.Status = domain.StatusWaitingMembers
	tg.Reply(ctx, b, chatID, texts.DealCard(deal, f.cfg.Location, "", ""), nil)
	tg.Reply(ctx, b, chatID, texts.InviteLinks(sellerLink, buyerLink), nil)
}

// inviteLinks создаёт две ссылки с заявкой на вступление — по ссылке бот понимает роль.
func (f *Deals) inviteLinks(ctx context.Context, b *bot.Bot, deal *domain.Deal) (seller, buyer string, err error) {
	for _, l := range []struct {
		name string
		out  *string
	}{
		{fmt.Sprintf("Продавец #%d", deal.ID), &seller},
		{fmt.Sprintf("Покупатель #%d", deal.ID), &buyer},
	} {
		link, err := b.CreateChatInviteLink(ctx, &bot.CreateChatInviteLinkParams{
			ChatID:             deal.ChatID,
			Name:               l.name,
			CreatesJoinRequest: true,
		})
		if err != nil {
			return "", "", err
		}
		*l.out = link.InviteLink
	}
	return seller, buyer, nil
}

func (f *Deals) save(ctx context.Context, b *bot.Bot, userID, chatID int64, step string, draft dealDraft) bool {
	data, err := json.Marshal(draft)
	if err == nil {
		err = f.db.SetState(ctx, userID, chatID, db.State{Flow: FlowDeal, Step: step, Data: data})
	}
	if err != nil {
		slog.Error("не удалось сохранить состояние мастера", "tg_id", userID, "err", err)
		tg.Reply(ctx, b, chatID, texts.InternalErr, nil)
		return false
	}
	return true
}

func (f *Deals) clear(ctx context.Context, userID, chatID int64) {
	if err := f.db.ClearState(ctx, userID, chatID); err != nil {
		slog.Error("не удалось очистить состояние мастера", "tg_id", userID, "err", err)
	}
}
