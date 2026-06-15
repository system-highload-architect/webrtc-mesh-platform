package app

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// TokenClaims описывает полезную нагрузку извлекаемого JWT-токена абонента (Req. 5)
type TokenClaims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"` // "ORGANIZER" (Владелец) / "EMPLOYEE" (Участник)
	Exp    int64  `json:"exp"`
}

// ValidateAndParseJwt проверяет криптографическую подпись HMAC-SHA256 токена и извлекает роли без похода в БД
func (s *SignalingService) ValidateAndParseJwt(tokenStr string) (*TokenClaims, error) {
	// 1. Разбираем JWT на три каноничные POSIX-части (Header, Payload, Signature)
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid token topology structure mismatch")
	}

	encodedHeader := parts[0]
	encodedPayload := parts[1]
	encodedSignature := parts[2]

	// 2. ПАТТЕРН БЕЗОПАСНОСТИ (Req. 5): Проверяем подпись токена на ключе авторизационного кластера
	// Мы используем тот же секретный ключ, что зашит на стороне auth-service
	jwtSecret := []byte("webrtc_cluster_jwt_protected_secret_key")
	signatureInput := encodedHeader + "." + encodedPayload

	mac := hmac.New(sha256.New, jwtSecret)
	mac.Write([]byte(signatureInput))
	expectedSignature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(encodedSignature), []byte(expectedSignature)) {
		return nil, errors.New("cryptographic token signature verification failed")
	}

	// 3. Декодируем Base64 полезной нагрузки в доменную структуру Go
	payloadBytes, err := base64.RawURLEncoding.DecodeString(encodedPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 token payload: %v", err)
	}

	claims := &TokenClaims{}
	if err := json.Unmarshal(payloadBytes, claims); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON claims profile: %v", err)
	}

	// 4. Проверяем TTL токена на предмет протухания сессии
	if time.Now().Unix() > claims.Exp {
		return nil, errors.New("token lifespan has expired log in again")
	}

	return claims, nil
}
