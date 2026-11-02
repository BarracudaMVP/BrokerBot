package scam

import "testing"

func has(cs []Candidate, typ, value string) bool {
	for _, c := range cs {
		if c.Type == typ && c.Value == value {
			return true
		}
	}
	return false
}

func TestCandidates(t *testing.T) {
	secret := []byte("s")
	cardHash, _ := HashCard(secret, "4276160012345678")

	// Карта с пробелами — один номер.
	if cs := Candidates(secret, "4276 1600 1234 5678"); !has(cs, TypeCard, cardHash) {
		t.Errorf("карта с пробелами не распознана: %+v", cs)
	}
	// Карта слитно.
	if cs := Candidates(secret, "4276160012345678"); !has(cs, TypeCard, cardHash) {
		t.Error("карта слитно не распознана")
	}
	// Телефон в фразе.
	if cs := Candidates(secret, "переводи на +7 912 345-67-89"); !has(cs, TypePhone, "79123456789") {
		t.Errorf("телефон во фразе не распознан: %+v", cs)
	}
	// Цифра рядом с номером не должна склеиваться с ним во фразе.
	cs := Candidates(secret, "у меня 2 карты 4276160012345678")
	if !has(cs, TypeCard, cardHash) {
		t.Error("карта во фразе не распознана")
	}
	for _, c := range cs {
		if c.Type == TypeCard && c.Value != cardHash {
			t.Errorf("склеился лишний номер: %+v", c)
		}
	}
	// Короткие числа не реквизиты.
	if cs := Candidates(secret, "2 билета по 5000"); len(cs) != 0 {
		t.Errorf("лишние кандидаты: %+v", cs)
	}
}
