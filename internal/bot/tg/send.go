// Package tg — тонкие обёртки над отправкой сообщений: ошибку доставки только
// логируем, падать из-за закрытой лички или удалённой группы нельзя.
package tg

import (
	"context"
	"log/slog"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func Reply(ctx context.Context, b *bot.Bot, chatID int64, text string, markup models.ReplyMarkup) {
	if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ReplyMarkup: markup,
	}); err != nil {
		slog.Warn("не удалось отправить сообщение", "chat_id", chatID, "err", err)
	}
}

// Broadcast рассылает один текст списку пользователей.
func Broadcast(ctx context.Context, b *bot.Bot, ids []int64, text string) {
	for _, id := range ids {
		Reply(ctx, b, id, text, nil)
	}
}

// Answer закрывает «часики» на inline-кнопке. Вызывать обязательно на каждый callback.
func Answer(ctx context.Context, b *bot.Bot, queryID, text string) {
	if _, err := b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: queryID,
		Text:            text,
		ShowAlert:       text != "",
	}); err != nil {
		slog.Warn("не удалось ответить на callback", "err", err)
	}
}
