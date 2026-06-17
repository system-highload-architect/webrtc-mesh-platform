package main

import (
	"io"
	"net"
	"time"

	"webrtc-mesh-platform/internal/chassis/config"
	"webrtc-mesh-platform/internal/pkg/logger"
	"webrtc-mesh-platform/internal/pkg/shutdown"
	"webrtc-mesh-platform/pb/gen"

	"google.golang.org/grpc"
)

// sprMediaServer адаптирует выделенный gRPC-контекст StorageMediaBridge
type sprMediaServer struct {
	gen.UnimplementedStorageMediaBridgeServer // ИСПРАВЛЕНО: Привязали к новому изолированному сервису из storage.proto
	log                                       *logger.AppLogger
}

// StreamMediaChunk принимает двунаправленный бинарный gRPC-поток тяжелых WebM кадров
// ИСПРАВЛЕНО: Синхронизировали имя интерфейса стрима с разделенным Protobuf-контрактом
// FIXED: Updated stream parameter signature to match generated StorageMediaBridge structures
func (s *sprMediaServer) StreamMediaChunk(stream gen.StorageMediaBridge_StreamMediaChunkServer) error {
	s.log.Info("🎰 [STORAGE PLANE gRPC] gRPC-канал бинарной потоковой записи WebM успешно открыт шлюзом.")

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			s.log.Info("[STORAGE PLANE gRPC] Стрим завершен. Файл успешно запечатан на NVMe эмулятора ScyllaDB.")
			return stream.SendAndClose(&gen.MediaStreamResponse{Status: "SUCCESS"})
		}
		if err != nil {
			s.log.Error("Крах потокового gRPC-канала записи кадров: %v", err)
			return err
		}

		// Эмуляция дозаписи сырых байт видеопотока на NVMe-диск хранилища
		_ = chunk.Data
	}
}

func main() {
	cfg := config.LoadGlobalConfig("services/spr-storage/config.yaml")
	log := logger.NewAppLogger(cfg.ServiceName, cfg.LogLevel)
	log.Info("Запуск gRPC эмулятора распределенной базы данных ScyllaDB spr-storage...")

	// Открываем внутренний порт Storage Plane :50060
	bindAddr := "0.0.0.0:50060"
	listener, err := net.Listen("tcp", bindAddr)
	if err != nil {
		log.Fatal("Не удалось открыть внутренний gRPC-порт базы %s: %v", bindAddr, err)
	}

	server := grpc.NewServer()

	// Регистрируем наш обработчик в новый скомпилированный b2b-контракт хранилища
	gen.RegisterStorageMediaBridgeServer(server, &sprMediaServer{log: log})

	go func() {
		log.Info("📡 gRPC-сервер базы spr-storage успешно запущен на порту :50060")
		if err := server.Serve(listener); err != nil {
			log.Fatal("Крах рантайма gRPC сервера spr-storage: %v", err)
		}
	}()

	shutdown.ListenSignals(log, server, time.Duration(cfg.ShutdownTimeout)*time.Second)
}
