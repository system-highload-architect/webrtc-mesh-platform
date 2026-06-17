package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	"webrtc-mesh-platform/internal/chassis/config"
	"webrtc-mesh-platform/internal/pkg/logger"
	"webrtc-mesh-platform/internal/pkg/shutdown"
	"webrtc-mesh-platform/pb/gen"
	"webrtc-mesh-platform/services/spr-storage/internal/app"

	"google.golang.org/grpc"
)

// b2bStorageServer реализует обновленный gRPC-интерфейс потоковой записи

type b2bStorageServer struct {
	gen.UnimplementedMediaSignalingBridgeServer // Фолбэк совместимости pb
	log                                         *logger.AppLogger
}

// StreamMediaChunk принимает непрерывный бинарный gRPC-поток фреймов от шлюза сигнализации
func (s *b2bStorageServer) StreamMediaChunk(stream gen.MediaSignalingBridge_StreamMediaChunkServer) error {
	var (
		file  *os.File
		err   error
		chunk *gen.MediaChunkRequest
	)

	defer func() {
		if file != nil {
			_ = file.Close()
		}
	}()

	for {
		chunk, err = stream.Recv()
		if err == io.EOF {
			// Поток завершен — закрываем файл и возвращаем SUCCESS-пакет
			return stream.SendAndClose(&gen.MediaStreamResponse{Status: "SUCCESS"})
		}
		if err != nil {
			return err
		}

		// ИНИЦИАЛИЗАЦИЯ: При первом пакете открываем дескриптор файла в папке хранения data/
		if file == nil {
			dirPath := filepath.Join("data", "video_records")
			_ = os.MkdirAll(dirPath, 0755)

			filePath := filepath.Join(dirPath, fmt.Sprintf("%s.webm", chunk.RecordId))
			file, err = os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				return fmt.Errorf("🔒 [SPR Storage Error]: Крах os.OpenFile: %v", err)
			}
			s.log.Info("💾 [STORAGE PLANE] Сервис spr-storage открыл NVMe-файл по gRPC: %s.webm", chunk.RecordId)
		}

		// Нативно пишем входящие сырые WebM-байты, прилетевшие из браузера Давида
		_, err = file.Write(chunk.Data)
		if err != nil {
			return fmt.Errorf("🔒 [SPR Storage Error]: Крах os.Write: %v", err)
		}
	}
}

func main() {
	cfg := config.LoadGlobalConfig("services/spr-storage/config.yaml")
	log := logger.NewAppLogger(cfg.ServiceName, cfg.LogLevel)
	log.Info("Запуск выделенного Storage Plane сервиса spr-storage (Порт :50060)...")

	var _ app.SprStorageEngine = app.NewSprStorageService(log)

	// Открываем TCP-слушатель на выделенном b2b-порту :50060
	listener, err := net.Listen("tcp", cfg.BindAddr)
	if err != nil {
		log.Fatal("Не удалось открыть сетевой gRPC-порт базы %s: %v", cfg.BindAddr, err)
	}

	server := grpc.NewServer()
	// Регистрируем наш gRPC-сервер записи в медиа-мост кластера
	gen.RegisterMediaSignalingBridgeServer(server, &b2bStorageServer{log: log})

	go func() {
		log.Info("📡 gRPC-сервер хранения spr-storage успешно запущен на порту :50060")
		if err := server.Serve(listener); err != nil {
			log.Fatal("Крах рантайма gRPC сервера spr-storage: %v", err)
		}
	}()

	shutdown.ListenSignals(log, server, time.Duration(cfg.ShutdownTimeout)*time.Second)
}
