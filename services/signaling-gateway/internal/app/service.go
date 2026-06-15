package app

import (
	"context"
	"hash/fnv"
	"sync"

	"webrtc-mesh-platform/internal/pkg/logger"
	"webrtc-mesh-platform/internal/pkg/trie" // Подключаем наше общее платформенное шасси кэша
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

// RoomShard описывает изолированный сегмент памяти со встроенным наносекундным LRU-кэшем
type RoomShard struct {
	mu       sync.RWMutex
	lruCache *trie.ReactiveLruCache                // ИСПРАВЛЕНО: Теперь стейт комнат контролируется LRU-кэшем за O(1)
	rooms    map[string]*domain.VideoRoom          // Карта для быстрого прямого маппинга структур
	conns    map[string]map[string]*PeerConnection // roomID -> peerID -> connection
}

// SignalingService отвечает ИСКЛЮЧИТЕЛЬНО за Control Plane плоскость медиа-комнат
type SignalingService struct {
	shards     []*RoomShard
	shardCount uint32
	log        *logger.AppLogger
	hmacSecret []byte
}

// NewSignalingService инициализирует 32-сегментный распределенный менеджер WebRTC сессий
func NewSignalingService(log *logger.AppLogger) *SignalingService {
	s := &SignalingService{
		shardCount: 32,
		shards:     make([]*RoomShard, 32),
		log:        log,
		hmacSecret: []byte("webrtc_b2b_secret_key"),
	}

	// Аллоцируем память под изолированные RAM шарды и вешаем на каждый лимит комнат (например, 1000)
	for i := uint32(0); i < s.shardCount; i++ {
		s.shards[i] = &RoomShard{
			lruCache: trie.NewReactiveLruCache(1000), // Каноничный лимит комнат на шард
			rooms:    make(map[string]*domain.VideoRoom),
			conns:    make(map[string]map[string]*PeerConnection),
		}
	}

	return s
}

func (s *SignalingService) getShardIndex(roomID string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(roomID))
	return h.Sum32() % s.shardCount
}

func (s *SignalingService) BroadcastControlMessage(ctx context.Context, roomID string, cmd string, targetPeer string) error {
	idx := s.getShardIndex(roomID)
	shard := s.shards[idx]

	shard.mu.Lock()
	defer shard.mu.Unlock()

	// Извлекаем комнату из LRU-индексов, автоматически обновляя её хронологическую позицию (Head)
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
