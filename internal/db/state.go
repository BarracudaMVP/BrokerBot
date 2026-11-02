package db

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// State — состояние пошагового диалога. Живёт в БД, чтобы переживать перезапуск бота.
type State struct {
	Flow string
	Step string
	Data []byte // JSON
}

func (d *DB) SetState(ctx context.Context, userID, chatID int64, s State) error {
	if len(s.Data) == 0 {
		s.Data = []byte("{}")
	}
	_, err := d.Exec(ctx, `
		INSERT INTO conversation_state (user_id, chat_id, flow, step, data)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, chat_id) DO UPDATE SET
			flow = $3, step = $4, data = $5, updated_at = now()`,
		userID, chatID, s.Flow, s.Step, s.Data)
	return err
}

// GetState возвращает ErrNotFound, если диалог не начат.
func (d *DB) GetState(ctx context.Context, userID, chatID int64) (*State, error) {
	var s State
	err := d.QueryRow(ctx, `SELECT flow, step, data FROM conversation_state
		WHERE user_id = $1 AND chat_id = $2`, userID, chatID).Scan(&s.Flow, &s.Step, &s.Data)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &s, err
}

func (d *DB) ClearState(ctx context.Context, userID, chatID int64) error {
	_, err := d.Exec(ctx,
		`DELETE FROM conversation_state WHERE user_id = $1 AND chat_id = $2`, userID, chatID)
	return err
}
