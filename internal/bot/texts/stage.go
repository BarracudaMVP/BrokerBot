package texts

import (
	"fmt"
	"time"

	"brokerbot/internal/domain"
)

const (
	OnlySeller   = "Это кнопка продавца сделки."
	StageChanged = "Сделка уже на другом этапе — обнови сообщение."

	AskPayoutDetailsHint = "Пришли номер карты или телефон, куда перевести выплату, " +
		"одним сообщением."
	PayoutDetailsSaved = "Реквизиты сохранены, я передал их гаранту."
	PayoutDetailsScam  = "Эти реквизиты принять не могу. Сделка приостановлена, " +
		"с ней разберётся модератор."
	PayoutDetailsMissing = "реквизиты продавец ещё не прислал"

	TicketsAllSent = "Продавец отправил все билеты. Покупатель, проверь их и нажми " +
		"«Билеты получил»."

	EntryTooEarly = "Мероприятие ещё не началось, кнопки станут рабочими после его начала."

	DisputeAskProof = "Опиши в этом чате, что произошло, и приложи доказательства: " +
		"скриншоты, фото, переписку. Решение примет модератор."
)

// StageWaitingPayment — что происходит после подтверждения сделки обеими сторонами.
func StageWaitingPayment(d *domain.Deal) string {
	return fmt.Sprintf("Сделка #%d: покупатель переводит %s гаранту. "+
		"Скриншот оплаты можно прислать в этот чат. Как деньги придут, "+
		"гарант нажмёт «Оплата получена».",
		d.ID, domain.FormatAmount(d.BuyerPays, d.Currency))
}

// AskPayoutDetails — запрос реквизитов продавцу в личку.
func AskPayoutDetails(d *domain.Deal) string {
	return fmt.Sprintf("Сделка #%d («%s»), выплата %s.\n\n%s",
		d.ID, d.EventName, domain.FormatAmount(d.SellerReceives, d.Currency), AskPayoutDetailsHint)
}

// PayoutDetailsForGuarantor — уведомление гаранту о полученных реквизитах.
func PayoutDetailsForGuarantor(dealID int64, details string) string {
	return fmt.Sprintf("Сделка #%d: продавец прислал реквизиты для выплаты:\n%s", dealID, details)
}

// StageWaitingTickets — оплата зафиксирована, ждём билеты.
func StageWaitingTickets(d *domain.Deal) string {
	return fmt.Sprintf("Сделка #%d: оплата получена. Продавец, отправь билеты (%d шт.) "+
		"файлами в этот чат.", d.ID, d.TicketsCount)
}

// TicketRecorded — подтверждение записи билета с вопросом о полноте.
func TicketRecorded(got, want int) string {
	return fmt.Sprintf("Билет записан: %d из %d. Это все билеты? "+
		"Если да — нажми «Все отправил».", got, want)
}

// PaymentProofRecorded — подтверждение записи скриншота оплаты.
func PaymentProofRecorded(dealID int64) string {
	return fmt.Sprintf("Подтверждение оплаты по сделке #%d записано.", dealID)
}

// StageWaitingEntry — билеты у покупателя, ждём мероприятие.
func StageWaitingEntry(d *domain.Deal, loc *time.Location) string {
	return fmt.Sprintf("Сделка #%d: билеты у покупателя. Ждём мероприятие %s. "+
		"После него покупатель отметит, пустили его или нет.",
		d.ID, domain.FormatEventTime(d.EventAt, loc))
}

// EntryPrompt — кнопки покупателю: прошёл или нет.
func EntryPrompt(d *domain.Deal, loc *time.Location) string {
	return fmt.Sprintf("Сделка #%d («%s», %s). После мероприятия отметь, пустили ли тебя.",
		d.ID, d.EventName, domain.FormatEventTime(d.EventAt, loc))
}

// StagePayoutPending — проход подтверждён, гаранту пора платить.
func StagePayoutPending(d *domain.Deal) string {
	return fmt.Sprintf("Сделка #%d: проход подтверждён. Гарант выплачивает %s продавцу.",
		d.ID, domain.FormatAmount(d.SellerReceives, d.Currency))
}

// PayoutRequest — заявка на выплату в личку гаранту.
func PayoutRequest(d *domain.Deal) string {
	details := d.PayoutDetails
	if details == "" {
		details = PayoutDetailsMissing
	}
	return fmt.Sprintf("Заявка на выплату по сделке #%d («%s»).\n\nСумма: %s\nРеквизиты: %s\n\n"+
		"Как переведёшь — нажми «Выплачено» в группе сделки.",
		d.ID, d.EventName, domain.FormatAmount(d.SellerReceives, d.Currency), details)
}

// DealClosed — сделка завершена успешно.
func DealClosed(d *domain.Deal) string {
	return fmt.Sprintf("Сделка #%d завершена. Выплата %s продавцу отправлена. Спасибо!",
		d.ID, domain.FormatAmount(d.SellerReceives, d.Currency))
}

// DealResolved — сделка закрыта по решению модератора.
func DealResolved(dealID int64) string {
	return fmt.Sprintf("Сделка #%d закрыта по решению модератора.", dealID)
}

// DisputeOpened — сообщение в группу об открытии спора.
func DisputeOpened(dealID int64, reason string) string {
	return fmt.Sprintf("⚠️ По сделке #%d открыт спор: %s\n\n%s", dealID, reason, DisputeAskProof)
}

// DisputeForModerators — уведомление модераторам.
func DisputeForModerators(dealID int64, by, reason string) string {
	return fmt.Sprintf("⚠️ Спор по сделке #%d.\n\nОткрыл: %s\nПричина: %s", dealID, by, reason)
}

// Причины споров.
const (
	DisputeReasonEntryFail = "покупателя не пустили на мероприятие"
	DisputeReasonManual    = "открыт участником вручную"
	DisputeReasonScamCard  = "реквизиты продавца совпали со скам-листом"
)

// DealCancelled — сделка отменена.
func DealCancelled(dealID int64) string {
	return fmt.Sprintf("Сделка #%d отменена.", dealID)
}

// ScamPayoutBlocked — уведомление модераторам: реквизиты продавца в скам-листе.
func ScamPayoutBlocked(dealID int64, matchType, display, reason string) string {
	if display == "" {
		display = "—"
	}
	return fmt.Sprintf("⚠️ Сделка #%d заморожена: реквизиты продавца в скам-листе.\n\n"+
		"Совпадение по: %s (%s)\nПричина записи: %s", dealID, matchType, display, reason)
}
