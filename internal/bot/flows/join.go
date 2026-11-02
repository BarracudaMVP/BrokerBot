package flows

import (
	"context"
	"errors"
	"log/slog"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"brokerbot/internal/bot/keyboards"
	"brokerbot/internal/bot/texts"
	"brokerbot/internal/bot/tg"
	"brokerbot/internal/db"
	"brokerbot/internal/domain"
)

// JoinRequest обрабатывает заявку на вступление в группу сделки: роль определяется
// по ссылке, по которой пришёл человек.
func (f *Deals) JoinRequest(ctx context.Context, b *bot.Bot, r *models.ChatJoinRequest) {
	chatID := r.Chat.ID

	deal, err := f.db.ActiveDealByChat(ctx, chatID)
	if err != nil {
		if !errors.Is(err, domain.ErrDealNotFound) {
			slog.Error("не удалось найти сделку группы", "chat_id", chatID, "err", err)
		}
		f.decline(ctx, b, chatID, r.From.ID, texts.JoinNoDeal)
		return
	}

	role := dealRoleByLink(deal, r.InviteLink)
	if role == "" {
		f.decline(ctx, b, chatID, r.From.ID, texts.JoinUnknownRef)
		return
	}

	user, err := f.db.GetUser(ctx, r.From.ID)
	if err != nil && !errors.Is(err, db.ErrNotFound) {
		slog.Error("не удалось получить пользователя", "tg_id", r.From.ID, "err", err)
		f.decline(ctx, b, chatID, r.From.ID, texts.InternalErr)
		return
	}
	if err != nil || !user.IsRegistered || user.IsBlocked {
		f.decline(ctx, b, chatID, r.From.ID, texts.JoinNotRegistered)
		return
	}

	entry, err := f.db.ScamCheckUser(ctx, user.TgID, user.Phone)
	if err == nil {
		// Причину совпадения в группу не пишем — только модераторам.
		f.decline(ctx, b, chatID, r.From.ID, texts.JoinScam)
		f.notifyModerators(ctx, b, texts.ScamJoinBlocked(deal.ID, userRef(user), entry.Type, entry.Reason))
		return
	}
	if !errors.Is(err, db.ErrNotFound) {
		slog.Error("не удалось проверить скам-лист", "tg_id", user.TgID, "err", err)
		f.decline(ctx, b, chatID, r.From.ID, texts.InternalErr)
		return
	}

	claimed, err := f.db.ClaimParty(ctx, deal.ID, role, user.TgID)
	if err != nil {
		slog.Error("не удалось занять роль в сделке", "deal_id", deal.ID, "err", err)
		f.decline(ctx, b, chatID, r.From.ID, texts.InternalErr)
		return
	}
	if !claimed {
		f.decline(ctx, b, chatID, r.From.ID, texts.JoinRoleTaken)
		return
	}

	if _, err := b.ApproveChatJoinRequest(ctx, &bot.ApproveChatJoinRequestParams{
		ChatID: chatID, UserID: user.TgID,
	}); err != nil {
		slog.Error("не удалось одобрить заявку", "tg_id", user.TgID, "err", err)
		return
	}

	// Ссылка одноразовая по смыслу: роль занята, больше по ней заходить нельзя.
	if _, err := b.RevokeChatInviteLink(ctx, &bot.RevokeChatInviteLinkParams{
		ChatID: chatID, InviteLink: r.InviteLink.InviteLink,
	}); err != nil {
		slog.Warn("не удалось отозвать invite-ссылку", "deal_id", deal.ID, "err", err)
	}

	tg.Reply(ctx, b, chatID, texts.JoinApproved(role, userRef(user)), nil)
	if rating := user.SellerRating + user.BuyerRating; rating < 0 {
		tg.Reply(ctx, b, deal.GuarantorID, texts.WarnNegativeRating(role, userRef(user), rating), nil)
	}

	f.publishCardIfReady(ctx, b, deal.ID)
}

// publishCardIfReady публикует карточку с кнопкой подтверждения, когда обе стороны вступили.
func (f *Deals) publishCardIfReady(ctx context.Context, b *bot.Bot, dealID int64) {
	deal, err := f.db.GetDeal(ctx, dealID)
	if err != nil {
		slog.Error("не удалось перечитать сделку", "deal_id", dealID, "err", err)
		return
	}
	if deal.SellerID == 0 || deal.BuyerID == 0 {
		return
	}
	seller, buyer := f.refs(ctx, deal)
	tg.Reply(ctx, b, deal.ChatID,
		texts.DealCardPrompt(deal, f.cfg.Location, seller, buyer), keyboards.ForStatus(deal))
}

// Confirm — нажатие «Подтверждаю» в карточке сделки.
func (f *Deals) Confirm(ctx context.Context, b *bot.Bot, cq *models.CallbackQuery, dealID int64) {
	deal, err := f.db.GetDeal(ctx, dealID)
	if err != nil {
		tg.Answer(ctx, b, cq.ID, texts.DealNotFound)
		return
	}

	// Кнопку видят все в группе — нажать может только сторона сделки.
	role := deal.Party(cq.From.ID)
	if role == "" {
		tg.Answer(ctx, b, cq.ID, texts.ConfirmNotParty)
		return
	}
	if deal.Status != domain.StatusWaitingMembers {
		tg.Answer(ctx, b, cq.ID, texts.ConfirmAlreadyDone)
		return
	}
	if (role == domain.PayerSeller && deal.SellerConfirmed) ||
		(role == domain.PayerBuyer && deal.BuyerConfirmed) {
		tg.Answer(ctx, b, cq.ID, texts.ConfirmAlreadyDone)
		return
	}

	sellerOK, buyerOK, err := f.db.Confirm(ctx, dealID, role)
	if err != nil {
		slog.Error("не удалось сохранить подтверждение", "deal_id", dealID, "err", err)
		tg.Answer(ctx, b, cq.ID, texts.InternalErr)
		return
	}
	tg.Answer(ctx, b, cq.ID, "")

	if !sellerOK || !buyerOK {
		tg.Reply(ctx, b, deal.ChatID, texts.ConfirmWaitOther, nil)
		return
	}

	actor := &domain.Actor{TgID: cq.From.ID, Role: domain.RoleUser}
	started, err := domain.Transition(ctx, f.db, dealID, domain.StatusWaitingPayment, actor, "", nil)
	if err != nil {
		if errors.Is(err, domain.ErrAlreadyInStatus) {
			return // вторая сторона успела нажать первой
		}
		slog.Error("не удалось перевести сделку в ожидание оплаты", "deal_id", dealID, "err", err)
		tg.Reply(ctx, b, deal.ChatID, texts.InternalErr, nil)
		return
	}
	tg.Reply(ctx, b, deal.ChatID, texts.ConfirmBothDone, nil)
	f.enterStage(ctx, b, started, "")
}

// dealRoleByLink определяет роль по invite-ссылке, по которой пришла заявка.
func dealRoleByLink(deal *domain.Deal, link *models.ChatInviteLink) string {
	// Пустая ссылка не должна совпасть с ещё не выданной ссылкой сделки.
	if link == nil || link.InviteLink == "" {
		return ""
	}
	switch link.InviteLink {
	case deal.SellerInviteLink:
		return domain.PayerSeller
	case deal.BuyerInviteLink:
		return domain.PayerBuyer
	}
	return ""
}

func (f *Deals) decline(ctx context.Context, b *bot.Bot, chatID, userID int64, reason string) {
	if _, err := b.DeclineChatJoinRequest(ctx, &bot.DeclineChatJoinRequestParams{
		ChatID: chatID, UserID: userID,
	}); err != nil {
		slog.Warn("не удалось отклонить заявку", "tg_id", userID, "err", err)
	}
	tg.Reply(ctx, b, chatID, reason, nil)
}

func (f *Deals) notifyModerators(ctx context.Context, b *bot.Bot, text string) {
	ids, err := f.db.Moderators(ctx)
	if err != nil {
		slog.Error("не удалось получить список модераторов", "err", err)
		return
	}
	tg.Broadcast(ctx, b, ids, text)
}

// refs — подписи продавца и покупателя для карточки сделки.
func (f *Deals) refs(ctx context.Context, deal *domain.Deal) (seller, buyer string) {
	for _, p := range []struct {
		id  int64
		out *string
	}{{deal.SellerID, &seller}, {deal.BuyerID, &buyer}} {
		if p.id == 0 {
			continue
		}
		u, err := f.db.GetUser(ctx, p.id)
		if err != nil {
			continue
		}
		*p.out = userRef(u)
	}
	return seller, buyer
}

func userRef(u *db.User) string {
	return texts.UserRef(u.FirstName, u.LastName, u.Username, u.TgID)
}
