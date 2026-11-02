package db

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// OpenDisputeTx создаёт запись спора в той же транзакции, что и переход статуса.
// openedBy = nil, если спор открыл планировщик. Второй открытый спор по сделке
// не создаётся.
func OpenDisputeTx(ctx context.Context, tx pgx.Tx, dealID int64, openedBy *int64, reason string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO disputes (deal_id, opened_by, reason, status)
		SELECT $1, $2, $3, 'open'
		WHERE NOT EXISTS (
			SELECT 1 FROM disputes WHERE deal_id = $1 AND status = 'open')`,
		dealID, openedBy, reason)
	return err
}

// HasDispute сообщает, был ли по сделке спор — от этого зависит, чем закончится
// выплата: CLOSED (обычная сделка) или RESOLVED (решение модератора).
func (d *DB) HasDispute(ctx context.Context, dealID int64) (bool, error) {
	var exists bool
	err := d.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM disputes WHERE deal_id = $1)`, dealID).Scan(&exists)
	return exists, err
}
