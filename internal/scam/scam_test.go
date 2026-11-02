package scam

import "testing"

func TestNormalizePhone(t *testing.T) {
	for in, want := range map[string]string{
		"+7 (912) 345-67-89": "79123456789",
		"89123456789":        "79123456789",
		"9123456789":         "79123456789",
		"+380501234567":      "380501234567",
		"":                   "",
	} {
		if got := NormalizePhone(in); got != want {
			t.Errorf("NormalizePhone(%q) = %q, хотели %q", in, got, want)
		}
	}
}

func TestHashCard(t *testing.T) {
	h1, d := HashCard([]byte("s"), "4276 1600 1234 5678")
	h2, _ := HashCard([]byte("s"), "4276160012345678")
	if h1 != h2 {
		t.Error("разделители в номере карты меняют хеш")
	}
	if d != "**** 5678" {
		t.Errorf("display = %q", d)
	}
	if h3, _ := HashCard([]byte("other"), "4276160012345678"); h3 == h1 {
		t.Error("хеш не зависит от секрета")
	}
	if h, _ := HashCard([]byte("s"), "нет цифр"); h != "" {
		t.Error("пустой номер должен дать пустой хеш")
	}
}
