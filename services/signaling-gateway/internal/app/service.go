package app

import (
	"context"
	"os"
	"regexp"
	"sync"

	"webrtc-mesh-platform/internal/pkg/logger"
	"webrtc-mesh-platform/internal/pkg/ratelimit"
	"webrtc-mesh-platform/internal/pkg/timewheel" // Твой чистый побитовый пакет
	"webrtc-mesh-platform/internal/pkg/trie"
	"webrtc-mesh-platform/services/signaling-gateway/internal/domain"

	"github.com/gorilla/websocket"
)

// PeerConnection инкапсулирует активный сокет и права. Лежит в app, так как зависит от websocket.Conn
type PeerConnection struct {
	PeerID      string
	WS          *websocket.Conn
	IsModerator bool
	IsMuted     bool
}

// RoomShard описывает изолированный сегмент ОЗУ рантайма
type RoomShard struct {
	mu       sync.RWMutex
	lruCache *trie.ReactiveLruCache
	conns    map[string]map[string]*PeerConnection // roomID -> peerID -> connection
	wheel    *timewheel.BitmappedTimeWheel         // Твой чистый b2b-радар
}

// SignalingService инкапсулирует монолитное ядро Control & User Plane плоскостей WebRTC
type SignalingService struct {
	shards      []*RoomShard
	shardCount  uint32
	log         *logger.AppLogger
	hmacSecret  []byte
	chatLimiter *ratelimit.TokenBucketLimiter
	chatQueue   chan string
	urlRegex    *regexp.Regexp

	recordMutex sync.RWMutex
	videoFiles  map[string]*os.File
}

// NewSignalingService инициализирует 32-сегментный распределенный менеджер
func NewSignalingService(log *logger.AppLogger) *SignalingService {
	s := &SignalingService{
		shardCount:  32,
		shards:      make([]*RoomShard, 32),
		log:         log,
		hmacSecret:  []byte("webrtc_b2b_secret_key"),
		chatLimiter: ratelimit.NewTokenBucketLimiter(5, 5),
		chatQueue:   make(chan string, 50000),
		urlRegex:    regexp.MustCompile(`https?://[^\s]+`),
		videoFiles:  make(map[string]*os.File),
	}

	for i := uint32(0); i < s.shardCount; i++ {
		s.shards[i] = &RoomShard{
			lruCache: trie.NewReactiveLruCache(1000),
			conns:    make(map[string]map[string]*PeerConnection),
			wheel:    timewheel.NewBitmappedTimeWheel(), // Инициализируем инкапсулированное Битовое Колесо
		}
	}
	return s
}

func (s *SignalingService) IsRoomOverloadedOrPaused(roomID string) bool {
	idx := s.getShardIndex(roomID)
	shard := s.shards[idx]
	shard.mu.RLock()
	defer shard.mu.RUnlock()

	roomObj, exists := shard.lruCache.Get(roomID)
	if !exists {
		return false
	}
	room := roomObj.(*domain.VideoRoom)
	return len(room.Peers) > 15 || room.IsPaused
}

func (s *SignalingService) BroadcastControlMessage(ctx context.Context, roomID string, cmd string, targetPeer string) error {
	idx := s.getShardIndex(roomID)
	shard := s.shards[idx]
	shard.mu.Lock()
	defer shard.mu.Unlock()

	roomObj, exists := shard.lruCache.Get(roomID)
	if !exists {
		return nil
	}
	room := roomObj.(*domain.VideoRoom)
	peer, peerExists := room.Peers[targetPeer]
	if peerExists && cmd == "MUTE_AUDIO" {
		peer.IsMuted = true
	}
	return nil
}

func (s *SignalingService) GetAppLogger() *logger.AppLogger {
	return s.log
}
