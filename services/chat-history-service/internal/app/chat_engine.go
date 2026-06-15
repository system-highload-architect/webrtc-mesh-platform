package app

import (
	"context"
	"strings"
	"sync"
	"webrtc-mesh-platform/internal/pkg/logger"
	"webrtc-mesh-platform/internal/pkg/trie"
)

type ChatHistoryEngine struct {
	mu          sync.RWMutex
	log         *logger.AppLogger
	t9Tree      *trie.T9PrefixEngine
	layoutTable map[rune]rune // Таблица соответствия символов для исправления раскладки
}

func NewChatHistoryEngine(log *logger.AppLogger) *ChatHistoryEngine {
	e := &ChatHistoryEngine{
		log:         log,
		t9Tree:      trie.NewT9PrefixEngine(),
		layoutTable: make(map[rune]rune),
	}
	e.bootstrapLayoutTable()
	e.bootstrapT9Dictionary()
	return e
}

// bootstrapLayoutTable инициализирует карту рун для мгновенного перевода раскладки qwerty -> йцукен
func (e *ChatHistoryEngine) bootstrapLayoutTable() {
	eng := "qwertyuiop[]asdfghjkl;'zxcvbnm,./QWERTYUIOP{}ASDFGHJKL:\"ZXCVBNM<>?"
	rus := "йцукенгшщзхъфывапролджэячсмитьбю.ЙЦУКЕНГШЩЗХЪФЫВАПРОЛДЖЭЯЧСМИТЬБЮ,"

	engRunes := []rune(eng)
	rusRunes := []rune(rus)

	for i := 0; i < len(engRunes); i++ {
		e.layoutTable[engRunes[i]] = rusRunes[i]
	}
}

func (e *ChatHistoryEngine) bootstrapT9Dictionary() {
	e.t9Tree.Insert("привет")
	e.t9Tree.Insert("протокол")
	e.t9Tree.Insert("архитектура")
	e.t9Tree.Insert("конференция")
	e.t9Tree.Insert("логирование")
}

// InterceptAndFixLayout за один пасс переводит сбитую раскладку (ghbdtn -> привет)
func (e *ChatHistoryEngine) InterceptAndFixLayout(input string) string {
	runes := []rune(input)
	var sb strings.Builder

	for _, r := range runes {
		if target, exists := e.layoutTable[r]; exists {
			sb.WriteRune(target)
		} else {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// QueryT9Prediction опрашивает Trie-дерево. Если совпадение смазано знаком, очищает его (Исправление багов лога)
func (e *ChatHistoryEngine) QueryT9Prediction(ctx context.Context, prefix string) (string, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Сначала принудительно прогоняем префикс через нормализатор раскладки рун
	fixedPrefix := e.InterceptAndFixLayout(strings.ToLower(prefix))

	// Очищаем от мусорных хвостов, которые летели в лог (например, %D0%BF -> п)
	fixedPrefix = strings.TrimSpace(fixedPrefix)
	fixedPrefix = strings.ReplaceAll(fixedPrefix, "?", "")

	suggestion, found := e.t9Tree.Search(fixedPrefix)
	if found {
		e.log.Info("[DATA PLANE TRIE] Префикс [%s] нормализован в [%s] -> Найдено слово: [%s]", prefix, fixedPrefix, suggestion)
	}
	return suggestion, found
}
