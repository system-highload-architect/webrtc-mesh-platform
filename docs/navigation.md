# 🗺️ ENTERPRISE SPECIFICATION INDEX / КАРТА СПЕЦИФИКАЦИЙ И ТЗ КЛАССТЕРА

[English version below]

## 🇷🇺 РУССКАЯ ВЕРСИЯ

Данный манифест является единой точкой входа в распределенную техническую документацию WebRTC-Mesh платформы Clearway PKI. Вся проектная документация разделена на требования (SRS) и архитектурные спецификации (Specification).

### 📋 1. ЭШЕЛОН ТЕХНИЧЕСКИХ ЗАДАНИЙ (DOCS/SRS/ - 7 ФАЙЛОВ)
* 🌐 [docs/srs/cloud-routing-proxy-srs.md](srs/cloud-routing-proxy-srs.md) — ТЗ: L7 API Балансировщик консистентного хэширования.
* 🎰 [docs/srs/signaling-gateway-srs.md](srs/signaling-gateway-srs.md) — ТЗ: Шлюз сигнализации Control Plane и лимиты комнат.
* 🔒 [docs/srs/auth-service-srs.md](srs/auth-service-srs.md) — ТЗ: gRPC-сервис PKI верификации и генерации JWT.
* 📝 [docs/srs/chat-history-service-srs.md](srs/chat-history-service-srs.md) — ТЗ: gRPC Data-Plane служба истории сообщений.
* 🗄️ [docs/srs/spr-storage-srs.md](srs/spr-storage-srs.md) — ТЗ: Распределенный gRPC-рекордер NVMe (ScyllaDB).
* 🧮 [docs/srs/core-pkg-srs.md](srs/core-pkg-srs.md) — ТЗ: Алгоритмические требования к модулям общего назначения internal/pkg.
* 📦 [docs/srs/infrastructure-srs.md](srs/infrastructure-srs.md) — ТЗ: Архитектура контейнеризации Docker и воркспейсов go.work.

### 📐 2. ЭШЕЛОН АРХИТЕКТУРНЫХ ОПИСАНИЙ (DOCS/SPECIFICATION/ - 8 ФАЙЛОВ)
* 🌐 [docs/specification/cloud-routing-proxy-spec.md](specification/cloud-routing-proxy-spec.md) — Спецификация: Контур Ingress, Consistent Hash Ring и роутинг.
* 🎰 [docs/specification/signaling-gateway-spec.md](specification/signaling-gateway-spec.md) — Спецификация: Оркестрация комнат и блокировки медиа-треков.
* 🔒 [docs/specification/auth-service-spec.md](specification/auth-service-spec.md) — Спецификация: Криптографические мосты и Protobuf-контракты.
* 📝 [docs/specification/chat-history-service-spec.md](specification/chat-history-service-spec.md) — Спецификация: Интеграция предикативного ввода Т9 и логирование.
* 🗄️ [docs/specification/spr-storage-spec.md](specification/spr-storage-spec.md) — Спецификация: Клиентский gRPC-стриминг WebM и Range-запросы.
* 🧪 [docs/specification/pkg-lru-spec.md](specification/pkg-lru-spec.md) — Спецификация функции: Реактивный LRU-кэш и Doubly Linked List.
* 🧬 [docs/specification/pkg-trie-spec.md](specification/pkg-trie-spec.md) — Спецификация функции: Нагруженное префиксное дерево Trie T9.
* ⏰ [docs/specification/pkg-timewheel-spec.md](specification/pkg-timewheel-spec.md) — Спецификация функции: Аппаратное Битовое Колесо Времени на uint64.

---

## 🇺🇸 ENGLISH VERSION

This manifest structures the complete systems engineering specification map for the Clearway PKI WebRTC Full-Mesh platform.

### 📋 1. SOFTWARE REQUIREMENT SPECIFICATIONS INTERFACE (DOCS/SRS/ - 7 FILES)
* [docs/srs/cloud-routing-proxy-srs.md](srs/cloud-routing-proxy-srs.md) — SRS: L7 API Edge Consistent Hashing Proxy.
* [docs/srs/signaling-gateway-srs.md](srs/signaling-gateway-srs.md) — SRS: Control Plane Signaling Core & Capacity Enforcement.
* [docs/srs/auth-service-srs.md](srs/auth-service-srs.md) — SRS: gRPC PKI Certificate Verification & JWT Identity Provider.
* [docs/srs/chat-history-service-srs.md](srs/chat-history-service-srs.md) — SRS: gRPC Data-Plane Chat Analytics Engine.
* [docs/srs/spr-storage-srs.md](srs/spr-storage-srs.md) — SRS: Client Streaming NVMe Record Vault (ScyllaDB Emulation).
* [docs/srs/core-pkg-srs.md](srs/core-pkg-srs.md) — SRS: Algorithmic constraints for standard internal/pkg modules.
* [docs/srs/infrastructure-srs.md](srs/infrastructure-srs.md) — SRS: Docker containerization topology and go.work orchestration layout.

### 📐 2. DETAILED ARCHITECTURAL IMPLEMENTATIONS (DOCS/SPECIFICATION/ - 8 FILES)
* [docs/specification/cloud-routing-proxy-spec.md](specification/cloud-routing-proxy-spec.md) — Spec: Edge Ingress Layer, Consistent Hash Ring, and reverse-proxy.
* [docs/specification/signaling-gateway-spec.md](specification/signaling-gateway-spec.md) — Spec: Room Orchestration Core, track muting, and lifecycle.
* [docs/specification/auth-service-spec.md](specification/auth-service-spec.md) — Spec: gRPC bridges implementation and Protobuf interfaces.
* [docs/specification/chat-history-service-spec.md](specification/chat-history-service-spec.md) — Spec: Predictive T9 completion and state logs aggregation.
* [docs/specification/spr-storage-spec.md](specification/spr-storage-spec.md) — Spec: Client-to-Server gRPC WebM stream slicing & block allocation.
* [docs/specification/pkg-lru-spec.md](specification/pkg-lru-spec.md) — Function Spec: Reactive LRU Cache & Doubly Linked List mapping.
* [docs/specification/pkg-trie-spec.md](specification/pkg-trie-spec.md) — Function Spec: Prefixed Trie T9 state graph implementation.
* [docs/specification/pkg-timewheel-spec.md](specification/pkg-timewheel-spec.md) — Function Spec: Bitmapped Time Wheel scheduler on uint64.
