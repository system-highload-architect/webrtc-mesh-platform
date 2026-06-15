package app

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"webrtc-mesh-platform/internal/pkg/logger"
	"webrtc-mesh-platform/services/auth-service/internal/domain"
)

// AuthService инкапсулирует базовое инфраструктурное шасси модуля авторизации
type AuthService struct {
	mu        sync.RWMutex
	log       *logger.AppLogger
	jwtSecret []byte
	profiles  map[string]*domain.SubscriberAuthProfile // Имитация NoSQL индексов ScyllaDB (SPR)
}

// NewAuthService инициализирует базовые зависимости и кэш профилей абонентов
func NewAuthService(log *logger.AppLogger) *AuthService {
	s := &AuthService{
		log:       log,
		jwtSecret: []byte("webrtc_cluster_jwt_protected_secret_key"),
		profiles:  make(map[string]*domain.SubscriberAuthProfile),
	}
	s.bootstrapMockProfiles()
	return s
}

// bootstrapMockProfiles наполняет базу тестовыми b2b паспортами учетных записей
func (s *AuthService) bootstrapMockProfiles() {
	h1 := sha256.New()
	h1.Write([]byte("admin123"))

	h2 := sha256.New()
	h2.Write([]byte("user123"))

	s.profiles["user_david"] = &domain.SubscriberAuthProfile{
		UserID:       "user_david",
		Name:         "Давид (Модератор)",
		Email:        "david@clearway.ru",
		PasswordHash: fmt.Sprintf("%x", h1.Sum(nil)),
		Role:         "ORGANIZER", // Владелец конференции
		CreatedAt:    time.Now(),
	}

	s.profiles["user_employee"] = &domain.SubscriberAuthProfile{
		UserID:       "user_employee",
		Name:         "Константин (Участник)",
		Email:        "konstantin@clearway.ru",
		PasswordHash: fmt.Sprintf("%x", h2.Sum(nil)),
		Role:         "EMPLOYEE", // Пассивный участник
		CreatedAt:    time.Now(),
	}
}
