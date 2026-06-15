package backoff

import (
	"math"
	"math/rand"
	"time"
)

// ExponentialBackoff инкапсулирует параметры алгоритма экспоненциальных повторов
type ExponentialBackoff struct {
	BaseDelay time.Duration // Начальная задержка (например, 1 секунда)
	MaxDelay  time.Duration // Потолок задержки (например, 30 секунд)
	Factor    float64       // Множитель экспоненты (стандартно: 2.0)
	Jitter    bool          // Флаг подмешивания случайного шума для размытия нагрузки
}

func NewExponentialBackoff(base, max time.Duration) *ExponentialBackoff {
	return &ExponentialBackoff{
		BaseDelay: base,
		MaxDelay:  max,
		Factor:    2.0,
		Jitter:    true,
	}
}

// CalculateDelay вычисляет время ожидания для конкретной попытки за O(1) времени (Req. 4)
func (b *ExponentialBackoff) CalculateDelay(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}

	// Формула: BaseDelay * (Factor ^ attempt)
	minShift := float64(b.BaseDelay) * math.Pow(b.Factor, float64(attempt))
	delay := time.Duration(minShift)

	// Ограничиваем сверху жестким b2b-лимитом MaxDelay
	if delay > b.MaxDelay {
		delay = b.MaxDelay
	}

	// Подмешиваем джиттер (случайный шум до 10%), чтобы тысячи подов не стреляли в одну миллисекунду
	if b.Jitter {
		randomNoise := rand.Float64() * 0.1 * float64(delay)
		delay = delay + time.Duration(randomNoise)
	}

	return delay
}
