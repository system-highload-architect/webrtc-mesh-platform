# 📝 SPECIFICATION: CHAT HISTORY SERVICE / СЕРВИС ИСТОРИИ ЧАТА И Т9

[English version below]

## 🇷🇺 РУССКАЯ ВЕРСИЯ
Микросервис `chat-history-service` (Порт `:8083`) обрабатывает персистентные транзакции логов сообщений и обеспечивает наносекундный предикативный Т9-ввод на базе изолированного суффиксного графа `Trie Engine` [2.1].

### 📊 Диаграмма наносекундной автоподстановки Т9 (T9 Prediction Pipeline):
```mermaid
sequenceDiagram
    autonumber
    participant UI as 📱 Браузер (UI Чат)
    participant Proxy as 🌐 cloud-routing-proxy
    participant ChatService as 📝 chat-history-service
    participant Trie as 🧬 Trie Graph Object

    UI->>Proxy: HTTP GET /api/v1/t9?prefix=при
    Proxy->>ChatService: gRPC: QueryT9Autocomplete({Prefix: "при"})
    ChatService->>Trie: Локальный спуск по рунам поинтеров: 'п'->'р'->'и'
    Trie->>Trie: Извлечение предрассчитанного Suggestion поля из ноды
    Trie-->>ChatService: Возврат строки: "привет" (is_found = true)
    ChatService-->>Proxy: Protobuf: T9QueryResponse({Suggestion: "привет"})
    Proxy-->>UI: HTTP 200 OK Text: "привет" (UI мгновенно подставляет слово)
```

---

## 🇺🇸 ENGLISH VERSION
The `chat-history-service` cluster entity (Port `:8083`) powers the text prediction mechanics and structures persistent telemetry streams via gRPC [2.1].

* **Algorithmic Decoupling**: offloads typing logic from the web socket signalling nodes to secure a strict bounded SLA.
* **Zero-regex Normalization**: matches inbound rune paths directly against pre-compiled prefix trees, ensuring predictable lookup execution times.
