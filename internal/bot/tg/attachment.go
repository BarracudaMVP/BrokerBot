package tg

import "github.com/go-telegram/bot/models"

// Attachment — вложение сообщения в удобном для БД виде.
type Attachment struct {
	FileID   string
	UniqueID string
	Name     string
	MimeType string
}

// FileOf возвращает вложение сообщения или nil, если его нет.
// Фото берём в максимальном размере — он последний в массиве.
func FileOf(m *models.Message) *Attachment {
	switch {
	case m.Document != nil:
		return &Attachment{m.Document.FileID, m.Document.FileUniqueID,
			m.Document.FileName, m.Document.MimeType}
	case len(m.Photo) > 0:
		p := m.Photo[len(m.Photo)-1]
		return &Attachment{FileID: p.FileID, UniqueID: p.FileUniqueID, MimeType: "image/jpeg"}
	case m.Video != nil:
		return &Attachment{m.Video.FileID, m.Video.FileUniqueID, m.Video.FileName, m.Video.MimeType}
	case m.Audio != nil:
		return &Attachment{m.Audio.FileID, m.Audio.FileUniqueID, m.Audio.FileName, m.Audio.MimeType}
	case m.Voice != nil:
		return &Attachment{FileID: m.Voice.FileID, UniqueID: m.Voice.FileUniqueID,
			MimeType: m.Voice.MimeType}
	case m.Animation != nil:
		return &Attachment{m.Animation.FileID, m.Animation.FileUniqueID,
			m.Animation.FileName, m.Animation.MimeType}
	case m.VideoNote != nil:
		return &Attachment{FileID: m.VideoNote.FileID, UniqueID: m.VideoNote.FileUniqueID}
	case m.Sticker != nil:
		return &Attachment{FileID: m.Sticker.FileID, UniqueID: m.Sticker.FileUniqueID}
	}
	return nil
}
