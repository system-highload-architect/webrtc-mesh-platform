# 📋 SOFTWARE REQUIREMENT SPECIFICATION (SRS): CLOUD ROUTING PROXY

[English version below]

## 🇷🇺 РУССКАЯ ВЕРСИЯ

### 1. Функциональные требования и назначение
Микросервис `cloud-routing-proxy` обязан выступать в качестве единого бронированного L7 API Ingress-шлюза для всего внешнего трафика. Клиентские приложения (браузеры) не имеют права общаться с внутренними сервисами напрямую [2.1].

### 2. Жесткие b2b-ограничения и SLA
* **Консистентное распределение комнат**: распределение WebSocket-соединений обязано выполняться через алгоритм консистентного хэширования (`Consistent Hash Ring`) [2.1]. Одна и та же комната созвона должна намертво приземляться на один и тот же RAM-шард шлюза сигнализации, исключая межсерверный рассинхрон [2.1].
* **Пропускная способность и задержки**: шлюз обязан обрабатывать до 50 000 RPS входящего REST-трафика. Допустимый лимит задержки (SLA Latency Tail) на этапе проксирования не должен превышать **3.5 миллисекунды** для 99-го процентиля.
* **Таймауты стриминга**: для ручки `/api/v1/records/upload` таймаут чтения HTTP-тела (`ReadTimeout`) обязан составлять не менее **10 минут**, чтобы обеспечить непрерывную Client Streaming загрузку тяжелых WebM-монолитов [2.1].
* **Favicon-Защита**: сервер обязан аппаратно перехватывать запросы `/favicon.ico` и возвращать статус `204 No Content` без запуска каскадного шаблонизатора, предотвращая ложные OOM паники.

---

## 🇺🇸 ENGLISH VERSION

### 1. Functional Requirements & Scope
The `cloud-routing-proxy` component enforces a zero-trust network perimeter acting as the single multi-tenant Ingress gateway for all public-facing HTTP and WebSockets connections [2.1].

### 2. High-Availability SLA & Constraints
* **Stateful Locality Binding**: upstream target selection must follow a deterministic Consistent Hash Ring distribution to map identical Room IDs onto matching signaling memory shards [2.1].
* **Ingress Budget**: Maximum allowed reverse-proxy routing overhead is bound to $\le 3.5\text{ms}$ at p99.
* **Streaming Window**: HTTP payload chunking bounds request body consumption to a 500 MB capacity shield to mitigate memory-bloating buffer attacks.
