package db

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/pressly/goose/v3"

	"brokerbot/migrations"
)

// TestMigrate прогоняет миграции вверх, вниз и снова вверх на живой базе.
// Без TEST_DATABASE_URL тест пропускается:
//
//	docker compose up -d db
//	TEST_DATABASE_URL='postgres://brokerbot:brokerbot@localhost:5432/brokerbot?sslmode=disable' go test ./internal/db/
func TestMigrate(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL не задан")
	}
	ctx := context.Background()

	if err := Migrate(ctx, url); err != nil {
		t.Fatalf("миграции вверх: %v", err)
	}

	conn, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	for _, table := range []string{"users", "username_history", "deals", "scam_entries",
		"deal_status_log", "deal_files", "message_log", "disputes", "reviews",
		"rating_changes", "settings", "conversation_state"} {
		var n int
		if err := conn.QueryRowContext(ctx,
			`SELECT count(*) FROM information_schema.tables
			 WHERE table_schema = 'public' AND table_name = $1`, table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Errorf("таблица %s не создана", table)
		}
	}

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	if err := goose.DownToContext(ctx, conn, ".", 0); err != nil {
		t.Fatalf("миграции вниз: %v", err)
	}
	if err := Migrate(ctx, url); err != nil {
		t.Fatalf("повторные миграции вверх: %v", err)
	}
}
