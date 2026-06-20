# 🧪 FUNCTION SPECIFICATION: REACTIVE LRU CACHE / РЕАКТИВНЫЙ LRU-КЭШ

## 🇷🇺 РУССКАЯ ВЕРСИЯ
Компонент `internal/pkg/trie/lru.go` реализует реактивный LRU (Least Recently Used) кэш комнат. Предназначен для удержания горячих стейтов комнат в ОЗУ со сложностью операций \(O(1)\) [1.1].

### Схема структуры ОЗУ и связанных указателей:
```text
  Хэш-мапа (Go Map): [roomID] ➔ Указатель на ноду (*Node)
   │
   ▼
  Двусвязный список (Doubly Linked List):
  [ Head (Самая свежая комната) ] ⇄ [ Node 1 ] ⇄ [ Node 2 ] ⇄ [ Tail (Устаревшая комната) ]
```

### Диаграмма вызовов при обращении к кэшу (Eviction Pipeline):
```mermaid
sequenceDiagram
    autonumber
    participant App as 🎰 signaling-gateway
    participant LRU as 🧮 ReactiveLruCache
    participant Map as 🗺️ Go Internal Map
    participant List as 🔗 Doubly Linked List

    App->>LRU: Get("clearway_pki_session")
    LRU->>Map: Поиск ноды по ключу roomID
    alt Ключ найден (Кэш-хит)
        Map-->>LRU: Возврат *Node
        LRU->>List: Переместить ноду в начало списка (Head)
        LRU-->>App: Возврат объекта VideoRoom (Комната горячая)
    else Ключ не найден (Кэш-мисс)
        Map-->>LRU: Возврат nil
        LRU-->>App: Возврат nil (Инициация ленивого создания)
    end

    App->>LRU: Set("clearway_pki_session", roomObj)
    alt Превышена емкость кэша (Capacity > 1000)
        LRU->>List: Извлечь хвостовую ноду (Tail)
        List-->>LRU: Возврат вытесняемой ноды
        LRU->>Map: Удалить ключ из хэш-мапы (Защита от OOM)
    end
    LRU->>List: Добавить новую ноду в начало (Head)
    LRU->>Map: Записать указатель в мапу
```

---

## 🇺🇸 ENGLISH VERSION
Component `internal/pkg/trie/lru.go` implements an $O(1)$ Reactive LRU Cache combining a hash map for microsecond lookup and a Doubly Linked List for atomic memory evictions.

* Accessing an existing room pointer dynamically shifts its position to the `Head` node.
* Boundary overflow forces the scheduler to delete the `Tail` node, purging memory leaks.
