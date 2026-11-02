package flows

import (
	"testing"
	"time"

	"brokerbot/internal/config"
	"brokerbot/internal/db"
	"brokerbot/internal/domain"
)

func testDeals(t *testing.T) *Deals {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatal(err)
	}
	// db не нужен: step работает только с черновиком и часовым поясом.
	return NewDeals(nil, &config.Config{Location: loc})
}

// TestWizardSteps прогоняет мастер целиком и проверяет, что собранный черновик
// совпадает с введённым.
func TestWizardSteps(t *testing.T) {
	f := testDeals(t)
	var draft dealDraft

	inputs := []struct {
		step, input, wantNext string
	}{
		{stepEventName, "Концерт в Олимпийском", stepEventAt},
		{stepEventAt, "31.12.2026 19:30", stepTickets},
		{stepTickets, "2", stepAmount},
		{stepAmount, "10 000", stepPercent},
		{stepPercent, "7,5", stepPayer},
	}
	for _, in := range inputs {
		next, _, err := f.step(in.step, in.input, &draft)
		if err != nil {
			t.Fatalf("шаг %s с вводом %q: %v", in.step, in.input, err)
		}
		if next != in.wantNext {
			t.Fatalf("после %s ожидали %s, получили %s", in.step, in.wantNext, next)
		}
	}

	if draft.EventName != "Концерт в Олимпийском" || draft.Tickets != 2 {
		t.Errorf("черновик: %+v", draft)
	}
	if draft.Amount != 1000000 || draft.Percent != 750 {
		t.Errorf("суммы черновика: %d, %d", draft.Amount, draft.Percent)
	}
	// Введено по московскому времени, хранится в UTC.
	if got := draft.EventAt.UTC().Format("2006-01-02 15:04"); got != "2026-12-31 16:30" {
		t.Errorf("время мероприятия в UTC = %s", got)
	}
}

// TestWizardBadInput: на плохом вводе шаг не двигается и черновик не портится.
func TestWizardBadInput(t *testing.T) {
	f := testDeals(t)

	for _, c := range []struct{ step, input string }{
		{stepEventName, ""},
		{stepEventAt, "завтра"},
		{stepEventAt, "31.02.2026 19:30"},
		{stepTickets, "0"},
		{stepTickets, "две"},
		{stepAmount, "0"},
		{stepAmount, "дорого"},
		{stepPercent, "101"},
	} {
		draft := dealDraft{EventName: "цел", Tickets: 1, Amount: 100, Percent: 100}
		before := draft
		if _, _, err := f.step(c.step, c.input, &draft); err == nil {
			t.Errorf("шаг %s принял мусор %q", c.step, c.input)
		}
		if draft != before {
			t.Errorf("шаг %s испортил черновик на вводе %q: %+v", c.step, c.input, draft)
		}
	}

	if _, _, err := f.step("непонятный_шаг", "что-то", &dealDraft{}); err == nil {
		t.Error("неизвестный шаг должен давать ошибку")
	}
}

// TestFileKind: чем считается вложение зависит от этапа и от того, кто его прислал.
func TestFileKind(t *testing.T) {
	deal := &domain.Deal{GuarantorID: 1, SellerID: 2, BuyerID: 3}

	cases := []struct {
		status string
		from   int64
		want   string
	}{
		// Скриншот оплаты — от покупателя, и только пока ждём оплату.
		{domain.StatusWaitingPayment, 3, db.FilePaymentProof},
		{domain.StatusWaitingPayment, 2, ""},
		{domain.StatusWaitingPayment, 1, ""},
		// Билеты — от продавца, и только пока ждём билеты.
		{domain.StatusWaitingTickets, 2, db.FileTicket},
		{domain.StatusWaitingTickets, 3, ""},
		// Доказательства в споре — от любой стороны и от гаранта.
		{domain.StatusDispute, 2, db.FileDisputeProof},
		{domain.StatusDispute, 3, db.FileDisputeProof},
		{domain.StatusDispute, 1, db.FileDisputeProof},
		{domain.StatusDispute, 99, ""},
		// На остальных этапах вложения к сделке не привязываем.
		{domain.StatusWaitingEntry, 3, ""},
		{domain.StatusClosed, 2, ""},
	}
	for _, c := range cases {
		deal.Status = c.status
		if got := fileKind(deal, c.from); got != c.want {
			t.Errorf("fileKind(%s, от %d) = %q, хотели %q", c.status, c.from, got, c.want)
		}
	}
}
