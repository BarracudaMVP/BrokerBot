package texts

import (
	"strings"
	"testing"
	"time"

	"brokerbot/internal/domain"
)

// TestDealCard проверяет, что в карточку попадают именно посчитанные суммы:
// это то, на что смотрят обе стороны перед подтверждением.
func TestDealCard(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatal(err)
	}

	amount := int64(1000000) // 10 000,00
	commission, buyerPays, sellerReceives := domain.Commission(amount, 750, domain.PayerBuyer)
	deal := &domain.Deal{
		ID: 7, Status: domain.StatusWaitingMembers, EventName: "Концерт",
		EventAt:      time.Date(2026, 12, 31, 16, 30, 0, 0, time.UTC),
		TicketsCount: 2, Currency: "RUB", Amount: amount,
		CommissionPct: 750, CommissionPayer: domain.PayerBuyer,
		CommissionAmount: commission, BuyerPays: buyerPays, SellerReceives: sellerReceives,
	}

	card := DealCard(deal, loc, "Продавец (id 2)", "Покупатель (id 3)")
	for _, want := range []string{
		"Сделка #7",
		"Концерт",
		"31.12.2026 19:30", // UTC + московский пояс
		"7,5%",
		"750,00 ₽",
		"10\u00a0750,00 ₽", // покупатель платит с комиссией сверху
		"ждём участников",
	} {
		if !strings.Contains(card, want) {
			t.Errorf("в карточке нет %q:\n%s", want, card)
		}
	}

	// Пока сторона не вступила, в карточке прочерк, а не пустота.
	empty := DealCard(deal, loc, "", "")
	if strings.Count(empty, "—") != 2 {
		t.Errorf("ожидали два прочерка:\n%s", empty)
	}
}

// TestPayoutRequest: если реквизитов нет, гарант должен видеть это явно, а не пустую строку.
func TestPayoutRequest(t *testing.T) {
	deal := &domain.Deal{ID: 7, EventName: "Концерт", Currency: "RUB", SellerReceives: 1000000}
	if got := PayoutRequest(deal); !strings.Contains(got, PayoutDetailsMissing) {
		t.Errorf("нет отметки об отсутствии реквизитов:\n%s", got)
	}
	deal.PayoutDetails = "4276 1600 1234 5678"
	if got := PayoutRequest(deal); !strings.Contains(got, deal.PayoutDetails) {
		t.Errorf("реквизиты не попали в заявку:\n%s", got)
	}
}
