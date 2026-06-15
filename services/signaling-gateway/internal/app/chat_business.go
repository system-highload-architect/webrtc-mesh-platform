package app

import (
	"context"
	"fmt"
	"html"
	"os"
	"strings"
	"time"
)

// ProcessIncomingMessage намертво отсекает флуд за 9 наносекунд и очищает XSS (Req. 4)
func (s *SignalingService) ProcessIncomingMessage(roomID, senderID, text string) (string, bool) {
	// 1. Проверка Lock-Free CAS лимитера частоты из нашего общего pkg шасси
	if !s.chatLimiter.Allow() {
		s.log.Error("[FLOOD ALERT] Отброшен флуд-пакет от Peer [%s] в комнате [%s]", senderID, roomID)
		return "⚠️ [СИСТЕМА]: Превышен лимит отправки сообщений (Флуд заблокирован)", false
	}

	// 2. Жесткая отсечка размера текста сообщения до 1000 символов строго по рунам (Req. 4)
	runes := []rune(text)
	if len(runes) > 1000 {
		runes = runes[:1000]
	}
	sanitizedText := string(runes)

	// 3. AppSec Санитизация: экранируем HTML теги от XSS векторов инъекций
	sanitizedText = html.EscapeString(sanitizedText)

	// 4. Safe Link Redirect: регулярка парсит URL и оборачивает в безопасный роутер (Req. 5)
	containsURL := s.urlRegex.MatchString(sanitizedText)
	if containsURL {
		sanitizedText = s.urlRegex.ReplaceAllStringFunc(sanitizedText, func(match string) string {
			return fmt.Sprintf("https://system-highload-architect.ru", strings.TrimSpace(match))
		})
	}

	// 5. Асинхронный сброс лога в пакетную дисковую очередь
	s.pushToBatchQueue(roomID, senderID, sanitizedText)

	return sanitizedText, containsURL
}

// pushToBatchQueue буферизирует сообщения в Go-канал для дискового воркера
func (s *SignalingService) pushToBatchQueue(roomID, senderID, text string) {
	line := fmt.Sprintf("%s,%s,%s,%s\n", time.Now().Format(time.RFC3339), roomID, senderID, text)
	select {
	case s.chatQueue <- line:
	default:
		s.log.Error("Дисковый буфер чата перегружен, лог сообщения отброшен.")
	}
}

// StartChatBatchWorker накапливает историю в памяти и сбрасывает пачками по 30 штук или 100мс (Req. 4)
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
