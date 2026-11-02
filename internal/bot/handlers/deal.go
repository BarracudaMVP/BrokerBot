package handlers

import (
	"context"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"brokerbot/internal/bot/keyboards"
	"brokerbot/internal/bot/texts"
	"brokerbot/internal/bot/tg"
	"brokerbot/internal/domain"
)

// Deal — /deal в группе: запуск мастера создания сделки.
func (h *Handlers) Deal(ctx context.Context, b *bot.Bot, update *models.Update) {
	m := update.Message
	if h.requireRole(ctx, b, m, domain.RoleGuarantor) == nil {
		return
	}
	h.deals.Start(ctx, b, m)
}

// DealCallback разбирает callback_data вида deal:<действие>:<аргумент>.
// Роль нажавшего проверяет обработчик действия: кнопку в группе видят все.
func (h *Handlers) DealCallback(ctx context.Context, b *bot.Bot, update *models.Update) {
	cq := update.CallbackQuery
	parts := strings.SplitN(strings.TrimPrefix(cq.Data, keyboards.DealPrefix), ":", 2)
	if len(parts) != 2 {
		tg.Answer(ctx, b, cq.ID, texts.InternalErr)
		return
	}
	action, arg := parts[0], parts[1]

	// payer — единственное действие без id сделки: сделки ещё нет.
	if action == keyboards.ActionPayer {
		if arg != domain.PayerBuyer && arg != domain.PayerSeller {
			tg.Answer(ctx, b, cq.ID, texts.InternalErr)
			return
		}
		h.deals.Payer(ctx, b, cq, arg)
		return
	}

	dealID, err := strconv.ParseInt(arg, 10, 64)
	if err != nil {
		tg.Answer(ctx, b, cq.ID, texts.InternalErr)
		return
	}

	byAction := map[string]func(context.Context, *bot.Bot, *models.CallbackQuery, int64){
		keyboards.ActionConfirm:    h.deals.Confirm,
		keyboards.ActionPaid:       h.deals.Paid,
		keyboards.ActionTicketsAll: h.deals.TicketsAll,
		keyboards.ActionTicketsGot: h.deals.TicketsGot,
		keyboards.ActionEntryOK:    h.deals.EntryOK,
		keyboards.ActionEntryFail:  h.deals.EntryFail,
		keyboards.ActionPayoutDone: h.deals.PayoutDone,
		keyboards.ActionRefundDone: h.deals.RefundDone,
		keyboards.ActionDispute:    h.deals.OpenDispute,
	}
	handler, ok := byAction[action]
	if !ok {
		tg.Answer(ctx, b, cq.ID, texts.InternalErr)
		return
	}
	handler(ctx, b, cq, dealID)
}
