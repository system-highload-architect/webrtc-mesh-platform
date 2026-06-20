package trie

import (
	"sync"
	"time"
)

// CacheNode описывает узел двусвязного списка в RAM памяти
type CacheNode struct {
	Key        string
	Value      any
	LastAccess time.Time
	Prev       *CacheNode
	Next       *CacheNode
}

// ReactiveLruCache реализует пуленепробиваемый b2b LRU вытеснитель за O(1)
type ReactiveLruCache struct {
	mu       sync.RWMutex
	capacity int
	items    map[string]*CacheNode
	head     *CacheNode
	tail     *CacheNode
}

func NewReactiveLruCache(capacity int) *ReactiveLruCache {
	return &ReactiveLruCache{
		capacity: capacity,
		items:    make(map[string]*CacheNode),
	}
}

// Set добавляет или обновляет комнату, а при переполнении каскадно вытесняет хвост за O(1)
func (c *ReactiveLruCache) Set(key string, value any) string {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 1. Если элемент уже существует, обновляем значение и двигаем в голову (Head)
	if node, exists := c.items[key]; exists {
		node.Value = value
		node.LastAccess = time.Now()
		c.moveToHead(node)
		return ""
	}

	// 2. Если достигнут лимит capacity комнат — каскадно отсекаем самый старый узел с хвоста
	var evictedKey string
	if len(c.items) >= c.capacity {
		evictedKey = c.tail.Key
		c.removeNode(c.tail)
		delete(c.items, evictedKey)
	}

	// 3. Создаем новый узел и заводим его в голову списка
	newNode := &CacheNode{
		Key:        key,
		Value:      value,
		LastAccess: time.Now(),
	}
	c.addToHead(newNode)
	c.items[key] = newNode

	return evictedKey
}

// Get извлекает элемент и атомарно перемещает его в голову как самый активный за O(1)
func (c *ReactiveLruCache) Get(key string) (any, bool) {
	c.mu.Lock() // Лочим на запись, так как Get меняет указатели двусвязного списка!
	defer c.mu.Unlock()

	node, exists := c.items[key]
	if !exists {
		return nil, false
	}

	node.LastAccess = time.Now()
	c.moveToHead(node)
	return node.Value, true
}

// Remove принудительно удаляет узел из индексов памяти
func (c *ReactiveLruCache) Remove(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if node, exists := c.items[key]; exists {
		c.removeNode(node)
		delete(c.items, key)
	}
}

// Вспомогательные низкоуровневые b2b методы управления указателями списка
func (c *ReactiveLruCache) addToHead(node *CacheNode) {
	node.Next = c.head
	node.Prev = nil
	if c.head != nil {
		c.head.Prev = node
	}
	c.head = node
	if c.tail == nil {
		c.tail = node
	}
}

func (c *ReactiveLruCache) removeNode(node *CacheNode) {
	if node.Prev != nil {
		node.Prev.Next = node.Next
	} else {
		c.head = node.Next
	}
	if node.Next != nil {
		node.Next.Prev = node.Prev
	} else {
		c.tail = node.Prev
	}
}

func (c *ReactiveLruCache) moveToHead(node *CacheNode) {
	c.removeNode(node)
	c.addToHead(node)
}

// PeekTail атомарно возвращает ключ и значение самого старого элемента на дне списка (хвоста)
func (c *ReactiveLruCache) PeekTail() (string, any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.tail == nil {
		return "", nil, false
	}
	return c.tail.Key, c.tail.Value, true
}

// RemoveTail принудительно выжигает самый старый узел с самого низа списка и возвращает его ключ
func (c *ReactiveLruCache) RemoveTail() (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.tail == nil {
		return "", false
	}

	evictedKey := c.tail.Key
	c.removeNode(c.tail)
	delete(c.items, evictedKey)

	return evictedKey, true
}
