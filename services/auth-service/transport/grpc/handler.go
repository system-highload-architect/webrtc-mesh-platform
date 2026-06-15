package grpc

import (
	"context"

	"webrtc-mesh-platform/pb/gen" // Скомпилированные gRPC-контракты кластера
	"webrtc-mesh-platform/services/auth-service/internal/app"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GrpcHandler struct {
	// Анонимно встраиваем нереализованный сервер для прохождения strict-аудита линкера Go 1.26
	gen.UnimplementedAuthenticationBridgeServer
	service app.AuthUseCaseManager // Зависим строго от инвертированного интерфейса абстракции
}

func NewGrpcHandler(service app.AuthUseCaseManager) *GrpcHandler {
	return &GrpcHandler{service: service}
}

// LoginSubscriber принимает сырой пароль, проверяет хэш и возвращает подписанный JWT (Req. 5)
func (h *GrpcHandler) LoginSubscriber(ctx context.Context, req *gen.LoginRequest) (*gen.LoginResponse, error) {
	if req.Email == "" || req.PasswordRaw == "" {
		return nil, status.Error(codes.InvalidArgument, "Email or Password raw fields cannot be empty")
	}

	// Вызываем Use-Case слой строго через интерфейс
	token, expiresAt, err := h.service.AuthenticateUser(ctx, req.Email, req.PasswordRaw)
	if err != nil {
		return &gen.LoginResponse{
			IsSuccess: false,
			JwtToken:  "",
			ExpiresAt: 0,
		}, nil
	}

	return &gen.LoginResponse{
		IsSuccess: true,
		JwtToken:  token,
		ExpiresAt: expiresAt,
	}, nil
}

// GetSubscriberProfile извлекает паспорт пользователя из In-Memory ScyllaDB (SPR) структуры
func (h *GrpcHandler) GetSubscriberProfile(ctx context.Context, req *gen.ProfileRequest) (*gen.ProfileResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "User ID query key cannot be empty")
	}

	profile, err := h.service.FetchProfileByID(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "subscriber profile not found: %v", err)
	}

	return &gen.ProfileResponse{
		UserId:   profile.UserID,
		Name:     profile.Name,
		Email:    profile.Email,
		UserRole: profile.Role, // "ORGANIZER" (Модератор) / "EMPLOYEE" (Участник)
	}, nil
}
