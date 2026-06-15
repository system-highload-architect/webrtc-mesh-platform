package app

import (
	"context"
	"hash/fnv"
	"sync"

	"webrtc-mesh-platform/internal/pkg/logger" // Подключаем логер шасси
	"webrtc-mesh-platform/services/signaling-gateway/internal/domain"

	"github.com/gorilla/websocket"
)

// PeerConnection инкапсулирует активный WebSocket сокет и права участника
type PeerConnection struct {
	PeerID      string
	WS          *websocket.Conn
	IsModerator bool
	IsMuted     bool
}

// RoomShard описывает изолированный сегмент памяти для полной ликвидации Mutex Contention
type RoomShard struct {
	mu    sync.RWMutex
	rooms map[string]*domain.VideoRoom
	conns map[string]map[string]*PeerConnection // roomID -> peerID -> connection
}

// SignalingService отвечает ИСКЛЮЧИТЕЛЬНО за Control Plane плоскость медиа-комнат (Req. 1 & 2)
type SignalingService struct {
	shards     []*RoomShard
	shardCount uint32
	log        *logger.AppLogger // ДОБАВЛЕНО: Поле логера для AppSec метрик
	hmacSecret []byte            // Ключ для криптографической HMAC-SHA256 CSRF защиты ссылок (Req. 5)
}

// NewSignalingService инициализирует 32-сегментный распределенный менеджер WebRTC сессий
func NewSignalingService(log *logger.AppLogger) *SignalingService {
	s := &SignalingService{
		shardCount: 32,
		shards:     make([]*RoomShard, 32),
		log:        log, // ДОБАВЛЕНО: Инжектим логер шасси
		hmacSecret: []byte("webrtc_b2b_secret_key"),
	}

	// Аллоцируем память под изолированные RAM шарды
	for i := uint32(0); i < s.shardCount; i++ {
		s.shards[i] = &RoomShard{
			rooms: make(map[string]*domain.VideoRoom),
			conns: make(map[string]map[string]*PeerConnection),
		}
	}

	return s
}

// getShardIndex вычисляет хэш FNV-1a для детерминированной маршрутизации в RAM за O(1)
func (s *SignalingService) getShardIndex(roomID string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(roomID))
	return h.Sum32() % s.shardCount
}

// BroadcastControlMessage реализует контракт внешнего gRPC управления
func (s *SignalingService) BroadcastControlMessage(ctx context.Context, roomID string, cmd string, targetPeer string) error {
	idx := s.getShardIndex(roomID)
	shard := s.shards[idx]

	shard.mu.Lock()
	defer shard.mu.Unlock()

	room, exists := shard.rooms[roomID]
	if !exists {
		return nil
	}

	peer, peerExists := room.Peers[targetPeer]
	if peerExists && cmd == "MUTE_AUDIO" {
		peer.IsMuted = true
	}
	return nil
}
