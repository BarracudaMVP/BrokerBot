package domain

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// DealColumns — список колонок для ScanDeal. commission_pct в БД хранится как
// NUMERIC(5,2) (процент), а в Go — целым числом сотых долей процента.
const DealColumns = `id, chat_id, guarantor_id, coalesce(seller_id, 0), coalesce(buyer_id, 0),
	seller_confirmed, buyer_confirmed, status, event_name, event_at, tickets_count, currency,
	amount, (commission_pct * 100)::bigint, commission_payer, commission_amount,
	buyer_pays, seller_receives, coalesce(payout_details, ''),
	coalesce(seller_invite_link, ''), coalesce(buyer_invite_link, ''), deadline_at, created_at`

// ScanDeal читает сделку из строки, выбранной по DealColumns.
func ScanDeal(row pgx.Row) (*Deal, error) {
	var d Deal
	err := row.Scan(&d.ID, &d.ChatID, &d.GuarantorID, &d.SellerID, &d.BuyerID,
		&d.SellerConfirmed, &d.BuyerConfirmed, &d.Status, &d.EventName, &d.EventAt,
		&d.TicketsCount, &d.Currency, &d.Amount, &d.CommissionPct, &d.CommissionPayer,
		&d.CommissionAmount, &d.BuyerPays, &d.SellerReceives, &d.PayoutDetails,
		&d.SellerInviteLink, &d.BuyerInviteLink, &d.DeadlineAt, &d.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrDealNotFound
	}
	return &d, err
}

// lockDeal читает сделку с блокировкой строки — защита от двойного нажатия кнопки.
func lockDeal(ctx context.Context, tx pgx.Tx, dealID int64) (*Deal, error) {
	return ScanDeal(tx.QueryRow(ctx,
		`SELECT `+DealColumns+` FROM deals WHERE id = $1 FOR UPDATE`, dealID))
}
