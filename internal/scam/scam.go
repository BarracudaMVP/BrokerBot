// Package scam нормализует и хеширует значения для скам-листа.
package scam

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// Типы записей скам-листа.
const (
	TypeTgID  = "tg_id"
	TypePhone = "phone"
	TypeCard  = "card"
)

// digits оставляет только цифры.
func digits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteByte(byte(r))
		}
	}
	return b.String()
}

// NormalizePhone приводит телефон к международному виду без плюса и разделителей.
// Российские 8XXXXXXXXXX и 9XXXXXXXXX приводятся к 7XXXXXXXXXX.
func NormalizePhone(s string) string {
	d := digits(s)
	switch {
	case len(d) == 11 && d[0] == '8':
		return "7" + d[1:]
	case len(d) == 10 && d[0] == '9':
		return "7" + d
	}
	return d
}

// HashCard возвращает HMAC-SHA256 номера карты и последние 4 цифры для показа.
// Пустая строка на входе — пустой результат: вызывающий должен это проверить.
func HashCard(secret []byte, card string) (hash, display string) {
	d := digits(card)
	if d == "" {
		return "", ""
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(d))
	return hex.EncodeToString(mac.Sum(nil)), "**** " + d[len(d)-4:]
}
