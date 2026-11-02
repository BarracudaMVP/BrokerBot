package texts

import "fmt"

// UserRef — как показывать пользователя в сообщениях: имя, username, tg_id.
func UserRef(firstName, lastName, username string, tgID int64) string {
	name := firstName
	if lastName != "" {
		name += " " + lastName
	}
	if name == "" {
		name = "без имени"
	}
	if username != "" {
		return fmt.Sprintf("%s (@%s, id %d)", name, username, tgID)
	}
	return fmt.Sprintf("%s (id %d)", name, tgID)
}

// RoleChanged — подтверждение смены роли для того, кто её менял.
func RoleChanged(ref, role string) string {
	return fmt.Sprintf("Роль изменена: %s → %s", ref, role)
}

// RoleChangedNotice — уведомление самому пользователю.
func RoleChangedNotice(role string) string {
	return fmt.Sprintf("Твоя роль изменена на: %s", role)
}

// ScamJoinBlocked — уведомление модераторам о попытке войти в сделку из скам-листа.
func ScamJoinBlocked(dealID int64, ref, matchType, reason string) string {
	return fmt.Sprintf("⚠️ Заявка в сделку #%d от пользователя из скам-листа.\n\n"+
		"Пользователь: %s\nСовпадение по: %s\nПричина записи: %s", dealID, ref, matchType, reason)
}

// GuarantorWaitingPayment — уведомление гаранту, что сделка ждёт оплату.
func GuarantorWaitingPayment(dealID int64) string {
	return fmt.Sprintf("Сделка #%d: обе стороны подтвердили, ждём оплату от покупателя. "+
		"Как получишь деньги — нажми «Оплата получена» в группе сделки.", dealID)
}

// ScamRegistrationBlocked — уведомление модераторам о попытке регистрации из скам-листа.
func ScamRegistrationBlocked(ref, matchType, reason string) string {
	return fmt.Sprintf("⚠️ Попытка регистрации из скам-листа.\n\nПользователь: %s\n"+
		"Совпадение по: %s\nПричина записи: %s", ref, matchType, reason)
}
