package app

import (
	"context"
	"fmt"
)

// GetProfileByEmail выполняет instantaneous-поиск абонента по уникальному Email-индексу
func (s *SprStorageService) GetProfileByEmail(ctx context.Context, email string) (*SubscriberProfile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, p := range s.profiles {
		if p.Email == email {
			return p, nil
		}
	}
	return nil, fmt.Errorf("subscriber profile with email %s missing inside ScyllaDB keyspace", email)
}

// GetProfileByID выполняет прямую выборку профиля по Primary Key (User_ID) за O(1)
func (s *SprStorageService) GetProfileByID(ctx context.Context, userID string) (*SubscriberProfile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, exists := s.profiles[userID]
	if !exists {
		return nil, fmt.Errorf("primary key violation: user_id %s missing in ScyllaDB partition", userID)
	}
	return p, nil
}
