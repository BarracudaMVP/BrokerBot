package domain

import "time"

// Статусы сделки (раздел 6 спеки).
const (
	StatusCreated        = "CREATED"
	StatusWaitingMembers = "WAITING_MEMBERS"
	StatusWaitingPayment = "WAITING_PAYMENT"
	StatusPaid           = "PAID"
	StatusWaitingTickets = "WAITING_TICKETS"
	StatusTicketsSent    = "TICKETS_SENT"
	StatusWaitingEntry   = "WAITING_ENTRY"
	StatusEntryOK        = "ENTRY_OK"
	StatusPayoutPending  = "PAYOUT_PENDING"
	StatusClosed         = "CLOSED"
	StatusDispute        = "DISPUTE"
	StatusRefundPending  = "REFUND_PENDING"
	StatusResolved       = "RESOLVED"
	StatusCancelled      = "CANCELLED"
)

// Terminal сообщает, что сделка дальше не двигается.
func Terminal(status string) bool {
	switch status {
	case StatusClosed, StatusResolved, StatusCancelled:
		return true
	}
	return false
}

var statusNames = map[string]string{
	StatusCreated:        "черновик",
	StatusWaitingMembers: "ждём участников",
	StatusWaitingPayment: "ждём оплату",
	StatusPaid:           "оплачено",
	StatusWaitingTickets: "ждём билеты",
	StatusTicketsSent:    "билеты переданы",
	StatusWaitingEntry:   "ждём мероприятие",
	StatusEntryOK:        "проход подтверждён",
	StatusPayoutPending:  "ждём выплату продавцу",
	StatusClosed:         "завершена",
	StatusDispute:        "спор",
	StatusRefundPending:  "ждём возврат покупателю",
	StatusResolved:       "закрыта решением модератора",
	StatusCancelled:      "отменена",
}

// StatusName — человеческое название статуса для сообщений.
func StatusName(status string) string {
	if name, ok := statusNames[status]; ok {
		return name
	}
	return status
}

// Deal — сделка. Повторяет таблицу deals.
type Deal struct {
	ID               int64
	ChatID           int64
	GuarantorID      int64
	SellerID         int64 // 0 — ещё не вступил
	BuyerID          int64
	SellerConfirmed  bool
	BuyerConfirmed   bool
	Status           string
	EventName        string
	EventAt          time.Time
	TicketsCount     int
	Currency         string
	Amount           int64
	CommissionPct    int64 // сотые доли процента
	CommissionPayer  string
	CommissionAmount int64
	BuyerPays        int64
	SellerReceives   int64
	PayoutDetails    string
	SellerInviteLink string
	BuyerInviteLink  string
	DeadlineAt       *time.Time
	CreatedAt        time.Time
}

// Party сообщает, в какой роли пользователь участвует в сделке.
func (d *Deal) Party(tgID int64) string {
	switch tgID {
	case d.SellerID:
		return PayerSeller
	case d.BuyerID:
		return PayerBuyer
	}
	return ""
}
