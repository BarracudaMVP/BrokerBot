package domain

import "testing"

func TestAtLeast(t *testing.T) {
	if !AtLeast(RoleAdmin, RoleModerator) {
		t.Error("админ должен быть не ниже модератора")
	}
	if AtLeast(RoleGuarantor, RoleModerator) {
		t.Error("гарант не должен быть модератором")
	}
	if !AtLeast(RoleUser, RoleUser) {
		t.Error("роль должна быть не ниже себя")
	}
	if AtLeast("мусор", RoleGuarantor) {
		t.Error("неизвестная роль не даёт прав")
	}
	if ValidRole("мусор") {
		t.Error("неизвестная роль прошла валидацию")
	}
}
