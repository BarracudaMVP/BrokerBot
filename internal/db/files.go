package db

import "context"

// Виды файлов сделки.
const (
	FileTicket       = "ticket"
	FilePaymentProof = "payment_proof"
	FileDisputeProof = "dispute_proof"
)

// DealFile — вложение, привязанное к сделке.
type DealFile struct {
	DealID       int64
	Kind         string
	FileID       string
	FileUniqueID string
	FileName     string
	MimeType     string
	UploadedBy   int64
}

// AddDealFile запоминает вложение и возвращает, сколько файлов этого вида теперь у сделки.
func (d *DB) AddDealFile(ctx context.Context, f DealFile) (int, error) {
	tx, err := d.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO deal_files (deal_id, kind, file_id, file_unique_id, file_name,
			mime_type, uploaded_by)
		VALUES ($1, $2, $3, $4, nullif($5, ''), nullif($6, ''), $7)`,
		f.DealID, f.Kind, f.FileID, f.FileUniqueID, f.FileName, f.MimeType, f.UploadedBy); err != nil {
		return 0, err
	}

	var n int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM deal_files WHERE deal_id = $1 AND kind = $2`,
		f.DealID, f.Kind).Scan(&n); err != nil {
		return 0, err
	}
	return n, tx.Commit(ctx)
}
