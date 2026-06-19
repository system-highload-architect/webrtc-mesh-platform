FROM golang:1.25-alpine AS builder
WORKDIR /build

COPY go.work ./
COPY internal/go.mod ./internal/
COPY pb/go.mod ./pb/
COPY services/cloud-routing-proxy/go.mod ./services/cloud-routing-proxy/
COPY services/signaling-gateway/go.mod ./services/signaling-gateway/
COPY services/spr-storage/go.mod ./services/spr-storage/
COPY services/chat-history-service/go.mod ./services/chat-history-service/
COPY services/auth-service/go.mod ./services/auth-service/

RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o chat-service services/chat-history-service/cmd/main.go

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /build/chat-service .
COPY --from=builder /build/services/chat-history-service/config.yaml ./services/chat-history-service/config.yaml

EXPOSE 8083
CMD ["./chat-service"]
