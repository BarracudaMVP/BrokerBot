package db

import "context"

// LoggedMessage — запись переписки сделки.
type LoggedMessage struct {
	ChatID       int64
	MessageID    int64
	Version      int
	DealID       int64 // 0 — вне сделки
	UserID       int64
	Text         string
	FileID       string
	LogMessageID int64 // id копии в лог-канале, 0 если копии нет
}

// NextVersion возвращает номер следующей версии сообщения: 0 — оригинал, 1+ — правки.
func (d *DB) NextVersion(ctx context.Context, chatID, messageID int64) (int, error) {
	var n int
	err := d.QueryRow(ctx, `SELECT coalesce(max(version) + 1, 0) FROM message_log
		WHERE chat_id = $1 AND message_id = $2`, chatID, messageID).Scan(&n)
	return n, err
}

// OriginalLogMessageID — id копии оригинала в лог-канале, чтобы сослаться на неё из правки.
func (d *DB) OriginalLogMessageID(ctx context.Context, chatID, messageID int64) (int64, error) {
	var id *int64
	err := d.QueryRow(ctx, `SELECT log_message_id FROM message_log
		WHERE chat_id = $1 AND message_id = $2 AND version = 0`, chatID, messageID).Scan(&id)
	if err != nil || id == nil {
		return 0, err
	}
	return *id, nil
}

// SaveMessage пишет сообщение в message_log. Повтор того же (chat, message, version)
// игнорируется: бот может увидеть один апдейт дважды после перезапуска.
func (d *DB) SaveMessage(ctx context.Context, m LoggedMessage) error {
	_, err := d.Exec(ctx, `
		INSERT INTO message_log (chat_id, message_id, version, deal_id, user_id,
			text, file_id, log_message_id)
		VALUES ($1, $2, $3, nullif($4, 0::bigint), $5, nullif($6, ''), nullif($7, ''),
			nullif($8, 0::bigint))
		ON CONFLICT (chat_id, message_id, version) DO NOTHING`,
		m.ChatID, m.MessageID, m.Version, m.DealID, m.UserID, m.Text, m.FileID, m.LogMessageID)
	return err
}
