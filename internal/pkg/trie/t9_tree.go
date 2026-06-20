package trie

import (
	"sync"
)

type TrieNode struct {
	Children  map[rune]*TrieNode
	IsWord    bool
	WordValue string // Хранит полное слово на конце ветки для быстрого Т9 ответа
}

type T9PrefixEngine struct {
	mu         sync.RWMutex
	root       *TrieNode
	translitEn map[rune]rune // Маппер раскладки: латиница -> кириллица
	translitRu map[rune]rune // Маппер раскладки: кириллица -> латиница
}

func NewT9PrefixEngine() *T9PrefixEngine {
	engine := &T9PrefixEngine{
		root:       &TrieNode{Children: make(map[rune]*TrieNode)},
		translitEn: make(map[rune]rune),
		translitRu: make(map[rune]rune),
	}
	engine.bootstrapLayoutMappers()
	return engine
}

// Insert добавляет новое легитимное b2b-слово в префиксное дерево за O(K)
func (e *T9PrefixEngine) Insert(word string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	runes := []rune(word)
	current := e.root

	for _, char := range runes {
		if _, exists := current.Children[char]; !exists {
			current.Children[char] = &TrieNode{Children: make(map[rune]*TrieNode)}
		}
		current = current.Children[char]
	}
	current.IsWord = true
	current.WordValue = word
}

// Search осуществляет наносекундный поиск подсказки автодополнения с авто-исправлением раскладки
// Search performs nanosecond autocomplete lookups with integrated keyboard layout normalizations
func (e *T9PrefixEngine) Search(prefix string) (string, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// 1. Пробуем прямой поиск по сырому префиксу
	if suggestion, found := e.searchTree(prefix); found {
		return suggestion, true
	}

	// Если слово не найдено, транслитерируем строку рун и делаем повторный Trie-пасс
	transliterated := e.normalizeLayout(prefix)
	if suggestion, found := e.searchTree(transliterated); found {
		return suggestion, true
	}

	return "", false
}

func (e *T9PrefixEngine) searchTree(prefix string) (string, bool) {
	runes := []rune(prefix)
	current := e.root

	// Проходим по символам за фиксированное время O(K)
	for _, char := range runes {
		if next, exists := current.Children[char]; exists {
			current = next
		} else {
			return "", false
		}
	}

	// Нашли узел префикса, теперь лениво спускаемся до первого валидного завершения слова
	return e.findFirstWordTail(current)
}

func (e *T9PrefixEngine) findFirstWordTail(node *TrieNode) (string, bool) {
	if node.IsWord {
		return node.WordValue, true
	}

	// Обходим дочерние узлы для вывода первой попавшейся Т9 подсказки
	for _, child := range node.Children {
		if word, found := e.findFirstWordTail(child); found {
			return word, true
		}
	}

	return "", false
}

func (e *T9PrefixEngine) normalizeLayout(input string) string {
	runes := []rune(input)
	output := make([]rune, len(runes))

	for i, char := range runes {
		if ruChar, exists := e.translitEn[char]; exists {
			output[i] = ruChar
		} else if enChar, exists := e.translitRu[char]; exists {
			output[i] = enChar
		} else {
			output[i] = char // Если символа нет в маппере, оставляем как есть
		}
	}
	return string(output)
}

func (e *T9PrefixEngine) bootstrapLayoutMappers() {
	en := []rune("qwertyuiop[]asdfghjkl;'zxcvbnm,.QWERTYUIOP{}ASDFGHJKL:\"ZXCVBNM<>")
	ru := []rune("йцукенгшщзхъфывапролджэячсмитьбюЙЦУКЕНГШЩЗХЪФЫВАПРОЛДЖЭЯЧСМИТЬБЮ")

	for i := 0; i < len(en) && i < len(ru); i++ {
		e.translitEn[en[i]] = ru[i]
		e.translitRu[ru[i]] = en[i]
	}
}
