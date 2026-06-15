package main

import (
	"context"
	"net"
	"time"

	"webrtc-mesh-platform/internal/chassis/config"
	"webrtc-mesh-platform/internal/pkg/logger"
	"webrtc-mesh-platform/internal/pkg/shutdown"
	"webrtc-mesh-platform/pb/gen"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SprServer struct {
	gen.UnimplementedAuthenticationBridgeServer
	log *logger.AppLogger
}

// GetSubscriberProfile имитирует жесткое извлечение b2b-паспорта из NoSQL ScyllaDB по PK (User_ID)
func (s *SprServer) GetSubscriberProfile(ctx context.Context, req *gen.ProfileRequest) (*gen.ProfileResponse, error) {
	s.log.Info("ScyllaDB SPR Query -> Чтение профиля по Primary Key [user_id: %s]", req.UserId)

	if req.UserId == "user_david" {
		return &gen.ProfileResponse{
			UserId:   "user_david",
			Name:     "Давид (Модератор)",
			Email:    "david@clearway.ru",
			UserRole: "ORGANIZER",
		}, nil
	} else if req.UserId == "user_employee" {
		return &gen.ProfileResponse{
			UserId:   "user_employee",
			Name:     "Константин (Участник)",
			Email:    "konstantin@clearway.ru",
			UserRole: "EMPLOYEE",
		}, nil
	}

	return nil, status.Errorf(codes.NotFound, "subscriber passport not found in ScyllaDB cluster data nodes")
}

func main() {
	cfg := config.LoadGlobalConfig("services/spr-storage/config.yaml")
	log := logger.NewAppLogger(cfg.ServiceName, cfg.LogLevel)
	log.Info("Запуск gRPC Эмулятора Базы Профилей ScyllaDB spr-storage...")

	server := grpc.NewServer()
	sprHandler := &SprServer{log: log}
	gen.RegisterAuthenticationBridgeServer(server, sprHandler)

	listener, err := net.Listen("tcp", cfg.BindAddr)
	if err != nil {
		log.Fatal("Не удалось открыть сетевой gRPC-порт %s: %v", cfg.BindAddr, err)
	}

	go func() {
		log.Info("gRPC spr-storage успешно запущен на %s", cfg.BindAddr)
		if err := server.Serve(listener); err != nil {
			log.Fatal("Крах рантайма gRPC spr-storage: %v", err)
		}
	}()

	shutdown.ListenSignals(log, server, time.Duration(cfg.ShutdownTimeout)*time.Second)
}
