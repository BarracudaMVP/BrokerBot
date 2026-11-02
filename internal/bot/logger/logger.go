// Package logger копирует переписку сделок в лог-канал и в message_log.
//
// Удаления сообщений Bot API не показывает, поэтому копируем строго в момент
// получения. Правки приходят как edited_message и логируются новой версией.
package logger

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"brokerbot/internal/bot/texts"
	"brokerbot/internal/bot/tg"
	"brokerbot/internal/db"
)

type Logger struct {
	db        *db.DB
	channelID int64
	loc       *time.Location
}

func New(d *db.DB, channelID int64, loc *time.Location) *Logger {
	return &Logger{db: d, channelID: channelID, loc: loc}
}

// Copy логирует сообщение группы сделки. Ошибки только логируются: потеря копии
// не должна ломать обработку апдейта.
func (l *Logger) Copy(ctx context.Context, b *bot.Bot, m *models.Message, dealID int64, edited bool) {
	chatID, msgID := m.Chat.ID, int64(m.ID)

	version, err := l.db.NextVersion(ctx, chatID, msgID)
	if err != nil {
		slog.Error("не удалось определить версию сообщения", "chat_id", chatID, "err", err)
		return
	}
	if version == 0 && edited {
		// Оригинал бот не видел (например, был оффлайн) — пишем правку как оригинал.
		edited = false
	}

	entry := db.LoggedMessage{
		ChatID: chatID, MessageID: msgID, Version: version, DealID: dealID,
		UserID: userID(m), Text: messageText(m), FileID: attachmentID(m),
	}
	entry.LogMessageID = l.copyToChannel(ctx, b, m, dealID, version, edited)

	if err := l.db.SaveMessage(ctx, entry); err != nil {
		slog.Error("не удалось записать сообщение в лог", "chat_id", chatID, "err", err)
	}
}

// copyToChannel отправляет заголовок и копию сообщения в лог-канал.
// Возвращает id копии или 0.
func (l *Logger) copyToChannel(ctx context.Context, b *bot.Bot, m *models.Message,
	dealID int64, version int, edited bool) int64 {

	if l.channelID == 0 {
		return 0
	}

	header := l.header(ctx, m, dealID, version, edited)
	if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: l.channelID, Text: header, ParseMode: models.ParseModeHTML,
	}); err != nil {
		slog.Warn("не удалось отправить заголовок в лог-канал", "err", err)
	}

	copied, err := b.CopyMessage(ctx, &bot.CopyMessageParams{
		ChatID:     l.channelID,
		FromChatID: m.Chat.ID,
		MessageID:  m.ID,
	})
	if err != nil {
		// Сервисные сообщения (вступил/вышел) копировать нечего — это нормально.
		slog.Debug("не удалось скопировать сообщение в лог-канал", "err", err)
		return 0
	}
	return int64(copied.ID)
}

func (l *Logger) header(ctx context.Context, m *models.Message, dealID int64, version int, edited bool) string {
	// Заголовок уходит с parse_mode=HTML, а имя и username автора — чужой ввод.
	var b strings.Builder
	fmt.Fprintf(&b, "Сделка #%d · %s · %s", dealID,
		html.EscapeString(author(m)), messageTime(m).In(l.loc).Format("02.01.2006 15:04:05"))

	if edited {
		fmt.Fprintf(&b, "\n✏️ изменено (версия %d)", version)
		if origID, err := l.db.OriginalLogMessageID(ctx, m.Chat.ID, int64(m.ID)); err == nil && origID != 0 {
			fmt.Fprintf(&b, " · <a href=\"%s\">оригинал</a>", channelLink(l.channelID, origID))
		}
	}
	return b.String()
}

// attachmentID — file_id вложения, если оно есть.
func attachmentID(m *models.Message) string {
	if a := tg.FileOf(m); a != nil {
		return a.FileID
	}
	return ""
}

// messageTime — время правки, если это правка, иначе время отправки.
func messageTime(m *models.Message) time.Time {
	ts := m.Date
	if m.EditDate != 0 {
		ts = m.EditDate
	}
	return time.Unix(int64(ts), 0)
}

func author(m *models.Message) string {
	if m.From == nil {
		return "без автора"
	}
	return texts.UserRef(m.From.FirstName, m.From.LastName, m.From.Username, m.From.ID)
}

func userID(m *models.Message) int64 {
	if m.From == nil {
		return 0
	}
	return m.From.ID
}

func messageText(m *models.Message) string {
	if m.Text != "" {
		return m.Text
	}
	return m.Caption
}

// channelLink строит ссылку на сообщение приватного канала: -1001234567890 → .../c/1234567890/5.
func channelLink(channelID, messageID int64) string {
	id := fmt.Sprintf("%d", channelID)
	id = strings.TrimPrefix(strings.TrimPrefix(id, "-"), "100")
	return fmt.Sprintf("https://t.me/c/%s/%d", id, messageID)
}
