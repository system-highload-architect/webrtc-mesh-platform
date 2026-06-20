# 🚀 CLUSTER BUILDING & INITIALIZATION REGULATION / РЕГЛАМЕНТ СБОРКИ И ИНИЦИАЛИЗАЦИИ КЛАССТЕРА CLEARWAY PKI MESH

Distributed Operator-Class Infrastructure Deployment Manual for Control, User, and Data Plane Microservices.
Исчерпывающий пошаговый регламент развёртывания, низкоуровневой сборки графа зависимостей Go Workspaces и запуска всех 5 микросервисов плоскостей Control, User и Data Plane.

---

## 🛠️ PHASE 1. COLD INITIALIZATION OF GO WORKSPACES / ЭТАП 1. ХОЛОДНАЯ ИНИЦИАЛИЗАЦИЯ GO WORKSPACES (ЛОКАЛЬНЫЙ ГРАФ)

* 🇺🇸 **EN:** if you are launching the project for the first time or have updated the Protobuf contracts, execute the following commands in the Git Bash terminal from the repository root for canonical module stitching:
* 🇷🇺 **RU:** если вы запускаете проект впервые или обновили Protobuf-контракты, выполните следующие команды в терминале Git Bash из корня репозитория для канонической сшивки модулей:

```bash
# 1. Entering the pb directory and initializing the Protobuf contracts module
# 1. Заходим в папку pb и инициализируем модуль контрактов Protobuf
cd pb
go mod init webrtc-mesh-platform/pb
go mod tidy
cd ..

# 2. Entering the internal directory and initializing the system chassis and shutdown module
# 2. Заходим в папку internal и инициализируем модуль системного шасси и shutdown
cd internal
go mod init webrtc-mesh-platform/internal
go mod tidy
cd ..

# 3. Completely wiping out the obsolete local go.work file (if it existed)
# 3. Полностью сносим старый дисковый go.work (если он существовал)
rm -f go.work

# 4. Initializing a clean b2b workspace area
# 4. Инициализируем чистое b2b рабочее пространство
go work init

# 5. Enforcing inclusion of all 7 named modules into the Go Work compilation context
# 5. Принудительно подключаем все 7 именованных модулей в контур сборки Go Work
go work use ./services/auth-service
go work use ./services/chat-history-service
go work use ./services/cloud-routing-proxy
go work use ./services/signaling-gateway
go work use ./services/spr-storage
go work use ./pb
go work use ./internal

# 6. Structurally synchronizing named import graphs across the cluster modules
# 6. Намертво синхронизируем именованные графы импортов кластера
go work sync
```

---

## 🐳 PHASE 2. ORCHESTRATION & DEPLOYMENT VIA DOCKER MESH / ЭТАП 2. ОРКЕСТРАЦИЯ И РАЗВЕРТЫВАНИЕ В ИЗОЛИРОВАННОМ DOCKER-ОКРУЖЕНИИ

* 🇺🇸 **EN:** the cluster is entirely Cloud-Native. All 5 services are compiled using optimized multi-stage Dockerfiles that strip Go source code and leave minimal production binaries weighing around 15–20 MB.
* 🇷🇺 **RU:** кластер полностью Cloud-Native. Все 5 сервисов собираются через оптимизированные двухэтапные (`Multi-stage`) Dockerfile, отсекающие исходный код Go и оставляющие бинарники весом по 15–20 МБ.

### Local launch of the signaling core (Fallback-debugging / Локальный запуск шлюза сигнализации):
```bash
go run ./services/signaling-gateway/cmd/main.go
```

### Ultimate launch of all 5 services inside the Docker network / Ультимативный старт всех 5 сервисов в Docker-сети:
```bash
# Building images with layer caching via go mod download inside the containers
# Сборка имиджей с кэшированием слоев через go mod download внутри контейнеров
docker-compose up --build
```

### Background initialization (Daemon mode / Фоновый запуск):
```bash
docker-compose up -d
```

### Real-time logs aggregation monitoring / Мониторинг системных логов в реальном времени:
```bash
docker-compose logs -f
```

---

## 🌐 MESH NETWORK PORT SPECIFICATION / СПЕЦИФИКАЦИЯ СЕТЕВЫХ ПОРТОВ DOCKER-СЕТКИ `clearway-mesh-network`

* **`cloud-routing-proxy`**: `:8080` (Central Entrance Ingress / Единый Контур Входа REST/WebSockets Proxy)
* **`signaling-gateway`**: `:8081` (Control Plane Room Signaling / Шлюз Сигнализации WebRTC)
* **`auth-service`**: `:8082` (gRPC Endpoint Node / gRPC Аутентификация PKI Ролей и JWT)
* **`chat-history-service`**: `:8083` (gRPC Workspace Engine / gRPC Предикативный Т9 и Архивация Логов)
* **`spr-storage`**: `:50060` (gRPC Production Block-DB / gRPC Client Streaming NVMe-Видеорекордер)
