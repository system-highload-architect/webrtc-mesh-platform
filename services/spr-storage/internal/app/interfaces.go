package app

import (
	"context"
)

// SprStorageEngine задает абстрактный контракт плоскости данных для SPR-хранилища
type SprStorageEngine interface {
	// GetProfileByEmail извлекает b2b-паспорт абонента из NoSQL таблиц
	GetProfileByEmail(ctx context.Context, email string) (*SubscriberProfile, error)

	// GetProfileByID выполняет instantaneous-выборку по Primary Key (User_ID)
	GetProfileByID(ctx context.Context, userID string) (*SubscriberProfile, error)
}
