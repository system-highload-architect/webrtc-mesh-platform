package app

import (
	"fmt"
	"hash/fnv"
	"sort"
	"strconv"
	"sync"
)

type ConsistentHashRing struct {
	mu           sync.RWMutex
	vNodesFactor int               // Количество виртуальных узлов для ровного распределения
	ring         []uint32          // Кольцо хэшей
	vNodeToNode  map[uint32]string // Маппинг хэша вирт-узла на реальный адрес ноды
}

func NewConsistentHashRing(nodes []string) *ConsistentHashRing {
	r := &ConsistentHashRing{
		vNodesFactor: 50, // Каждая физическая нода плодит 50 виртуальных точек на кольце
		vNodeToNode:  make(map[uint32]string),
	}
	for _, node := range nodes {
		r.AddNode(node)
	}
	return r
}

func (r *ConsistentHashRing) hash(key string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return h.Sum32()
}

func (r *ConsistentHashRing) AddNode(node string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := 0; i < r.vNodesFactor; i++ {
		vNodeKey := node + "#" + strconv.Itoa(i)
		vHash := r.hash(vNodeKey)
		r.ring = append(r.ring, vHash)
		r.vNodeToNode[vHash] = node
	}
	sort.Slice(r.ring, func(i, j int) bool { return r.ring[i] < r.ring[j] })
}

// RouteRoom находит ближайший узел по часовой стрелке на кольце хэшей за O(log N) (Consistent Hashing)
func (r *ConsistentHashRing) RouteRoom(roomID string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.ring) == 0 {
		return "", fmt.Errorf("consistent hash ring is empty, no target signaling nodes available")
	}

	roomHash := r.hash(roomID)

	// Бинарный поиск ближайшего узла на кольце (O(log N))
	idx := sort.Search(len(r.ring), func(i int) bool {
		return r.ring[i] >= roomHash
	})

	// Если дошли до конца кольца, закольцовываемся на первый элемент (0 узел)
	if idx == len(r.ring) {
		idx = 0
	}

	targetVNodeHash := r.ring[idx]
	return r.vNodeToNode[targetVNodeHash], nil
}
