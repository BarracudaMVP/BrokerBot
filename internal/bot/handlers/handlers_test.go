package handlers

import (
	"testing"

	"github.com/go-telegram/bot/models"
)

func TestUpdateUsers(t *testing.T) {
	u := &models.Update{
		Message: &models.Message{
			From:           &models.User{ID: 1},
			NewChatMembers: []models.User{{ID: 2}},
		},
		CallbackQuery:   &models.CallbackQuery{From: models.User{ID: 3}},
		ChatJoinRequest: &models.ChatJoinRequest{From: models.User{ID: 4}},
		MyChatMember:    &models.ChatMemberUpdated{From: models.User{ID: 5}},
	}
	got := map[int64]bool{}
	for _, user := range updateUsers(u) {
		got[user.ID] = true
	}
	for id := int64(1); id <= 5; id++ {
		if !got[id] {
			t.Errorf("пользователь %d не найден в апдейте", id)
		}
	}
	if n := len(updateUsers(&models.Update{})); n != 0 {
		t.Errorf("пустой апдейт дал %d пользователей", n)
	}
}
