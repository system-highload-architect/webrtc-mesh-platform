# Stage 1: Build execution context inside Go Workspaces ring
FROM golang:1.25-alpine AS builder
WORKDIR /build

# Копируем go.work и манифесты всех модулей для кэширования слоев
COPY go.work ./
COPY internal/go.mod ./internal/
COPY pb/go.mod ./pb/
COPY services/cloud-routing-proxy/go.mod ./services/cloud-routing-proxy/
COPY services/signaling-gateway/go.mod ./services/signaling-gateway/
COPY services/spr-storage/go.mod ./services/spr-storage/
COPY services/chat-history-service/go.mod ./services/chat-history-service/
COPY services/auth-service/go.mod ./services/auth-service/

# ИСПРАВЛЕНО (Стандарт Go Workspaces): Используем go mod download для монорепозитория
# FIXED: Replaced invalid go work token with native go mod download execution context
RUN go mod download

# Копируем исходный код всего монорепозитория
COPY . .

# Собираем бинарник целевого сервиса
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o proxy-service services/cloud-routing-proxy/cmd/main.go

# Stage 2: Minimal runner
FROM alpine:3.19
WORKDIR /app
COPY --from=builder /build/proxy-service .
COPY --from=builder /build/services/signaling-gateway/web ./services/signaling-gateway/web
COPY --from=builder /build/services/cloud-routing-proxy/config.yaml ./services/cloud-routing-proxy/config.yaml

EXPOSE 8080
CMD ["./proxy-service"]
