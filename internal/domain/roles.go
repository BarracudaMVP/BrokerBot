package domain

// Роли. Порядок важен: каждая следующая включает права предыдущей.
const (
	RoleUser      = "user"
	RoleGuarantor = "guarantor"
	RoleModerator = "moderator"
	RoleAdmin     = "admin"
)

var roleLevel = map[string]int{
	RoleUser:      0,
	RoleGuarantor: 1,
	RoleModerator: 2,
	RoleAdmin:     3,
}

// ValidRole сообщает, что такая роль существует.
func ValidRole(role string) bool {
	_, ok := roleLevel[role]
	return ok
}

// AtLeast сообщает, что role даёт права не ниже min.
func AtLeast(role, min string) bool {
	return roleLevel[role] >= roleLevel[min]
}
