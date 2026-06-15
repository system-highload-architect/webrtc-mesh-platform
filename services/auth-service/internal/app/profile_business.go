package app

import (
	"context"
	"fmt"

	"webrtc-mesh-platform/services/auth-service/internal/domain"
)

// FetchProfileByID атомарно извлекает профиль абонента из NoSQL структуры по его ID
func (s *AuthService) FetchProfileByID(ctx context.Context, userID string) (*domain.SubscriberAuthProfile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, exists := s.profiles[userID]
	if !exists {
		return nil, fmt.Errorf("profile node %s missing inside storage index", userID)
	}
	return p, nil
}
