package logger

import (
	"testing"

	"github.com/go-telegram/bot/models"
)

func TestChannelLink(t *testing.T) {
	if got := channelLink(-1001234567890, 42); got != "https://t.me/c/1234567890/42" {
		t.Errorf("channelLink = %q", got)
	}
}

func TestAttachmentID(t *testing.T) {
	// Из фото берём максимальный размер — он последний в массиве.
	m := &models.Message{Photo: []models.PhotoSize{{FileID: "small"}, {FileID: "big"}}}
	if got := attachmentID(m); got != "big" {
		t.Errorf("attachmentID(фото) = %q, хотели big", got)
	}
	if got := attachmentID(&models.Message{Text: "привет"}); got != "" {
		t.Errorf("attachmentID(текст) = %q", got)
	}
}
