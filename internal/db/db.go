// Package db — подключение к Postgres, миграции и запросы.
package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // регистрирует драйвер "pgx" для миграций
	"github.com/pressly/goose/v3"

	"brokerbot/migrations"
)

type DB struct {
	*pgxpool.Pool
}

// Connect открывает пул и проверяет связь.
func Connect(ctx context.Context, url string) (*DB, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("пул postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &DB{pool}, nil
}

// Migrate накатывает миграции из папки migrations.
func Migrate(ctx context.Context, url string) error {
	conn, err := sql.Open("pgx", url)
	if err != nil {
		return fmt.Errorf("открыть соединение для миграций: %w", err)
	}
	defer conn.Close()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.UpContext(ctx, conn, ".")
}
