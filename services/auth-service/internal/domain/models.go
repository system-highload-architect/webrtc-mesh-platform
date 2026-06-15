package domain

import "time"

// UserClaims инкапсулирует полезную нагрузку JWT-токена для авторизации личности в Кубере (Req. 5)
type UserClaims struct {
	UserID    string    `json:"user_id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"` // "ORGANIZER" (Модератор) / "EMPLOYEE" (Участник)
	ExpiresAt time.Time `json:"expires_at"`
}

// SubscriberAuthProfile описывает b2b-паспорт аккаунта, извлекаемый из NoSQL базы ScyllaDB (SPR)
type SubscriberAuthProfile struct {
	UserID       string    `json:"user_id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"password_hash"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}
