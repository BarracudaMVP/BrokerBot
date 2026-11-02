package flows

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/jackc/pgx/v5"

	"brokerbot/internal/bot/texts"
	"brokerbot/internal/bot/tg"
	"brokerbot/internal/db"
	"brokerbot/internal/domain"
	"brokerbot/internal/scam"
)

// FlowPayout — диалог сбора реквизитов продавца в личке.
const FlowPayout = "payout"

const stepDetails = "details"

type payoutDraft struct {
	DealID int64 `json:"deal_id"`
}

// askPayoutDetails просит продавца прислать реквизиты в личку. Состояние ключуется
// приватным чатом продавца, id которого совпадает с его tg_id.
func (f *Deals) askPayoutDetails(ctx context.Context, b *bot.Bot, deal *domain.Deal) {
	if deal.SellerID == 0 {
		return
	}
	data, err := json.Marshal(payoutDraft{DealID: deal.ID})
	if err == nil {
		err = f.db.SetState(ctx, deal.SellerID, deal.SellerID,
			db.State{Flow: FlowPayout, Step: stepDetails, Data: data})
	}
	if err != nil {
		slog.Error("не удалось запросить реквизиты", "deal_id", deal.ID, "err", err)
		return
	}
	tg.Reply(ctx, b, deal.SellerID, texts.AskPayoutDetails(deal), nil)
}

// feedPayout принимает реквизиты от продавца и сверяет их со скам-листом.
func (f *Deals) feedPayout(ctx context.Context, b *bot.Bot, m *models.Message, state *db.State) {
	var draft payoutDraft
	if err := json.Unmarshal(state.Data, &draft); err != nil {
		f.clear(ctx, m.From.ID, m.Chat.ID)
		tg.Reply(ctx, b, m.Chat.ID, texts.InternalErr, nil)
		return
	}

	deal, err := f.db.GetDeal(ctx, draft.DealID)
	if err != nil {
		f.clear(ctx, m.From.ID, m.Chat.ID)
		tg.Reply(ctx, b, m.Chat.ID, texts.DealNotFound, nil)
		return
	}
	if deal.Status != domain.StatusWaitingPayment {
		f.clear(ctx, m.From.ID, m.Chat.ID)
		tg.Reply(ctx, b, m.Chat.ID, texts.StageChanged, nil)
		return
	}

	details := strings.TrimSpace(m.Text)
	if details == "" {
		tg.Reply(ctx, b, m.Chat.ID, texts.AskPayoutDetailsHint, nil)
		return
	}

	entry, err := f.db.ScamFindAny(ctx, scam.Candidates(f.cfg.CardHashSecret, details))
	if err != nil && !errors.Is(err, db.ErrNotFound) {
		slog.Error("не удалось проверить реквизиты по скам-листу", "deal_id", deal.ID, "err", err)
		tg.Reply(ctx, b, m.Chat.ID, texts.InternalErr, nil)
		return
	}
	f.clear(ctx, m.From.ID, m.Chat.ID)

	if err == nil {
		f.freezeOnScamDetails(ctx, b, deal, entry)
		return
	}

	if err := f.db.SetPayoutDetails(ctx, deal.ID, details); err != nil {
		slog.Error("не удалось сохранить реквизиты", "deal_id", deal.ID, "err", err)
		tg.Reply(ctx, b, m.Chat.ID, texts.InternalErr, nil)
		return
	}
	tg.Reply(ctx, b, m.Chat.ID, texts.PayoutDetailsSaved, nil)
	tg.Reply(ctx, b, deal.GuarantorID, texts.PayoutDetailsForGuarantor(deal.ID, details), nil)
}

// freezeOnScamDetails замораживает сделку: реквизиты продавца в скам-листе.
// Переход делает бот (планировщик), поэтому actor — nil.
func (f *Deals) freezeOnScamDetails(ctx context.Context, b *bot.Bot, deal *domain.Deal, entry *db.ScamEntry) {
	reason := texts.DisputeReasonScamCard
	open := func(ctx context.Context, tx pgx.Tx, d *domain.Deal) error {
		return db.OpenDisputeTx(ctx, tx, d.ID, nil, reason)
	}
	frozen, err := domain.Transition(ctx, f.db, deal.ID, domain.StatusDispute, nil, reason, open)
	if err != nil {
		slog.Error("не удалось заморозить сделку", "deal_id", deal.ID, "err", err)
		return
	}

	// Продавцу — без подробностей о том, что именно совпало.
	tg.Reply(ctx, b, deal.SellerID, texts.PayoutDetailsScam, nil)
	f.enterStage(ctx, b, frozen, reason)
	f.notifyModerators(ctx, b, texts.ScamPayoutBlocked(deal.ID, entry.Type, entry.Display, entry.Reason))
}
