package db

import (
	"context"
	"fmt"

	"brokerbot/internal/domain"
)

// CreateDeal сохраняет сделку в статусе CREATED и пишет первую запись в лог статусов.
func (d *DB) CreateDeal(ctx context.Context, deal *domain.Deal) (*domain.Deal, error) {
	tx, err := d.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	created, err := domain.ScanDeal(tx.QueryRow(ctx, `
		INSERT INTO deals (chat_id, guarantor_id, status, event_name, event_at, tickets_count,
			currency, amount, commission_pct, commission_payer, commission_amount,
			buyer_pays, seller_receives)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::numeric / 100, $10, $11, $12, $13)
		RETURNING `+domain.DealColumns,
		deal.ChatID, deal.GuarantorID, domain.StatusCreated, deal.EventName, deal.EventAt,
		deal.TicketsCount, deal.Currency, deal.Amount, deal.CommissionPct, deal.CommissionPayer,
		deal.CommissionAmount, deal.BuyerPays, deal.SellerReceives))
	if err != nil {
		return nil, fmt.Errorf("создать сделку: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO deal_status_log (deal_id, to_status, actor_id)
		VALUES ($1, $2, $3)`, created.ID, domain.StatusCreated, deal.GuarantorID); err != nil {
		return nil, err
	}
	return created, tx.Commit(ctx)
}

func (d *DB) GetDeal(ctx context.Context, id int64) (*domain.Deal, error) {
	return domain.ScanDeal(d.QueryRow(ctx,
		`SELECT `+domain.DealColumns+` FROM deals WHERE id = $1`, id))
}

// ActiveDealByChat возвращает незавершённую сделку группы (их не больше одной).
func (d *DB) ActiveDealByChat(ctx context.Context, chatID int64) (*domain.Deal, error) {
	return domain.ScanDeal(d.QueryRow(ctx, `SELECT `+domain.DealColumns+` FROM deals
		WHERE chat_id = $1 AND status NOT IN ('CLOSED', 'RESOLVED', 'CANCELLED')`, chatID))
}

// LatestDealByChat возвращает последнюю сделку группы, в том числе завершённую —
// нужно, чтобы логировать переписку и после закрытия сделки.
func (d *DB) LatestDealByChat(ctx context.Context, chatID int64) (*domain.Deal, error) {
	return domain.ScanDeal(d.QueryRow(ctx, `SELECT `+domain.DealColumns+` FROM deals
		WHERE chat_id = $1 ORDER BY id DESC LIMIT 1`, chatID))
}

// SetInviteLinks сохраняет invite-ссылки для продавца и покупателя.
func (d *DB) SetInviteLinks(ctx context.Context, dealID int64, seller, buyer string) error {
	_, err := d.Exec(ctx, `UPDATE deals SET seller_invite_link = $2, buyer_invite_link = $3,
		updated_at = now() WHERE id = $1`, dealID, seller, buyer)
	return err
}

// ClaimParty занимает роль в сделке. Возвращает false, если роль уже занята —
// проверка и занятие в одном запросе, чтобы двое по одной ссылке не затёрли друг друга.
func (d *DB) ClaimParty(ctx context.Context, dealID int64, role string, tgID int64) (bool, error) {
	col := "seller_id"
	if role == domain.PayerBuyer {
		col = "buyer_id"
	}
	tag, err := d.Exec(ctx, `UPDATE deals SET `+col+` = $2, updated_at = now()
		WHERE id = $1 AND `+col+` IS NULL`, dealID, tgID)
	return tag.RowsAffected() == 1, err
}

// Confirm отмечает подтверждение стороной и возвращает обе отметки.
func (d *DB) Confirm(ctx context.Context, dealID int64, role string) (seller, buyer bool, err error) {
	col := "seller_confirmed"
	if role == domain.PayerBuyer {
		col = "buyer_confirmed"
	}
	err = d.QueryRow(ctx, `UPDATE deals SET `+col+` = true, updated_at = now()
		WHERE id = $1 RETURNING seller_confirmed, buyer_confirmed`, dealID).Scan(&seller, &buyer)
	return seller, buyer, err
}

// SetPayoutDetails сохраняет реквизиты продавца для выплаты.
func (d *DB) SetPayoutDetails(ctx context.Context, dealID int64, details string) error {
	_, err := d.Exec(ctx,
		`UPDATE deals SET payout_details = $2, updated_at = now() WHERE id = $1`, dealID, details)
	return err
}
