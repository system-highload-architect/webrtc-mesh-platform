package app

import (
	"context"
	"hash/fnv"
	"regexp"
	"sync"

	"webrtc-mesh-platform/internal/pkg/logger"
	"webrtc-mesh-platform/internal/pkg/ratelimit" // Подключаем наш общий Lock-Free лимитер из pkg
	"webrtc-mesh-platform/internal/pkg/trie"      // Подключаем наш LRU-кэш комнат
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
	lruCache *trie.ReactiveLruCache
	rooms    map[string]*domain.VideoRoom
	conns    map[string]map[string]*PeerConnection // roomID -> peerID -> connection
}

// SignalingService инкапсулирует монолитное ядро Control & User Plane плоскостей WebRTC
type SignalingService struct {
	shards      []*RoomShard
	shardCount  uint32
	log         *logger.AppLogger
	hmacSecret  []byte
	t9Engine    *trie.T9PrefixEngine          // Наш наносекундный префиксный движок Т9
	chatLimiter *ratelimit.TokenBucketLimiter // ИСПРАВЛЕНО: Добавлено поле Lock-Free CAS лимитера
	chatQueue   chan string                   // ИСПРАВЛЕНО: Добавлен неблокирующий Go-канал пакетного логера
	urlRegex    *regexp.Regexp                // ИСПРАВЛЕНО: Добавлено регулярное выражение парсинга URL
}

// NewSignalingService инициализирует 32-сегментный распределенный менеджер и общее pkg-шасси
func NewSignalingService(log *logger.AppLogger) *SignalingService {
	s := &SignalingService{
		shardCount:  32,
		shards:      make([]*RoomShard, 32),
		log:         log,
		hmacSecret:  []byte("webrtc_b2b_secret_key"),
		t9Engine:    trie.NewT9PrefixEngine(),
		chatLimiter: ratelimit.NewTokenBucketLimiter(5, 5), // Лимит: 5 сообщений в секунду, емкость 5 токенов
		chatQueue:   make(chan string, 50000),              // Неблокирующий буфер емкостью 50k сообщений
		urlRegex:    regexp.MustCompile(`https?://[^\s]+`),
	}

	// Аллоцируем память под изолированные RAM шарды и вешаем на каждый лимит в 1000 комнат
	for i := uint32(0); i < s.shardCount; i++ {
		s.shards[i] = &RoomShard{
			lruCache: trie.NewReactiveLruCache(1000),
			rooms:    make(map[string]*domain.VideoRoom),
			conns:    make(map[string]map[string]*PeerConnection),
		}
	}

	// Наполняем Т9 словарь базовыми легитимными терминами для проверки автодополнения по Tab
	s.t9Engine.Insert("привет")
	s.t9Engine.Insert("протокол")
	s.t9Engine.Insert("архитектура")
	s.t9Engine.Insert("конференция")
	s.t9Engine.Insert("логирование")

	return s
}

// getShardIndex вычисляет хэш FNV-1a для детерминированной маршрутизации в RAM за O(1)
func (s *SignalingService) getShardIndex(roomID string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(roomID))
	return h.Sum32() % s.shardCount
}

// BroadcastControlMessage реализует gRPC контракт внешнего администрирования
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
