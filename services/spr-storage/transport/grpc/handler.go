package grpc

import (
	"context"

	"webrtc-mesh-platform/pb/gen" // Скомпилированные Protobuf-контракты кластера
	"webrtc-mesh-platform/services/spr-storage/internal/app"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GrpcHandler реализует бинарный контракт плоскости данных ScyllaDB (SPR)
type GrpcHandler struct {
	gen.UnimplementedAuthenticationBridgeServer
	storage app.SprStorageEngine // Зависим строго от инвертированного интерфейса абстракции
}

func NewGrpcHandler(storage app.SprStorageEngine) *GrpcHandler {
	return &GrpcHandler{storage: storage}
}

// GetSubscriberProfile извлекает b2b-паспорт пользователя из дисковых NoSQL-таблиц по Primary Key (User_ID)
func (h *GrpcHandler) GetSubscriberProfile(ctx context.Context, req *gen.ProfileRequest) (*gen.ProfileResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "User ID key descriptor cannot be empty")
	}

	// Вызываем Use-Case слой строго через интерфейсный контракт
	profile, err := h.storage.GetProfileByID(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "ScyllaDB lookup failed: %v", err)
	}

	return &gen.ProfileResponse{
		UserId:   profile.UserID,
		Name:     profile.Name,
		Email:    profile.Email,
		UserRole: profile.Role, // "ORGANIZER" (Модератор) / "EMPLOYEE" (Участник)
	}, nil
}

// LoginSubscriber в данном сервисе переиспользуется для поиска абонента по уникальному индексу Email
func (h *GrpcHandler) LoginSubscriber(ctx context.Context, req *gen.LoginRequest) (*gen.LoginResponse, error) {
	if req.Email == "" {
		return nil, status.Error(codes.InvalidArgument, "Subscriber email index key cannot be empty")
	}

	// Вызываем Use-Case слой для поиска по Email-индексу
	profile, err := h.storage.GetProfileByEmail(ctx, req.Email)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "ScyllaDB index scan failed: %v", err)
	}

	// Возвращаем b2b-паспорт (пароль и роль) для верификации на стороне auth-service
	return &gen.LoginResponse{
		IsSuccess: true,
		JwtToken:  profile.Password, // Временно передаем хэш пароля в поле для сверки на стороне auth-service
		ExpiresAt: 0,                // Сигнализирует, что это промежуточный ответ паспорта из SPR
	}, nil
}
