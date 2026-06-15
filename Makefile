.PHONY: proto help

proto:
	@echo "⚙️ Компиляция WebRTC Protobuf контрактов в Go-структуры..."
	@mkdir -p pb/gen
	protoc --go_out=. --go_opt=paths=source_relative \
	       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
	       pb/*.proto
	@echo "✅ Контракты успешно скомпилированы в pb/gen/"

help:
	@echo "🎛️ WebRTC Mesh Platform Control Board:"
	@echo "  make proto  - Скомпилировать Protobuf контракты комнат"
