# 1. Заходим в папку pb и инициализируем модуль контрактов Protobuf
cd pb
go mod init webrtc-mesh-platform/pb
go mod tidy
cd ..

# 2. Заходим в папку internal и инициализируем модуль системного шасси и shutdown
cd internal
go mod init webrtc-mesh-platform/internal
go mod tidy
cd ..

# 3. Полностью сносим старый дисковый go.work
rm -f go.work

# 4. Инициализируем чистое b2b рабочее пространство
go work init

# 5. Принудительно подключаем все 7 именованных модулей в контур сборки Go Work
go work use ./services/auth-service
go work use ./services/chat-history-service
go work use ./services/cloud-routing-proxy
go work use ./services/signaling-gateway
go work use ./services/spr-storage
go work use ./pb
go work use ./internal

# 6. Намертво синхронизируем именованные графы импортов кластера
go work sync
