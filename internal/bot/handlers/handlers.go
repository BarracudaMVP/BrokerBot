// Package handlers — обработчики апдейтов Telegram.
package handlers

import (
	"context"
	"errors"
	"log/slog"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"brokerbot/internal/bot/flows"
	"brokerbot/internal/bot/keyboards"
	botlog "brokerbot/internal/bot/logger"
	"brokerbot/internal/bot/texts"
	"brokerbot/internal/bot/tg"
	"brokerbot/internal/config"
	"brokerbot/internal/db"
	"brokerbot/internal/domain"
)

type Handlers struct {
	db     *db.DB
	cfg    *config.Config
	deals  *flows.Deals
	msgLog *botlog.Logger
}

func New(d *db.DB, cfg *config.Config) *Handlers {
	return &Handlers{
		db:     d,
		cfg:    cfg,
		deals:  flows.NewDeals(d, cfg),
		msgLog: botlog.New(d, cfg.LogChannelID, cfg.Location),
	}
}

// Register навешивает команды и кнопки на бота.
func (h *Handlers) Register(b *bot.Bot) {
	b.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypeCommand, h.Start)
	b.RegisterHandler(bot.HandlerTypeMessageText, "/role", bot.MatchTypeCommand, h.Role)
	b.RegisterHandler(bot.HandlerTypeMessageText, "/deal", bot.MatchTypeCommand, h.Deal)
	b.RegisterHandler(bot.HandlerTypeCallbackQueryData, keyboards.DealPrefix,
		bot.MatchTypePrefix, h.DealCallback)
}

// Middlewares — порядок важен: сначала запоминаем людей, потом логируем переписку,
// и только потом обрабатываем команду.
func (h *Handlers) Middlewares() []bot.Middleware {
	return []bot.Middleware{h.TrackUsers, h.LogMessages}
}

// Default — апдейты, которые не попали ни в одну команду: контакт при регистрации,
// шаги мастера сделки, заявки на вступление.
func (h *Handlers) Default(ctx context.Context, b *bot.Bot, update *models.Update) {
	if r := update.ChatJoinRequest; r != nil {
		h.deals.JoinRequest(ctx, b, r)
		return
	}

	m := update.Message
	if m == nil || m.From == nil {
		return
	}
	if m.Chat.Type == models.ChatTypePrivate {
		if m.Contact != nil {
			h.onContact(ctx, b, m)
			return
		}
		h.deals.Feed(ctx, b, m)
		return
	}

	// Шаг мастера съедает сообщение целиком, иначе это может быть файл сделки.
	if !h.deals.Feed(ctx, b, m) {
		h.deals.OnGroupFile(ctx, b, m)
	}
}

// TrackUsers запоминает каждого увиденного пользователя и его username.
// Ограничение Bot API: по @username получить user_id нельзя, поэтому ведём свою базу.
func (h *Handlers) TrackUsers(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		for _, u := range updateUsers(update) {
			if u.IsBot {
				continue
			}
			if _, err := h.db.TouchUser(ctx, u.ID, u.Username, u.FirstName, u.LastName); err != nil {
				slog.Error("не удалось сохранить пользователя", "tg_id", u.ID, "err", err)
			}
		}
		next(ctx, b, update)
	}
}

// LogMessages копирует переписку групп сделок в лог-канал и message_log.
// Делается в middleware, а не в обработчике: логировать надо всё, включая команды,
// и строго в момент получения — удалений Bot API не показывает.
func (h *Handlers) LogMessages(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		for _, m := range []struct {
			msg    *models.Message
			edited bool
		}{{update.Message, false}, {update.EditedMessage, true}} {
			if m.msg == nil || m.msg.Chat.Type == models.ChatTypePrivate {
				continue
			}
			deal, err := h.db.LatestDealByChat(ctx, m.msg.Chat.ID)
			if err != nil {
				if !errors.Is(err, domain.ErrDealNotFound) {
					slog.Error("не удалось найти сделку группы", "chat_id", m.msg.Chat.ID, "err", err)
				}
				continue
			}
			h.msgLog.Copy(ctx, b, m.msg, deal.ID, m.edited)
		}
		next(ctx, b, update)
	}
}

// updateUsers собирает всех пользователей, которых видно в апдейте.
func updateUsers(u *models.Update) []models.User {
	var out []models.User
	add := func(user *models.User) {
		if user != nil {
			out = append(out, *user)
		}
	}

	for _, m := range []*models.Message{u.Message, u.EditedMessage} {
		if m == nil {
			continue
		}
		add(m.From)
		out = append(out, m.NewChatMembers...)
		if m.LeftChatMember != nil {
			add(m.LeftChatMember)
		}
	}
	if u.CallbackQuery != nil {
		add(&u.CallbackQuery.From)
	}
	if u.ChatJoinRequest != nil {
		add(&u.ChatJoinRequest.From)
	}
	for _, cm := range []*models.ChatMemberUpdated{u.MyChatMember, u.ChatMember} {
		if cm != nil {
			add(&cm.From)
		}
	}
	return out
}

// notifyModerators рассылает сообщение всем модераторам и админам.
func (h *Handlers) notifyModerators(ctx context.Context, b *bot.Bot, text string) {
	ids, err := h.db.Moderators(ctx)
	if err != nil {
		slog.Error("не удалось получить список модераторов", "err", err)
		return
	}
	tg.Broadcast(ctx, b, ids, text)
}

// requireRole достаёт автора сообщения и проверяет, что его роль не ниже min.
// Возвращает nil, если прав не хватает (ответ уже отправлен).
func (h *Handlers) requireRole(ctx context.Context, b *bot.Bot, m *models.Message, min string) *db.User {
	u, err := h.db.GetUser(ctx, m.From.ID)
	if err != nil {
		slog.Error("не удалось получить пользователя", "tg_id", m.From.ID, "err", err)
		tg.Reply(ctx, b, m.Chat.ID, texts.InternalErr, nil)
		return nil
	}
	if !domain.AtLeast(u.Role, min) {
		tg.Reply(ctx, b, m.Chat.ID, texts.NoRights, nil)
		return nil
	}
	return u
}

// privateOnly отсекает команды, которые работают только в личке.
func privateOnly(ctx context.Context, b *bot.Bot, m *models.Message) bool {
	if m.Chat.Type != models.ChatTypePrivate {
		tg.Reply(ctx, b, m.Chat.ID, texts.PrivateOnly, nil)
		return false
	}
	return true
}
