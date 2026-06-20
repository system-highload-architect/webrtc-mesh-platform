# 📋 SOFTWARE REQUIREMENT SPECIFICATION (SRS): CHAT HISTORY SERVICE

[English version below]

## 🇷🇺 РУССКАЯ ВЕРСИЯ

### 1. Функциональные требования и назначение
Микросервис `chat-history-service` обязан выступать изолированным gRPC Data-Plane эшелоном для персистентного хранения истории текстовых коммуникаций и обеспечения наносекундного предиктивного ввода Т9 [2.1].

### 2. Метрики SLA и Бизнес-логика Т9
* **SLA на предиктивный ввод**: Предикативный gRPC-метод `QueryT9Autocomplete` обязан возвращать вариант автоподстановки слова из префиксного дерева Trie со скоростью не более **8 миллисекунд** для 99.9-го процентиля. Запрещено использовать регулярные выражения (RegEx) или тяжелые SQL-запросы `LIKE` на горячем пути поиска.
* **Изоляция нагрузки**: Модуль обязан быть полностью развязан по памяти с контуром сигнализации WebRTC. Крах или перегрузка чата не имеют права влиять на трансляцию видеопотоков участников конференции [2.1].
* **Санитизация логов**: Сервис обязан сохранять историю сообщений в ОЗУ-структурах шарда только после валидации контура API Gateway, гарантируя отсутствие SQL/NoSQL инъекций в теле лога.

---

## 🇺🇸 ENGLISH VERSION

### 1. Functional Scope & Requirements
The `chat-history-service` is engineered as a decoupled gRPC Data-Plane tier dedicated to indexing persistent message logs and managing real-time predictive text completion [2.1].

### 2. Core SLA & T9 Constraints
* **Sub-Millisecond Autocomplete**: The underlying `QueryT9Autocomplete` gRPC service must execute word matching within a $\le 8\text{ms}$ performance window at p99.9.
* **Failure Domain Isolation**: Text streaming faults or heavy analytics spikes must maintain 100% architectural independence from the active WebRTC signaling core [2.1].
