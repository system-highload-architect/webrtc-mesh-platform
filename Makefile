.PHONY: proto run-local stop-local help

proto:
	@echo "⚙️ Компиляция WebRTC Protobuf контрактов в Go-структуры..."
	@mkdir -p pb/gen
	@protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative pb/*.proto
	@mkdir -p pb/gen
	@mv pb/pb/gen/* pb/gen/ 2>/dev/null || true
	@rm -rf pb/pb 2>/dev/null || true
	@echo "✅ Контракты успешно скомпилированы в pb/gen/"

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
