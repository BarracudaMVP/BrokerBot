package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ErrNotFound — записи нет.
var ErrNotFound = errors.New("не найдено")

type User struct {
	TgID         int64
	Username     string
	FirstName    string
	LastName     string
	Phone        string
	Role         string
	IsRegistered bool
	IsBlocked    bool
	SellerRating float64
	BuyerRating  float64
	SellerDeals  int
	BuyerDeals   int
	RegisteredAt *time.Time
}

const userCols = `tg_id, coalesce(username, ''), first_name, coalesce(last_name, ''),
	coalesce(phone, ''), role, is_registered, is_blocked,
	seller_rating, buyer_rating, seller_deals, buyer_deals, registered_at`

func scanUser(row pgx.Row) (*User, error) {
	var u User
	err := row.Scan(&u.TgID, &u.Username, &u.FirstName, &u.LastName, &u.Phone, &u.Role,
		&u.IsRegistered, &u.IsBlocked, &u.SellerRating, &u.BuyerRating,
		&u.SellerDeals, &u.BuyerDeals, &u.RegisteredAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &u, err
}

// TouchUser запоминает любого увиденного пользователя и обновляет его имя/username,
// а новый username дописывает в историю. Вызывается на каждом апдейте.
func (d *DB) TouchUser(ctx context.Context, tgID int64, username, firstName, lastName string) (*User, error) {
	tx, err := d.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	u, err := scanUser(tx.QueryRow(ctx, `
		INSERT INTO users (tg_id, username, first_name, last_name)
		VALUES ($1, nullif($2, ''), $3, nullif($4, ''))
		ON CONFLICT (tg_id) DO UPDATE SET
			username   = nullif($2, ''),
			first_name = $3,
			last_name  = nullif($4, ''),
			updated_at = now()
		RETURNING `+userCols, tgID, username, firstName, lastName))
	if err != nil {
		return nil, fmt.Errorf("upsert пользователя %d: %w", tgID, err)
	}

	if username != "" {
		if _, err := tx.Exec(ctx, `
			INSERT INTO username_history (tg_id, username) VALUES ($1, $2)
			ON CONFLICT (tg_id, username) DO NOTHING`, tgID, username); err != nil {
			return nil, fmt.Errorf("история username %d: %w", tgID, err)
		}
	}

	return u, tx.Commit(ctx)
}

func (d *DB) GetUser(ctx context.Context, tgID int64) (*User, error) {
	return scanUser(d.QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE tg_id = $1`, tgID))
}

// FindUserByUsername ищет по текущему username, затем по истории.
// Username бот знает только для тех, кого видел — это ограничение Bot API.
func (d *DB) FindUserByUsername(ctx context.Context, username string) (*User, error) {
	return scanUser(d.QueryRow(ctx, `
		SELECT `+userCols+` FROM users
		WHERE lower(username) = lower($1)
		   OR tg_id IN (SELECT tg_id FROM username_history WHERE lower(username) = lower($1))
		ORDER BY lower(username) = lower($1) DESC
		LIMIT 1`, username))
}

// Register завершает регистрацию: сохраняет телефон и отметку времени.
func (d *DB) Register(ctx context.Context, tgID int64, phone string) error {
	_, err := d.Exec(ctx, `
		UPDATE users SET phone = $2, is_registered = true,
			registered_at = coalesce(registered_at, now()), updated_at = now()
		WHERE tg_id = $1`, tgID, phone)
	return err
}

// SetBlocked помечает пользователя заблокированным (попал в скам-лист).
func (d *DB) SetBlocked(ctx context.Context, tgID int64, blocked bool) error {
	_, err := d.Exec(ctx,
		`UPDATE users SET is_blocked = $2, updated_at = now() WHERE tg_id = $1`, tgID, blocked)
	return err
}

// SetRole меняет роль. Пользователь должен быть уже известен боту.
func (d *DB) SetRole(ctx context.Context, tgID int64, role string) error {
	tag, err := d.Exec(ctx,
		`UPDATE users SET role = $2, updated_at = now() WHERE tg_id = $1`, tgID, role)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// EnsureAdmins проставляет роль admin начальным админам из конфига.
func (d *DB) EnsureAdmins(ctx context.Context, ids []int64, role string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := d.Exec(ctx, `
		INSERT INTO users (tg_id, role) SELECT unnest($1::bigint[]), $2
		ON CONFLICT (tg_id) DO UPDATE SET role = $2, updated_at = now()`, ids, role)
	return err
}

// Moderators возвращает tg_id всех модераторов и админов — им уходят уведомления.
func (d *DB) Moderators(ctx context.Context) ([]int64, error) {
	rows, err := d.Query(ctx, `SELECT tg_id FROM users WHERE role IN ('moderator', 'admin')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
