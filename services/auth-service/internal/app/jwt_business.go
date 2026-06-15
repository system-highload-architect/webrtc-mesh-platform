package app

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"webrtc-mesh-platform/services/auth-service/internal/domain"
)

// AuthenticateUser проверяет хэш пароля и генерирует пуленепробиваемый b2b JWT-токен за наносекунды (Req. 5)
func (s *AuthService) AuthenticateUser(ctx context.Context, email, passwordRaw string) (string, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var targetProfile *domain.SubscriberAuthProfile
	for _, p := range s.profiles {
		if p.Email == email {
			targetProfile = p
			break
		}
	}

	if targetProfile == nil {
		return "", 0, errors.New("subscriber identity not found within SPR database")
	}

	hasher := sha256.New()
	hasher.Write([]byte(passwordRaw))
	if hexHash := fmt.Sprintf("%x", hasher.Sum(nil)); targetProfile.PasswordHash != hexHash {
		return "", 0, errors.New("invalid credentials password mismatch")
	}

	duration := 2 * time.Hour
	expiresAt := time.Now().Add(duration).Unix()

	token, err := s.generateHmacJwt(targetProfile.UserID, targetProfile.Email, targetProfile.Role, expiresAt)
	if err != nil {
		return "", 0, err
	}

	s.log.Info("AUTH SUCCESS -> Сгенерирован защищенный JWT для [%s] с ролью [%s]", email, targetProfile.Role)
	return token, expiresAt, nil
}

// generateHmacJwt собирает и подписывает JWT-токен личности по b2b-стандарту RFC 7519
func (s *AuthService) generateHmacJwt(userID, email, role string, expiresAt int64) (string, error) {
	header := `{"alg":"HS256","typ":"JWT"}`
	payload := fmt.Sprintf(`{"user_id":"%s","email":"%s","role":"%s","exp":%d}`, userID, email, role, expiresAt)

	encodedHeader := base64.RawURLEncoding.EncodeToString([]byte(header))
	encodedPayload := base64.RawURLEncoding.EncodeToString([]byte(payload))

	signatureInput := encodedHeader + "." + encodedPayload
	mac := hmac.New(sha256.New, s.jwtSecret)
	mac.Write([]byte(signatureInput))
	encodedSignature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return signatureInput + "." + encodedSignature, nil
}
