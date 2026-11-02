package domain

import "testing"

func TestParseAmount(t *testing.T) {
	for in, want := range map[string]int64{
		"1500":     150000,
		"1 500,50": 150050,
		"1500.5":   150050,
		"0,99":     99,
		"1500.999": 150099, // лишние знаки отбрасываем, не округляем
	} {
		got, err := ParseAmount(in)
		if err != nil || got != want {
			t.Errorf("ParseAmount(%q) = %d, %v; хотели %d", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "0", "-5", "много", "1,2,3"} {
		if _, err := ParseAmount(bad); err == nil {
			t.Errorf("ParseAmount(%q) должен был дать ошибку", bad)
		}
	}
}

func TestParsePercent(t *testing.T) {
	for in, want := range map[string]int64{"7": 700, "7,5": 750, "0": 0, "10%": 1000} {
		got, err := ParsePercent(in)
		if err != nil || got != want {
			t.Errorf("ParsePercent(%q) = %d, %v; хотели %d", in, got, err, want)
		}
	}
	if _, err := ParsePercent("101"); err == nil {
		t.Error("процент больше 100 должен отвергаться")
	}
}

func TestCommission(t *testing.T) {
	// 10 000,00 под 7,5% = 750,00
	c, pays, gets := Commission(1000000, 750, PayerBuyer)
	if c != 75000 || pays != 1075000 || gets != 1000000 {
		t.Errorf("комиссия с покупателя: %d, %d, %d", c, pays, gets)
	}
	c, pays, gets = Commission(1000000, 750, PayerSeller)
	if c != 75000 || pays != 1000000 || gets != 925000 {
		t.Errorf("комиссия с продавца: %d, %d, %d", c, pays, gets)
	}
	// округление половины вверх: 1,00 под 5,5% = 0,055 → 0,06
	if c, _, _ := Commission(100, 550, PayerBuyer); c != 6 {
		t.Errorf("округление: %d, хотели 6", c)
	}
}

func TestFormat(t *testing.T) {
	if got := FormatAmount(150050, "RUB"); got != "1\u00a0500,50 ₽" {
		t.Errorf("FormatAmount = %q", got)
	}
	if got := FormatAmount(1234567890, "USD"); got != "12\u00a0345\u00a0678,90 USD" {
		t.Errorf("FormatAmount = %q", got)
	}
	for in, want := range map[int64]string{750: "7,5%", 700: "7%", 755: "7,55%", 0: "0%"} {
		if got := FormatPercent(in); got != want {
			t.Errorf("FormatPercent(%d) = %q, хотели %q", in, got, want)
		}
	}
}
