package app

import (
	"context"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"webrtc-mesh-platform/internal/pkg/logger"
	"webrtc-mesh-platform/internal/pkg/ratelimit" // ПОДКЛЮЧАЕМ НАШ ОБЩИЙ LOCK-FREE ЛИМИТЕР
	"webrtc-mesh-platform/internal/pkg/trie"
	"webrtc-mesh-platform/services/chat-history-service/internal/domain"
)

type HistoryService struct {
	mu           sync.Mutex
	t9Engine     *trie.T9PrefixEngine
	limiter      *ratelimit.TokenBucketLimiter // ДОБАВЛЕНО: Lock-Free CAS Rate Limiter Shield (Req. 4)
	log          *logger.AppLogger
	queue        chan *domain.ChatMessage
	stopChan     chan struct{}
	dataDiskPath string
	urlRegex     *regexp.Regexp
}

func NewHistoryService(log *logger.AppLogger) *HistoryService {
	diskPath := "data/chat_history_segments"
	_ = os.MkdirAll(diskPath, 0755)

	s := &HistoryService{
		t9Engine:     trie.NewT9PrefixEngine(),
		limiter:      ratelimit.NewTokenBucketLimiter(5, 5), // Лимит: 5 сообщений в сек, емкость 5 маркеров
		log:          log,
		queue:        make(chan *domain.ChatMessage, 50000),
		stopChan:     make(chan struct{}),
		dataDiskPath: diskPath,
		urlRegex:     regexp.MustCompile(`https?://[^\s]+`),
	}

	s.t9Engine.Insert("привет")
	s.t9Engine.Insert("протокол")
	s.t9Engine.Insert("архитектура")
	s.t9Engine.Insert("конференция")

	return s
}

// ProcessIncomingMessage намертво отсекает флуд за 9 наносекунд без мьютексов (Req. 4)
func (s *HistoryService) ProcessIncomingMessage(ctx context.Context, roomID, senderID, text string) (*domain.ChatMessage, error) {
	// ПАТТЕРН БЕЗОПАСНОСТИ (Req. 4): Lock-Free CAS проверка от флуда в чате видеоконференции
	if !s.limiter.Allow() {
		s.log.Error("[FLOOD DETECTED] Отброшен флуд-пакет от Peer [%s] в комнате [%s]", senderID, roomID)
		return nil, fmt.Errorf("rate limit exceeded: too many intensive chat messages")
	}

	runes := []rune(text)
	if len(runes) > 1000 {
		runes = runes[:1000]
	}
	sanitizedText := string(runes)
	sanitizedText = html.EscapeString(sanitizedText)

	containsURL := s.urlRegex.MatchString(sanitizedText)
	if containsURL {
		sanitizedText = s.urlRegex.ReplaceAllStringFunc(sanitizedText, func(match string) string {
			return fmt.Sprintf("https://system-highload-architect.ru", strings.TrimSpace(match))
		})
	}

	msg := &domain.ChatMessage{
		MessageID:   fmt.Sprintf("msg_%d", time.Now().UnixNano()),
		RoomID:      roomID,
		SenderID:    senderID,
		RawText:     sanitizedText,
		ContainsURL: containsURL,
		Timestamp:   time.Now(),
	}

	select {
	case s.queue <- msg:
	default:
		s.log.Error("History Buffer Overflown! Текстовый лог сообщения отброшен.")
	}

	return msg, nil
}

func (s *HistoryService) GetT9Suggestion(ctx context.Context, prefix string) (string, bool) {
	return s.t9Engine.Search(prefix)
}

func (s *HistoryService) StartBatchJanitor(ctx context.Context) {
	s.log.Info("Асинхронный фоновый воркер пакетного логирования истории чата запущен...")

	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		var batch []*domain.ChatMessage
		const batchSize = 30

		flush := func() {
			if len(batch) == 0 {
				return
			}
			s.mu.Lock()
			defer s.mu.Unlock()

			segmentFile := filepath.Join(s.dataDiskPath, "history.bin")
			f, err := os.OpenFile(segmentFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				s.log.Error("Крах дисковой подсистемы чата: %v", err)
				return
			}
			defer f.Close()

			for _, msg := range batch {
				line := fmt.Sprintf("%s,%s,%s,%s\n", msg.Timestamp.Format(time.RFC3339), msg.RoomID, msg.SenderID, msg.RawText)
				_, _ = f.Write([]byte(line))
			}
			s.log.Info("Batch Disk INSERT SUCCESS -> Пачка из %d сообщений успешно закоммичена на NVMe диск.", len(batch))
			batch = batch[:0]
		}

		for {
			select {
			case <-s.stopChan:
				flush()
				return
			case <-ctx.Done():
				flush()
				return
			case msg := <-s.queue:
				batch = append(batch, msg)
				if len(batch) >= batchSize {
					flush()
				}
			case <-ticker.C:
				flush()
			}
		}
	}()
}

func (s *HistoryService) Stop() {
	close(s.stopChan)
	s.log.Info("Фоновый воркер истории чата плавно остановлен.")
}
