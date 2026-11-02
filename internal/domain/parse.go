package domain

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// DefaultCurrency — валюта сделок. В мастере не спрашивается (раздела 5 спеки нет
// такого шага), поэтому пока константа.
const DefaultCurrency = "RUB"

// Деньги храним в минимальных единицах, два знака после запятой.
const minorUnits = 100

var eventTimeLayouts = []string{"02.01.2006 15:04", "2006-01-02 15:04"}

// ParseAmount разбирает сумму («1500», «1 500,50», «1500.5») в минимальные единицы.
func ParseAmount(s string) (int64, error) {
	whole, frac, err := splitDecimal(s, 2)
	if err != nil {
		return 0, fmt.Errorf("не похоже на сумму: %s", s)
	}
	if whole == 0 && frac == 0 {
		return 0, fmt.Errorf("сумма должна быть больше нуля")
	}
	return whole*minorUnits + frac, nil
}

// ParsePercent разбирает процент комиссии в сотые доли процента: «7,5» → 750.
func ParsePercent(s string) (int64, error) {
	whole, frac, err := splitDecimal(strings.TrimSuffix(strings.TrimSpace(s), "%"), 2)
	if err != nil {
		return 0, fmt.Errorf("не похоже на процент: %s", s)
	}
	pct := whole*100 + frac
	if pct > 100*100 {
		return 0, fmt.Errorf("процент не может быть больше 100")
	}
	return pct, nil
}

// splitDecimal разбирает неотрицательное десятичное число в целую часть и
// дробную, приведённую к decimals знакам (с отбрасыванием лишних).
func splitDecimal(s string, decimals int) (whole, frac int64, err error) {
	s = strings.NewReplacer(" ", "", "\u00a0", "", ",", ".").Replace(strings.TrimSpace(s))
	if s == "" {
		return 0, 0, fmt.Errorf("пусто")
	}
	intPart, fracPart, _ := strings.Cut(s, ".")
	if intPart == "" {
		intPart = "0"
	}
	if whole, err = strconv.ParseInt(intPart, 10, 64); err != nil || whole < 0 {
		return 0, 0, fmt.Errorf("целая часть: %s", intPart)
	}
	if fracPart == "" {
		return whole, 0, nil
	}
	if len(fracPart) > decimals {
		fracPart = fracPart[:decimals]
	}
	fracPart += strings.Repeat("0", decimals-len(fracPart))
	if frac, err = strconv.ParseInt(fracPart, 10, 64); err != nil || frac < 0 {
		return 0, 0, fmt.Errorf("дробная часть: %s", fracPart)
	}
	return whole, frac, nil
}

// ParseCount разбирает количество билетов.
func ParseCount(s string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 1 {
		return 0, fmt.Errorf("количество билетов — целое число от 1")
	}
	return n, nil
}

// ParseEventTime разбирает дату и время мероприятия в часовом поясе loc и
// возвращает их в UTC.
func ParseEventTime(s string, loc *time.Location) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range eventTimeLayouts {
		if t, err := time.ParseInLocation(layout, s, loc); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("формат даты: ДД.ММ.ГГГГ ЧЧ:ММ, например 31.12.2026 19:30")
}

// FormatAmount печатает сумму из минимальных единиц: «1 500,50 ₽».
func FormatAmount(v int64, currency string) string {
	sign := ""
	if v < 0 {
		sign, v = "−", -v
	}
	whole := strconv.FormatInt(v/minorUnits, 10)
	for i := len(whole) - 3; i > 0; i -= 3 {
		whole = whole[:i] + "\u00a0" + whole[i:]
	}
	return fmt.Sprintf("%s%s,%02d %s", sign, whole, v%minorUnits, currencySymbol(currency))
}

func currencySymbol(currency string) string {
	if currency == "RUB" {
		return "₽"
	}
	return currency
}

// FormatPercent печатает процент из сотых долей: 750 → «7,5%».
func FormatPercent(pct int64) string {
	s := fmt.Sprintf("%d,%02d", pct/100, pct%100)
	s = strings.TrimRight(s, "0")
	return strings.TrimSuffix(s, ",") + "%"
}

// FormatEventTime печатает время мероприятия в часовом поясе loc.
func FormatEventTime(t time.Time, loc *time.Location) string {
	return t.In(loc).Format("02.01.2006 15:04")
}
