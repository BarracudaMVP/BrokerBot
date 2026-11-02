package db

import (
	"context"
	"errors"
	"strconv"

	"github.com/jackc/pgx/v5"

	"brokerbot/internal/scam"
)

type ScamEntry struct {
	ID      int64
	Type    string
	Value   string
	Display string
	Reason  string
}

// ScamFind ищет активную запись скам-листа. Нет записи — ErrNotFound.
func (d *DB) ScamFind(ctx context.Context, typ, value string) (*ScamEntry, error) {
	if value == "" {
		return nil, ErrNotFound
	}
	var e ScamEntry
	err := d.QueryRow(ctx, `
		SELECT id, type, value, coalesce(display, ''), reason FROM scam_entries
		WHERE type = $1 AND value = $2 AND removed_at IS NULL`, typ, value).
		Scan(&e.ID, &e.Type, &e.Value, &e.Display, &e.Reason)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &e, err
}

// ScamCheckUser проверяет пользователя по tg_id и телефону (телефон уже нормализован).
// Возвращает первую найденную запись или ErrNotFound.
func (d *DB) ScamCheckUser(ctx context.Context, tgID int64, phone string) (*ScamEntry, error) {
	e, err := d.ScamFind(ctx, scam.TypeTgID, strconv.FormatInt(tgID, 10))
	if err == nil || !errors.Is(err, ErrNotFound) {
		return e, err
	}
	return d.ScamFind(ctx, scam.TypePhone, phone)
}

// ScamFindAny проверяет сразу несколько кандидатов и возвращает первое совпадение
// или ErrNotFound.
func (d *DB) ScamFindAny(ctx context.Context, candidates []scam.Candidate) (*ScamEntry, error) {
	for _, c := range candidates {
		e, err := d.ScamFind(ctx, c.Type, c.Value)
		if err == nil || !errors.Is(err, ErrNotFound) {
			return e, err
		}
	}
	return nil, ErrNotFound
}
