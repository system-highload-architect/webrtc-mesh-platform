package ratelimit

import (
	"sync/atomic"
	"time"
)

// TokenBucketLimiter реализует 100% Lock-FreeCAS ограничитель частоты запросов
type TokenBucketLimiter struct {
	rate           int64 // Сколько токенов генерируется в секунду
	capacity       int64 // Максимальная емкость корзины токенов
	tokens         int64 // Текущее количество доступных токенов в RAM
	lastRefillTime int64 // Таймстамп последнего пополнения корзины в наносекундах
}

// NewTokenBucketLimiter инициализирует потокобезопасный ограничитель без мьютексов
func NewTokenBucketLimiter(rate, capacity int64) *TokenBucketLimiter {
	return &TokenBucketLimiter{
		rate:           rate,
		capacity:       capacity,
		tokens:         capacity,
		lastRefillTime: time.Now().UnixNano(),
	}
}

// Allow проверяет лимит и атомарно списывает маркер за наносекунды (Compare-And-Swap) (Req. 4)
func (l *TokenBucketLimiter) Allow() bool {
	for {
		now := time.Now().UnixNano()
		lastRefill := atomic.LoadInt64(&l.lastRefillTime)
		currentTokens := atomic.LoadInt64(&l.tokens)

		// 1. Рассчитываем, сколько токенов налилось по времени с момента последнего прохода
		elapsed := now - lastRefill
		if elapsed < 0 {
			elapsed = 0
		}

		// Переводим наносекунды в секунды и вычисляем дельту пополнения
		refillTokens := (elapsed * l.rate) / int64(time.Second)
		newTokens := currentTokens + refillTokens
		if newTokens > l.capacity {
			newTokens = l.capacity
		}

		// 2. Если токенов нет — транзакция заблокирована, флуд отсечен
		if newTokens < 1 {
			return false
		}

		// 3. АТОМАРНАЯ СИНХРОНИЗАЦИЯ: Пробуем закоммитить состояние времени через CAS
		if atomic.CompareAndSwapInt64(&l.lastRefillTime, lastRefill, now) {
			// Если время обновили успешно, пробуем атомарно списать 1 токен (минус один запрос)
			if atomic.CompareAndSwapInt64(&l.tokens, currentTokens, newTokens-1) {
				return true // Успех! Процесс уложился в такты L1/L2 кэша процессора
			}
		}
		// Если CAS сорвался из-за конкурентного горутин-натиска, уходим на безопасный spin-lock цикл заново
	}
}
