// Package keyboards строит клавиатуры бота.
package keyboards

import (
	"fmt"

	"github.com/go-telegram/bot/models"

	"brokerbot/internal/domain"
)

// DealPrefix — префикс callback_data сделки: deal:<действие>:<аргумент>.
const DealPrefix = "deal:"

// Действия в callback_data.
const (
	ActionPayer      = "payer"
	ActionConfirm    = "confirm"
	ActionPaid       = "paid"
	ActionTicketsAll = "tickets_all"
	ActionTicketsGot = "tickets_got"
	ActionEntryOK    = "entry_ok"
	ActionEntryFail  = "entry_fail"
	ActionPayoutDone = "payout_done"
	ActionRefundDone = "refund_done"
	ActionDispute    = "dispute"
)

func data(action, arg string) string { return DealPrefix + action + ":" + arg }

func button(text, action string, dealID int64) models.InlineKeyboardButton {
	return models.InlineKeyboardButton{Text: text, CallbackData: data(action, fmt.Sprint(dealID))}
}

func markup(rows ...[]models.InlineKeyboardButton) *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// CommissionPayer — шаг мастера: кто платит комиссию.
func CommissionPayer() *models.InlineKeyboardMarkup {
	return markup(
		[]models.InlineKeyboardButton{{Text: "Покупатель (сверху)",
			CallbackData: data(ActionPayer, domain.PayerBuyer)}},
		[]models.InlineKeyboardButton{{Text: "Продавец (из выплаты)",
			CallbackData: data(ActionPayer, domain.PayerSeller)}},
	)
}

// ForStatus — кнопки в группе сделки для текущего статуса. Видят их все,
// право нажать проверяется при нажатии.
func ForStatus(deal *domain.Deal) *models.InlineKeyboardMarkup {
	dispute := []models.InlineKeyboardButton{button("⚠️ Открыть спор", ActionDispute, deal.ID)}

	switch deal.Status {
	case domain.StatusWaitingMembers:
		return markup([]models.InlineKeyboardButton{
			button("✅ Подтверждаю", ActionConfirm, deal.ID)})

	case domain.StatusWaitingPayment:
		return markup([]models.InlineKeyboardButton{
			button("💰 Оплата получена", ActionPaid, deal.ID)})

	case domain.StatusWaitingTickets:
		return markup(
			[]models.InlineKeyboardButton{button("📨 Все отправил", ActionTicketsAll, deal.ID)},
			[]models.InlineKeyboardButton{button("✅ Билеты получил", ActionTicketsGot, deal.ID)},
			dispute,
		)

	case domain.StatusWaitingEntry:
		// Кнопки «Прошёл» / «Не пустили» уходят покупателю в личку.
		return markup(dispute)

	case domain.StatusPayoutPending:
		return markup(
			[]models.InlineKeyboardButton{button("✅ Выплачено", ActionPayoutDone, deal.ID)},
			dispute,
		)

	case domain.StatusRefundPending:
		return markup([]models.InlineKeyboardButton{
			button("✅ Возврат сделан", ActionRefundDone, deal.ID)})
	}
	return nil
}

// Entry — кнопки покупателю: прошёл он на мероприятие или нет.
func Entry(dealID int64) *models.InlineKeyboardMarkup {
	return markup([]models.InlineKeyboardButton{
		button("✅ Прошёл", ActionEntryOK, dealID),
		button("❌ Не пустили", ActionEntryFail, dealID),
	})
}
