package app

import (
	"os"
	"sync"

	"webrtc-mesh-platform/internal/pkg/logger"
)

type SubscriberProfile struct {
	UserID   string
	Name     string
	Email    string
	Password string
	Role     string // "ORGANIZER" (Модератор) / "EMPLOYEE" (Участник)
}

// SprStorageService инкапсулирует инфраструктурный слой NoSQL ScyllaDB (SPR)
type SprStorageService struct {
	mu       sync.RWMutex
	log      *logger.AppLogger
	dbPath   string
	profiles map[string]*SubscriberProfile
}

// NewSprStorageService инициализирует память и каталоги хранения таблиц БД
func NewSprStorageService(log *logger.AppLogger) *SprStorageService {
	s := &SprStorageService{
		log:      log,
		dbPath:   "data/scylladb_spr_emulation",
		profiles: make(map[string]*SubscriberProfile),
	}
	_ = os.MkdirAll(s.dbPath, 0755)
	s.bootstrapScyllaTables()
	return s
}
