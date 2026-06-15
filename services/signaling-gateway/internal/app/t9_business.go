package app

import (
	"context"
)

/**
 * QueryT9Autocomplete осуществляет наносекундный поиск совпадений в Trie-дереве
 * QueryT9Autocomplete performs direct prefix lookup inside the In-Memory Trie tree
 */
func (s *SignalingService) QueryT9Autocomplete(ctx context.Context, prefix string) (string, bool) {
	// Нативно пробрасываем текстовый префикс в наше общее pkg-ядро Trie
	// Алгоритм за один пасс исправит раскладку клавиатуры (ghbdtn -> привет)
	suggestion, found := s.t9Engine.Search(prefix)

	if found {
		s.log.Info("[Trie T9 Engine] Успешное совпадение префикса [%s] -> Подсказано слово: [%s]", prefix, suggestion)
	}

	return suggestion, found
}
