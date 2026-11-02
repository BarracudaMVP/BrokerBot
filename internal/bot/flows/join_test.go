package flows

import (
	"testing"

	"github.com/go-telegram/bot/models"

	"brokerbot/internal/domain"
)

func TestDealRoleByLink(t *testing.T) {
	deal := &domain.Deal{
		SellerInviteLink: "https://t.me/+seller",
		BuyerInviteLink:  "https://t.me/+buyer",
	}
	for link, want := range map[string]string{
		"https://t.me/+seller": domain.PayerSeller,
		"https://t.me/+buyer":  domain.PayerBuyer,
		"https://t.me/+other":  "",
	} {
		if got := dealRoleByLink(deal, &models.ChatInviteLink{InviteLink: link}); got != want {
			t.Errorf("dealRoleByLink(%q) = %q, хотели %q", link, got, want)
		}
	}
	// Вступление не по ссылке (добавил админ) роли не даёт.
	if got := dealRoleByLink(deal, nil); got != "" {
		t.Errorf("dealRoleByLink(nil) = %q", got)
	}
	// Пустые ссылки в сделке не должны совпадать с пустой ссылкой в заявке.
	if got := dealRoleByLink(&domain.Deal{}, &models.ChatInviteLink{}); got == domain.PayerSeller {
		t.Error("пустая ссылка совпала с ролью продавца")
	}
}
