# 🧬 FUNCTION SPECIFICATION: PREFIXED TRIE T9 / ПРЕФИКСНОЕ ДЕРЕВО TRIE T9

## 🇷🇺 РУССКАЯ ВЕРСИЯ
Компонент `internal/pkg/trie/t9.go` реализует структуру нагруженного суффиксного дерева для нахождений предикативного ввода слов чата со сложностью $O(K)$, где $K$ — длина префикса, полностью исключая перебор словаря ($O(N)$) [1.1].

### Граф состояний и переходов по рунам алфавита:
```mermaid
graph TD
    Root((Корневой узел)) -->|п| P(Нода 'п')
    P -->|р| PR(Нода 'пр')
    PR -->|и| PRI(Нода 'при')
    PRI -->|в| PRIV(Нода 'прив')
    PRIV -->|е| PRIVE(Нода 'приве')
    PRIVE -->|т| PRIVET(Нода 'привет')
    PRIVET -->|suggestion| S[Финальное слово: привет, is_end=true]
```

### Диаграмма наносекундной предикции gRPC-запроса:
```mermaid
sequenceDiagram
    autonumber
    participant Client as 🌐 cloud-routing-proxy
    participant Service as 📝 chat-history-service
    participant Trie as 🧬 TrieEngine

    Client->>Service: gRPC: QueryT9Autocomplete({"prefix": "пр"})
    Service->>Trie: Найти узел по рунам ['п', 'р']
    Trie->>Trie: Итерация по слайсу дочерних указателей Children
    alt Узел найден
        Trie-->>Service: Возврат ноды (Suggestion: "привет", IsFound: true)
        Service-->>Client: Protobuf: T9QueryResponse({"suggestion": "привет"})
    else Узел отсутствует
        Trie-->>Service: Возврат nil
        Service-->>Client: Protobuf: T9QueryResponse({"is_found": false})
    end
```

---

## 🇺🇸 ENGLISH VERSION
Component `internal/pkg/trie/t9.go` manages words predictive auto-completion inside `chat-history-service`. 

* Space complexity is optimized by mapping character bytes into nested arrays.
* Search runtime is bound strictly to prefix character length $O(K)$, ignoring raw dictionary scale metrics.
