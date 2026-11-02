package texts

import (
	"fmt"
	"strings"
	"time"

	"brokerbot/internal/domain"
)

const (
	DealGroupOnly    = "Команда /deal работает только в группе сделки."
	DealAlreadyHere  = "В этой группе уже есть незакрытая сделка. Закрой или отмени её сначала."
	DealWizardCancel = "Создание сделки отменено."

	AskEventName = "Создаём сделку. Название мероприятия?"
	AskEventAt   = "Дата и время начала: ДД.ММ.ГГГГ ЧЧ:ММ (например 31.12.2026 19:30)"
	AskTickets   = "Сколько билетов?"
	AskAmount    = "Сумма сделки (цена билетов)?"
	AskPercent   = "Процент комиссии? Например 7,5"
	AskPayer     = "Кто платит комиссию?"

	InviteLinksIntro = "Сделка создана. Разошли ссылки сторонам — по ним я приму заявку " +
		"на вступление и проверю человека:"
	InviteLinksFailed = "Не удалось выдать invite-ссылки, сделка отменена. " +
		"Проверь, что я админ группы с правом приглашать по ссылке, и повтори /deal."

	JoinNotRegistered = "Заявка отклонена: пользователь не зарегистрирован. " +
		"Пусть напишет мне /start в личку и попробует снова."
	JoinRoleTaken  = "Заявка отклонена: эта роль в сделке уже занята."
	JoinScam       = "Заявка отклонена: пользователь в скам-листе."
	JoinNoDeal     = "Заявка отклонена: в этой группе нет активной сделки."
	JoinUnknownRef = "Заявка отклонена: ссылка не относится к этой сделке."

	ConfirmNotParty    = "Подтверждать могут только продавец и покупатель этой сделки."
	ConfirmWaitOther   = "Подтверждено. Ждём вторую сторону."
	ConfirmAlreadyDone = "Ты уже подтвердил сделку."
	ConfirmBothDone    = "Обе стороны подтвердили сделку. Покупатель, переводи деньги гаранту — " +
		"гарант подтвердит получение кнопкой."

	DealNotFound = "Сделка не найдена."
)

// WizardStepError — ответ на некорректный ввод на шаге мастера.
func WizardStepError(err error) string {
	return "Не понял: " + err.Error() + "\nПопробуй ещё раз."
}

// JoinApproved — сообщение в группу о вступившей стороне.
func JoinApproved(role, ref string) string {
	return fmt.Sprintf("%s в сделке: %s", roleTitle(role), ref)
}

// WarnNegativeRating — предупреждение гаранту о стороне с отрицательным рейтингом.
func WarnNegativeRating(role, ref string, rating float64) string {
	return fmt.Sprintf("⚠️ %s %s с отрицательным рейтингом: %.1f", roleTitle(role), ref, rating)
}

func roleTitle(role string) string {
	if role == domain.PayerSeller {
		return "Продавец"
	}
	return "Покупатель"
}

// InviteLinks — сообщение со ссылками для сторон.
func InviteLinks(sellerLink, buyerLink string) string {
	return fmt.Sprintf("%s\n\nПродавец: %s\nПокупатель: %s",
		InviteLinksIntro, sellerLink, buyerLink)
}

// DealCard — карточка сделки со всеми параметрами.
func DealCard(d *domain.Deal, loc *time.Location, seller, buyer string) string {
	payer := "покупатель, сверх суммы"
	if d.CommissionPayer == domain.PayerSeller {
		payer = "продавец, удерживается из выплаты"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Сделка #%d\n", d.ID)
	fmt.Fprintf(&b, "Мероприятие: %s\n", d.EventName)
	fmt.Fprintf(&b, "Начало: %s\n", domain.FormatEventTime(d.EventAt, loc))
	fmt.Fprintf(&b, "Билетов: %d\n", d.TicketsCount)
	fmt.Fprintf(&b, "Сумма: %s\n", domain.FormatAmount(d.Amount, d.Currency))
	fmt.Fprintf(&b, "Комиссия: %s = %s (%s)\n",
		domain.FormatPercent(d.CommissionPct),
		domain.FormatAmount(d.CommissionAmount, d.Currency), payer)
	fmt.Fprintf(&b, "Покупатель переводит: %s\n", domain.FormatAmount(d.BuyerPays, d.Currency))
	fmt.Fprintf(&b, "Продавец получает: %s\n\n", domain.FormatAmount(d.SellerReceives, d.Currency))
	fmt.Fprintf(&b, "Продавец: %s\n", orDash(seller))
	fmt.Fprintf(&b, "Покупатель: %s\n", orDash(buyer))
	fmt.Fprintf(&b, "Статус: %s", domain.StatusName(d.Status))
	return b.String()
}

// DealCardPrompt — карточка плюс просьба подтвердить.
func DealCardPrompt(d *domain.Deal, loc *time.Location, seller, buyer string) string {
	return DealCard(d, loc, seller, buyer) +
		"\n\nПроверьте параметры и подтвердите сделку — каждая сторона своей кнопкой."
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
