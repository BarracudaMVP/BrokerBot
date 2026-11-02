package domain

import "testing"

func TestTransitionTable(t *testing.T) {
	// Все целевые статусы должны быть известны, у терминальных нет выходов.
	for from, tos := range transitions {
		if _, ok := statusNames[from]; !ok {
			t.Errorf("неизвестный исходный статус %q", from)
		}
		if Terminal(from) {
			t.Errorf("у терминального статуса %q есть переходы", from)
		}
		for to, actors := range tos {
			if _, ok := statusNames[to]; !ok {
				t.Errorf("неизвестный целевой статус %q", to)
			}
			if len(actors) == 0 {
				t.Errorf("%s → %s: никто не может выполнить переход", from, to)
			}
		}
	}
	// Нельзя прыгнуть из ожидания прохода сразу в выплату.
	if _, ok := transitions[StatusWaitingEntry][StatusPayoutPending]; ok {
		t.Error("выплата без подтверждённого прохода не должна быть разрешена")
	}
}

func TestActorKinds(t *testing.T) {
	deal := &Deal{GuarantorID: 1, SellerID: 2, BuyerID: 3}

	if kinds := actorKinds(deal, nil); len(kinds) != 1 || kinds[0] != ActorSystem {
		t.Errorf("планировщик = %v", kinds)
	}
	// Гарант, который ещё и модератор, получает оба качества.
	kinds := actorKinds(deal, &Actor{TgID: 1, Role: RoleModerator})
	if !hasKind([]ActorKind{ActorGuarantor}, kinds) || !hasKind([]ActorKind{ActorModerator}, kinds) {
		t.Errorf("гарант-модератор = %v", kinds)
	}
	// Посторонний не получает ничего.
	if kinds := actorKinds(deal, &Actor{TgID: 99, Role: RoleUser}); len(kinds) != 0 {
		t.Errorf("посторонний = %v", kinds)
	}
	// Пока продавец не вступил (SellerID = 0), никто не должен стать продавцом.
	empty := &Deal{GuarantorID: 1}
	if kinds := actorKinds(empty, &Actor{TgID: 0, Role: RoleUser}); len(kinds) != 0 {
		t.Errorf("нулевой tg_id совпал с пустой стороной: %v", kinds)
	}
}
