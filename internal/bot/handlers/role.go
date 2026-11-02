package handlers

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"brokerbot/internal/bot/texts"
	"brokerbot/internal/bot/tg"
	"brokerbot/internal/db"
	"brokerbot/internal/domain"
)

// Role — /role <tg_id|@username> <роль>, только для админа.
func (h *Handlers) Role(ctx context.Context, b *bot.Bot, update *models.Update) {
	m := update.Message
	if !privateOnly(ctx, b, m) || h.requireRole(ctx, b, m, domain.RoleAdmin) == nil {
		return
	}

	args := strings.Fields(m.Text)
	if len(args) != 3 {
		tg.Reply(ctx, b, m.Chat.ID, texts.RoleUsage, nil)
		return
	}
	role := strings.ToLower(args[2])
	if !domain.ValidRole(role) {
		tg.Reply(ctx, b, m.Chat.ID, texts.RoleBadRole, nil)
		return
	}

	target, err := h.findUser(ctx, args[1])
	if errors.Is(err, db.ErrNotFound) {
		tg.Reply(ctx, b, m.Chat.ID, texts.RoleUserUnknown, nil)
		return
	}
	if err != nil {
		slog.Error("не удалось найти пользователя", "arg", args[1], "err", err)
		tg.Reply(ctx, b, m.Chat.ID, texts.InternalErr, nil)
		return
	}

	if err := h.db.SetRole(ctx, target.TgID, role); err != nil {
		slog.Error("не удалось сменить роль", "tg_id", target.TgID, "err", err)
		tg.Reply(ctx, b, m.Chat.ID, texts.InternalErr, nil)
		return
	}

	ref := texts.UserRef(target.FirstName, target.LastName, target.Username, target.TgID)
	tg.Reply(ctx, b, m.Chat.ID, texts.RoleChanged(ref, role), nil)
	tg.Reply(ctx, b, target.TgID, texts.RoleChangedNotice(role), nil)
}

// findUser принимает tg_id числом или @username (username — только для тех, кого бот видел).
func (h *Handlers) findUser(ctx context.Context, arg string) (*db.User, error) {
	if id, err := strconv.ParseInt(arg, 10, 64); err == nil {
		return h.db.GetUser(ctx, id)
	}
	return h.db.FindUserByUsername(ctx, strings.TrimPrefix(arg, "@"))
}
