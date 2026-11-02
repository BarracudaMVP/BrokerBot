package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	BotToken       string
	DatabaseURL    string
	LogChannelID   int64
	AdminIDs       []int64
	Location       *time.Location
	CardHashSecret []byte
}

func Load() (*Config, error) {
	c := &Config{
		BotToken:    os.Getenv("BOT_TOKEN"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
	}
	if c.BotToken == "" {
		return nil, fmt.Errorf("BOT_TOKEN не задан")
	}
	if c.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL не задан")
	}

	secret := os.Getenv("CARD_HASH_SECRET")
	if secret == "" {
		return nil, fmt.Errorf("CARD_HASH_SECRET не задан")
	}
	c.CardHashSecret = []byte(secret)

	if v := os.Getenv("LOG_CHANNEL_ID"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("LOG_CHANNEL_ID: %w", err)
		}
		c.LogChannelID = id
	}

	for _, part := range strings.Split(os.Getenv("ADMIN_IDS"), ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("ADMIN_IDS: %w", err)
		}
		c.AdminIDs = append(c.AdminIDs, id)
	}

	tz := os.Getenv("TIMEZONE")
	if tz == "" {
		tz = "UTC"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return nil, fmt.Errorf("TIMEZONE: %w", err)
	}
	c.Location = loc

	return c, nil
}
