.PHONY: proto run-local stop-local help

# ИСПРАВЛЕНО (Пуленепробиваемая зачистка портов Windows / Linux): Жестко выжигаем процессы по номерам занятых портов
# FIXED: Reengineered task termination wrapper to kill zombie tasks by targeted TCP binding ports directly
ifeq ($(OS),Windows_NT)
    CLEAN_CMD = taskkill /F /IM main.exe /T 2>nul || true; taskkill /F /IM go.exe /T 2>nul || true
else
    CLEAN_CMD = pkill -f "meet-service" || true; fuser -k 8080/tcp 8081/tcp 8082/tcp 50051/tcp 50060/tcp 2>/dev/null || true
endif

proto:
	@echo "⚙️ Компиляция 5 декомпозированных Protobuf контрактов в единый Go-пакет gen..."
	@mkdir -p pb/gen
	@protoc --go_out=. --go-grpc_out=. pb/*.proto
	@mkdir -p pb/gen
	@mv pb/pb/gen/* pb/gen/ 2>/dev/null || true
	@rm -rf pb/pb 2>/dev/null || true
	@echo "✅ Все 5 b2b-контрактов успешно скомпилированы in pb/gen/"

run:
	@echo "🔥 ЗАПУСК ПЯТИСЕРВИСНОГО WEBRTC MESH КЛАСТЕРА В ПАРАЛЛЕЛЬНОМ КОНТУРЕ..."
	@echo "🧼 [0/5] Экстренное освобождение сетевых портов..."
	@$(CLEAN_CMD)
	@sleep 1
	@mkdir -p data/chat_history_segments data/video_records
	
	@echo "📡 [1/5] Запуск Identity Plane: auth-service..."
	@go run services/auth-service/cmd/main.go &
	
	@echo "📡 [2/5] Запуск Storage Plane эмулятора: spr-storage..."
	@go run services/spr-storage/cmd/main.go &
	
	@echo "📡 [3/5] Запуск Data Plane аналитики чата: chat-history-service..."
	@go run services/chat-history-service/cmd/main.go &
	
	@echo "📡 [4/5] Запуск Control Plane WebRTC шлюза: signaling-gateway..."
	@go run services/signaling-gateway/cmd/main.go &
	
	@echo "🚀 [5/5] Запуск API Gateway балансировщика: cloud-routing-proxy..."
	@go run services/cloud-routing-proxy/cmd/main.go &
	
	@echo "🎰 КЛАСТЕР УСПЕШНО РАЗВЕРНУТ. Единая точка входа: http://localhost:8080"
	@echo "🛑 Для плавного останова всех нод выполните команду: make stop"

stop:
	@echo "🛑 Экстренное глушение и деаллокация портов кластера..."
	@$(CLEAN_CMD)
	@echo "✅ Все сетевые сокеты и зомби-процессы успешно зачищены."

# Локальный сквозной запуск всех 3-х медиа-компонентов кластера в фоне одного терминала
run-local: stop-local
	@echo "🚀 [START] Запуск Control Plane плоскости (signaling-gateway) на порту :50055..."
	@go run services/signaling-gateway/cmd/main.go &
	@sleep 1
	@echo "📊 [START] Запуск Аналитического чат-слоя (chat-history-service) на порту :50057..."
	@go run services/chat-history-service/cmd/main.go &
	@sleep 1
	@echo "🔥 [START] Спусковой крючок: Включение нагрузочного client-emulator..."
	@go run services/client-emulator/cmd/main.go

# Умная b2b-очистка: освобождает строго порты WebRTC-кластера, не трогая родительский процесс make
stop-local:
	@echo "🛑 Остановка всех фоновых WebRTC Go-сервисов и зачистка сокетов..."
	@if command -v fuser >/dev/null 2>&1; then \
		fuser -k -n tcp 50055 50057 50058 >/dev/null 2>&1 || true; \
	elif command -v lsof >/dev/null 2>&1; then \
		lsof -ti :50055,:50057,:50058 | xargs kill -9 >/dev/null 2>&1 || true; \
	else \
		pkill -f "go run services/" || true; \
	fi
	@echo "✅ Все локальные медиа-процессы успешно зачищены."

help:
	@echo "🎛️ WEBRTC MESH PLATFORM CONTROL BOARD (GO 1.26):"
	@echo "  make proto      - Скомпилировать Protobuf контракты комнат"
	@echo "  make run-local  - Локальный запуск всех 3-х сервисов в фоне одного окна"
	@echo "  make stop-local - Принудительно убить все фоновые процессы медиа-сервисов"
