package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	_ "time/tzdata" // чтобы TIMEZONE работал без системной базы часовых поясов

	"github.com/go-telegram/bot"

	"brokerbot/internal/bot/handlers"
	"brokerbot/internal/config"
	"brokerbot/internal/db"
	"brokerbot/internal/domain"
)

func main() {
	if err := run(); err != nil {
		slog.Error("бот остановлен с ошибкой", "err", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if err := db.Migrate(ctx, cfg.DatabaseURL); err != nil {
		return err
	}

	store, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer store.Close()

	if err := store.EnsureAdmins(ctx, cfg.AdminIDs, domain.RoleAdmin); err != nil {
		return err
	}

	h := handlers.New(store, cfg)
	b, err := bot.New(cfg.BotToken,
		bot.WithMiddlewares(h.Middlewares()...),
		bot.WithDefaultHandler(h.Default),
	)
	if err != nil {
		return err
	}
	h.Register(b)

	slog.Info("бот запущен", "timezone", cfg.Location.String())
	b.Start(ctx)
	return nil
}
