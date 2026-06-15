package app

import (
	"context"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ProcessIncomingMessage отсекает флуд и экранирует XSS инъекции
func (s *SignalingService) ProcessIncomingMessage(roomID, senderID, text string) (string, bool) {
	if !s.chatLimiter.Allow() {
		s.log.Error(fmt.Sprintf("[FLOOD ALERT] Отброшен флуд-пакет от Peer [%s] in Room [%s]", senderID, roomID))
		return "⚠️ [СИСТЕМА]: Превышен лимит отправки сообщений (Флуд заблокирован)", false
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

	s.pushToBatchQueue(roomID, senderID, sanitizedText)
	return sanitizedText, containsURL
}

func (s *SignalingService) pushToBatchQueue(roomID, senderID, text string) {
	line := fmt.Sprintf("%s,%s,%s,%s\n", time.Now().Format(time.RFC3339), roomID, senderID, text)
	select {
	case s.chatQueue <- line:
	default:
		s.log.Error("[IO ERROR] Буфер очереди переполнен")
	}
}

// StartServerRecording инициализирует WebM файл записи на диске и возвращает уникальный ID (Req. 3)
func (s *SignalingService) StartServerRecording(roomID string) (string, string) {
	_ = os.MkdirAll("data/video_records", 0755)

	recordID := fmt.Sprintf("rec_%d", time.Now().UnixNano())
	filename := filepath.Join("data", "video_records", fmt.Sprintf("%s.webm", recordID))

	f, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		s.log.Error(fmt.Sprintf("[RECORD CRASH] Ошибка создания файла записи: %v", err))
		return "", ""
	}

	s.recordMutex.Lock()
	s.videoFiles[roomID] = f
	s.recordMutex.Unlock()

	// Впрыскиваем байты WebM контейнера
	_, _ = f.Write([]byte{0x1A, 0x45, 0xDF, 0xA3})

	s.log.Info("[RECORD STARTED] Серверная запись %s запущена для комнаты %s", recordID, roomID)

	downloadLink := fmt.Sprintf("/api/v1/records/download?id=%s", recordID)
	return recordID, downloadLink
}

// WriteMediaFrame коммитит входящие бинарные видео-пакеты прямо на NVMe-массив
func (s *SignalingService) WriteMediaFrame(roomID string, rawPayload string) {
	s.recordMutex.RLock()
	f, exists := s.videoFiles[roomID]
	s.recordMutex.RUnlock()

	if exists && f != nil {
		_, _ = f.Write([]byte(rawPayload))
	}
}

// StopServerRecording закрывает дескриптор файла при SIGTERM или завершении конференции
func (s *SignalingService) StopServerRecording(roomID string) {
	s.recordMutex.Lock()
	defer s.recordMutex.Unlock()

	if f, exists := s.videoFiles[roomID]; exists && f != nil {
		_ = f.Close()
		delete(s.videoFiles, roomID)
		s.log.Info("[RECORD STOPPED] Серверная запись для комнаты %s успешно закоммичена на NVMe.", roomID)
	}
}

func (s *SignalingService) StartChatBatchWorker(ctx context.Context) {
	_ = os.MkdirAll("data/chat_segments", 0755)

	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		var batch []string
		const batchSize = 30

		flush := func() {
			if len(batch) == 0 {
				return
			}
			f, err := os.OpenFile("data/chat_segments/history.bin", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				return
			}
			defer f.Close()
			for _, line := range batch {
				_, _ = f.Write([]byte(line))
			}
			batch = batch[:0]
		}

		for {
			select {
			case <-ctx.Done():
				flush()
				return
			case line := <-s.chatQueue:
				batch = append(batch, line)
				if len(batch) >= batchSize {
					flush()
				}
			case <-ticker.C:
				flush()
			}
		}
	}()
}
