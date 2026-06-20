# 📋 SOFTWARE REQUIREMENT SPECIFICATION (SRS): INFRASTRUCTURE TOPOLOGY

[English version below]

## 🇷🇺 РУССКАЯ ВЕРСИЯ

### 1. Назначение и Контур Оркестрации Контейнеров
Инфраструктурный слой регламентирует правила контейнеризации всех 5 микросервисов платформы и архитектуру их сборки внутри Go Workspaces [2.1].

### 2. Требования к Изоляции и Сборке Кластера
* **Go Workspaces Монорепозиторий**: Все подмодули (`/services/*`, `/internal`, `/pb`) обязаны компилироваться внутри единого воркспейса `go.work`. Использование относительных путей `replace` внутри индивидуальных `go.mod` запрещено.
* **Multi-stage Контейнеризация**: Каждый Dockerfile обязан компилироваться в два этапа [2.1]. Первый этап (`builder` на базе Alpine Go) выкачивает вендоры через `go mod download`. Финальный этап копирует только скомпилированный бинарник весом до 20 МБ в чистый образ `alpine:3.19` [2.1].
* **Разделяемые Тома (Shared Volumes)**: Для обеспечения сквозного доступа к медиафайлам WebM, контейнеры `spr-storage` и `cloud-routing-proxy` обязаны монтировать общий именованный том `spr-nosql-data` по строго идентичному пути `/app/data/scylladb_spr_emulation` [2.1].
* **Изоляция сети**: Все контейнеры обязаны общаться через выделенный внутренний драйвер моста `clearway-mesh-network` без прямого проброса gRPC-портов во внешнюю сеть [2.1].

---

## 🇺🇸 ENGLISH VERSION

### 1. Scope & Core Orchestration Bounds
The deployment topology dictates operational isolation standards for the multi-tenant Go Workspace repository inside production micro-topologies [2.1].

### 2. Containerization and Workspace Constraints
* **Go Workspaces Enforcement**: Inter-module compilation is achieved using a top-level unified `go.work` schema file.
* **Multi-stage Slicing**: Every service image build separates development layers from deployment binaries, mapping the final container state straight onto ultra-lightweight `alpine:3.19` abstractions [2.1].
* **Volume Sharing Layout**: Mounts the identical local named volume `spr-nosql-data` inside both proxy and store nodes to ensure instantaneous file reads [2.1].
