package config

import "testing"

// setEnv выставляет полный валидный набор переменных, поверх которого тесты ломают
// по одной.
func setEnv(t *testing.T, override map[string]string) {
	t.Helper()
	env := map[string]string{
		"BOT_TOKEN":        "123:abc",
		"DATABASE_URL":     "postgres://localhost/db",
		"CARD_HASH_SECRET": "secret",
		"LOG_CHANNEL_ID":   "-1001234567890",
		"ADMIN_IDS":        " 42, 43 ,",
		"TIMEZONE":         "Europe/Moscow",
	}
	for k, v := range override {
		env[k] = v
	}
	for k, v := range env {
		t.Setenv(k, v)
	}
}

func TestLoadOK(t *testing.T) {
	setEnv(t, nil)
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.LogChannelID != -1001234567890 {
		t.Errorf("LogChannelID = %d", c.LogChannelID)
	}
	// Пробелы и пустой хвост в ADMIN_IDS не должны ломать разбор.
	if len(c.AdminIDs) != 2 || c.AdminIDs[0] != 42 || c.AdminIDs[1] != 43 {
		t.Errorf("AdminIDs = %v", c.AdminIDs)
	}
	if c.Location.String() != "Europe/Moscow" {
		t.Errorf("Location = %s", c.Location)
	}
	if string(c.CardHashSecret) != "secret" {
		t.Errorf("CardHashSecret = %q", c.CardHashSecret)
	}
}

func TestLoadDefaults(t *testing.T) {
	setEnv(t, map[string]string{"TIMEZONE": "", "LOG_CHANNEL_ID": "", "ADMIN_IDS": ""})
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Location.String() != "UTC" {
		t.Errorf("без TIMEZONE ожидали UTC, получили %s", c.Location)
	}
	// Без лог-канала бот должен работать, просто не копируя переписку.
	if c.LogChannelID != 0 || len(c.AdminIDs) != 0 {
		t.Errorf("LogChannelID = %d, AdminIDs = %v", c.LogChannelID, c.AdminIDs)
	}
}

func TestLoadRejectsBadEnv(t *testing.T) {
	for name, broken := range map[string]map[string]string{
		"без токена":           {"BOT_TOKEN": ""},
		"без базы":             {"DATABASE_URL": ""},
		"без секрета для карт": {"CARD_HASH_SECRET": ""},
		"нечисловой канал":     {"LOG_CHANNEL_ID": "channel"},
		"нечисловой админ":     {"ADMIN_IDS": "42,вася"},
		"несуществующий пояс":  {"TIMEZONE": "Europe/Atlantis"},
	} {
		setEnv(t, broken)
		if _, err := Load(); err == nil {
			t.Errorf("%s: конфиг должен был не загрузиться", name)
		}
	}
}
