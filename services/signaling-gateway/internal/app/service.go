package app

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"sync"
	"time"

	"webrtc-mesh-platform/internal/pkg/trie"
	"webrtc-mesh-platform/services/signaling-gateway/internal/domain"
)

type RoomShard struct {
	mu    sync.RWMutex
	rooms map[string]*domain.VideoRoom
}

type SignalingService struct {
	shards     []*RoomShard
	shardCount uint32
	t9Engine   *trie.T9PrefixEngine
	hmacSecret []byte
}

func NewSignalingService() *SignalingService {
	s := &SignalingService{
		shardCount: 32,
		shards:     make([]*RoomShard, 32),
		t9Engine:   trie.NewT9PrefixEngine(),
		hmacSecret: []byte("webrtc_b2b_secret_key"),
	}

	for i := uint32(0); i < s.shardCount; i++ {
		s.shards[i] = &RoomShard{rooms: make(map[string]*domain.VideoRoom)}
	}

	// Наполняем Т9 словарь базовыми техническими b2b-терминалами для демонстрации
	s.t9Engine.Insert("привет")
	s.t9Engine.Insert("протокол")
	s.t9Engine.Insert("конференция")
	s.t9Engine.Insert("архитектура")
	s.t9Engine.Insert("логирование")

	return s
}

func (s *SignalingService) getShardIndex(roomID string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(roomID))
	return h.Sum32() % s.shardCount
}

// CreateRoom инициализирует комнату за O(1) и генерирует HMAC-SHA256 токен для защиты от CSRF (Req. 5)
func (s *SignalingService) CreateRoom(ctx context.Context, roomID string, maxPeers int32) (string, error) {
	idx := s.getShardIndex(roomID)
	shard := s.shards[idx]

	shard.mu.Lock()
	defer shard.mu.Unlock()

	shard.rooms[roomID] = &domain.VideoRoom{
		RoomID:    roomID,
		MaxPeers:  maxPeers,
		IsPaused:  false,
		Peers:     make(map[string]*domain.PeerSession),
		CreatedAt: time.Now(),
	}

	// Генерация токена защиты
	mac := hmac.New(sha256.New, s.hmacSecret)
	mac.Write([]byte(roomID + fmt.Sprintf("%d", time.Now().UnixNano())))
	token := hex.EncodeToString(mac.Sum(nil))

	return token, nil
}

// GetAutocompleteSuggestion прокидывает префикс в наше наносекундное Trie-дерево за O(K) (Req. 4)
func (s *SignalingService) GetAutocompleteSuggestion(ctx context.Context, prefix string) (string, bool) {
	return s.t9Engine.Search(prefix)
}

// BroadcastControlMessage управляет модерацией WebSocket фреймов комнат (Req. 1)
func (s *SignalingService) BroadcastControlMessage(ctx context.Context, roomID string, cmd string, targetPeer string) error {
	idx := s.getShardIndex(roomID)
	shard := s.shards[idx]

	shard.mu.Lock()
	defer shard.mu.Unlock()

	room, exists := shard.rooms[roomID]
	if !exists {
		return fmt.Errorf("target video room %s not found in memory", roomID)
	}

	peer, peerExists := room.Peers[targetPeer]
	if peerExists {
		if cmd == "MUTE_AUDIO" {
			peer.IsMuted = true
		}
	}

	return nil
}
