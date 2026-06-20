package timewheel

import "sync"

// BitmappedTimeWheel реализует 320-битное колесо времени Давида на массиве uint64
type BitmappedTimeWheel struct {
	mu      sync.RWMutex
	slots   [5]uint64                   // 5 * 64 бита = 320 минут (покрывает 5 часов лимита)
	pointer int                         // Текущая минутная стрелка (0-299)
	buckets map[int]map[string]struct{} // Слот -> Сет ID зарегистрированных задач
}

// NewBitmappedTimeWheel инициализирует пустое битовое кольцо на 300 минут
func NewBitmappedTimeWheel() *BitmappedTimeWheel {
	buckets := make(map[int]map[string]struct{})
	for m := 0; m < 300; m++ {
		buckets[m] = make(map[string]struct{})
	}
	return &BitmappedTimeWheel{
		slots:   [5]uint64{},
		pointer: 0,
		buckets: buckets,
	}
}

// Add регистрирует ключ в кольце на указанное количество минут вперед за O(1)
func (tw *BitmappedTimeWheel) Add(key string, minutesAhead int) int {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	targetSlot := (tw.pointer + minutesAhead) % 300
	wordIdx := targetSlot / 64
	bitIdx := targetSlot % 64

	// Аппаратный взвод бита в 1 на регистрах CPU за 1 такт
	tw.slots[wordIdx] |= (1 << bitIdx)
	tw.buckets[targetSlot][key] = struct{}{}

	return targetSlot
}

// Move атомарно переносит ключ из старого временного слота в новый (фича продления) за O(1)
func (tw *BitmappedTimeWheel) Move(key string, extendMinutes int) (int, error) {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	// 1. Находим, в каком слоте ключ лежит сейчас, и удаляем его
	oldSlot := -1
	for m := 0; m < 300; m++ {
		if _, found := tw.buckets[m][key]; found {
			oldSlot = m
			delete(tw.buckets[m], key)
			break
		}
	}

	// 2. Если старый слот опустел — тушим бит в 0
	if oldSlot != -1 && len(tw.buckets[oldSlot]) == 0 {
		wordIdx := oldSlot / 64
		bitIdx := oldSlot % 64
		tw.slots[wordIdx] &= ^(1 << bitIdx)
	}

	// 3. Вычисляем новый слот в будущем и взводим бит
	newSlot := (tw.pointer + extendMinutes) % 300
	newWordIdx := newSlot / 64
	newBitIdx := newSlot % 64

	tw.slots[newWordIdx] |= (1 << newBitIdx)
	tw.buckets[newSlot][key] = struct{}{}

	return newSlot, nil
}

// Tick сдвигает стрелку на 1 минуту вперед и возвращает список просроченных ID за 1 такт CPU
func (tw *BitmappedTimeWheel) Tick() ([]string, int) {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	tw.pointer = (tw.pointer + 1) % 300
	currentPointer := tw.pointer

	wordIdx := currentPointer / 64
	bitIdx := currentPointer % 64

	// БИТОВЫЙ ТЕСТ: Если 0 — в эту минуту корзина пуста, мгновенный выход без аллокаций!
	if (tw.slots[wordIdx] & (1 << bitIdx)) == 0 {
		return nil, currentPointer
	}

	// Собираем ключи для удаления
	var expiredKeys []string
	for k := range tw.buckets[currentPointer] {
		expiredKeys = append(expiredKeys, k)
		delete(tw.buckets[currentPointer], k)
	}

	// Гасим бит обратно в 0
	tw.slots[wordIdx] &= ^(1 << bitIdx)

	return expiredKeys, currentPointer
}

// Remove принудительно стирает ключ из всех индексов (например, при force close)
func (tw *BitmappedTimeWheel) Remove(key string) {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	for m := 0; m < 300; m++ {
		if _, found := tw.buckets[m][key]; found {
			delete(tw.buckets[m], key)
			if len(tw.buckets[m]) == 0 {
				wordIdx := m / 64
				bitIdx := m % 64
				tw.slots[wordIdx] &= ^(1 << bitIdx)
			}
			break
		}
	}
}
