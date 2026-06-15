package app

import (
	"context"

	"webrtc-mesh-platform/services/auth-service/internal/domain"
)

// AuthUseCaseManager определяет абстрактный b2b-контракт для управления JWT-сессиями личности
type AuthUseCaseManager interface {
	// AuthenticateUser проверяет сырые учетные данные и генерирует подписанный JWT токен
	AuthenticateUser(ctx context.Context, email, passwordRaw string) (string, int64, error)

	// FetchProfileByID извлекает паспорт пользователя из доменного хранилища
	FetchProfileByID(ctx context.Context, userID string) (*domain.SubscriberAuthProfile, error)
}
