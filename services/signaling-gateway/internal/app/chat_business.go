package app

import (
	"context"
	"fmt"
	"html"
	"os"
	"strings"
	"time"
)

func (s *SignalingService) ProcessIncomingMessage(roomID, senderID, text string) (string, bool) {
	if !s.chatLimiter.Allow() {
		// ИСПРАВЛЕНО: Корректное b2b-форматирование логера без утечки EXTRA string
		s.log.Error(fmt.Sprintf("[FLOOD ALERT] Отброшен флуд-пакет от Peer [%s] в комнате [%s]", senderID, roomID))
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
		s.log.Error("[IO ERROR] Дисковый буфер чата перегружен")
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
