package handlers

import (
	"context"
	"errors"
	"log/slog"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"brokerbot/internal/bot/texts"
	"brokerbot/internal/bot/tg"
	"brokerbot/internal/db"
	"brokerbot/internal/scam"
)

var phoneKeyboard = &models.ReplyKeyboardMarkup{
	Keyboard: [][]models.KeyboardButton{{
		{Text: texts.SharePhoneButton, RequestContact: true},
	}},
	ResizeKeyboard:  true,
	OneTimeKeyboard: true,
}

var removeKeyboard = &models.ReplyKeyboardRemove{RemoveKeyboard: true}

// Start — /start в личке: приглашение к регистрации.
func (h *Handlers) Start(ctx context.Context, b *bot.Bot, update *models.Update) {
	m := update.Message
	if !privateOnly(ctx, b, m) {
		return
	}

	u, err := h.db.GetUser(ctx, m.From.ID)
	if err != nil {
		slog.Error("не удалось получить пользователя", "tg_id", m.From.ID, "err", err)
		tg.Reply(ctx, b, m.Chat.ID, texts.InternalErr, nil)
		return
	}
	if u.IsBlocked {
		tg.Reply(ctx, b, m.Chat.ID, texts.Blocked, removeKeyboard)
		return
	}
	if u.IsRegistered {
		tg.Reply(ctx, b, m.Chat.ID, texts.AlreadyRegistered, removeKeyboard)
		return
	}
	if h.blockIfScam(ctx, b, m.Chat.ID, u, "") {
		return
	}

	tg.Reply(ctx, b, m.Chat.ID, texts.StartGreeting, phoneKeyboard)
}

// onContact завершает регистрацию по присланному контакту.
func (h *Handlers) onContact(ctx context.Context, b *bot.Bot, m *models.Message) {
	// Чужой контакт переслать можно — поэтому сверяем, что это контакт автора.
	if m.Contact.UserID != m.From.ID {
		tg.Reply(ctx, b, m.Chat.ID, texts.ContactNotYours, phoneKeyboard)
		return
	}

	u, err := h.db.GetUser(ctx, m.From.ID)
	if err != nil {
		slog.Error("не удалось получить пользователя", "tg_id", m.From.ID, "err", err)
		tg.Reply(ctx, b, m.Chat.ID, texts.InternalErr, nil)
		return
	}
	if u.IsBlocked {
		tg.Reply(ctx, b, m.Chat.ID, texts.Blocked, removeKeyboard)
		return
	}
	if u.IsRegistered {
		tg.Reply(ctx, b, m.Chat.ID, texts.AlreadyRegistered, removeKeyboard)
		return
	}

	phone := scam.NormalizePhone(m.Contact.PhoneNumber)
	if h.blockIfScam(ctx, b, m.Chat.ID, u, phone) {
		return
	}

	if err := h.db.Register(ctx, u.TgID, phone); err != nil {
		slog.Error("не удалось зарегистрировать пользователя", "tg_id", u.TgID, "err", err)
		tg.Reply(ctx, b, m.Chat.ID, texts.InternalErr, nil)
		return
	}
	tg.Reply(ctx, b, m.Chat.ID, texts.RegisteredOK, removeKeyboard)
}

// blockIfScam проверяет tg_id и (если известен) телефон по скам-листу.
// При совпадении блокирует пользователя, уведомляет модераторов и возвращает true.
func (h *Handlers) blockIfScam(ctx context.Context, b *bot.Bot, chatID int64, u *db.User, phone string) bool {
	entry, err := h.db.ScamCheckUser(ctx, u.TgID, phone)
	if errors.Is(err, db.ErrNotFound) {
		return false
	}
	if err != nil {
		// Падать нельзя, но и пускать непроверенного — тоже.
		slog.Error("не удалось проверить скам-лист", "tg_id", u.TgID, "err", err)
		tg.Reply(ctx, b, chatID, texts.InternalErr, nil)
		return true
	}

	if err := h.db.SetBlocked(ctx, u.TgID, true); err != nil {
		slog.Error("не удалось заблокировать пользователя", "tg_id", u.TgID, "err", err)
	}
	tg.Reply(ctx, b, chatID, texts.Blocked, removeKeyboard)
	h.notifyModerators(ctx, b, texts.ScamRegistrationBlocked(
		texts.UserRef(u.FirstName, u.LastName, u.Username, u.TgID), entry.Type, entry.Reason))
	return true
}
