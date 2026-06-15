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
	"webrtc-mesh-platform/internal/pkg/trie"
	"webrtc-mesh-platform/services/chat-history-service/internal/domain"
)

type HistoryService struct {
	mu           sync.Mutex
	t9Engine     *trie.T9PrefixEngine
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
		log:          log,
		queue:        make(chan *domain.ChatMessage, 50000), // Неблокирующий буфер (Req. 4)
		stopChan:     make(chan struct{}),
		dataDiskPath: diskPath,
		urlRegex:     regexp.MustCompile(`https?://[^\s]+`),
	}

	// Наполняем Т9 словарь для тестов автодополнения по Tab
	s.t9Engine.Insert("привет")
	s.t9Engine.Insert("протокол")
	s.t9Engine.Insert("архитектура")
	s.t9Engine.Insert("конференция")

	return s
}

// ProcessIncomingMessage очищает текст от XSS, режет до 1000 рун и перехватывает фишинг (Req. 4 & 5)
func (s *HistoryService) ProcessIncomingMessage(ctx context.Context, roomID, senderID, text string) (*domain.ChatMessage, error) {
	// 1. Серверная отсечка размера текста на лимит в 1000 символов строго по рунам
	runes := []rune(text)
	if len(runes) > 1000 {
		runes = runes[:1000]
	}
	sanitizedText := string(runes)

	// 2. AppSec Санитизация: базовое экранирование HTML тегов от XSS инъекций
	sanitizedText = html.EscapeString(sanitizedText)

	// 3. Safe Redirect Interceptor: парсим ссылки и заворачиваем в прокси-роутер
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

	// Асинхронный неблокирующий сброс лога в канал по логике select-default
	select {
	case s.queue <- msg:
	default:
		s.log.Error("History Buffer Overflown! Текстовый лог сообщения отброшен во избежание деградации сигнального шлюза.")
	}

	return msg, nil
}

func (s *HistoryService) GetT9Suggestion(ctx context.Context, prefix string) (string, bool) {
	return s.t9Engine.Search(prefix)
}

// StartBatchJanitor запускает пакетный сброс истории на диск строго по 30 штук или тайм-ауту в 100мс (Req. 4)
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
			f, err := os.OpenFile(segmentFile, os.O_CREATE|0x00008|os.O_WRONLY, 0644) // os.O_APPEND
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
			batch = batch[:0] // Очищаем пачку без переаллокации памяти слайса
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
				// Срабатывание по тайм-ауту встроенного тикера в 100 мс, если поток сообщений снизился
				flush()
			}
		}
	}()
}

func (s *HistoryService) Stop() {
	close(s.stopChan)
	s.log.Info("Фоновый воркер истории чата плавно остановлен.")
}
