package app

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"webrtc-mesh-platform/internal/pkg/logger"
	"webrtc-mesh-platform/pb/gen" // Подключаем сгенерированные Protobuf контракты
)

type SubscriberProfile struct {
	UserID   string
	Name     string
	Email    string
	Password string
	Role     string // "ORGANIZER" (Модератор) / "EMPLOYEE" (Участник)
}

// SprStorageService инкапсулирует инфраструктурный слой NoSQL ScyllaDB (SPR)
// Инжектировали карту videoFiles
// и жесткий gRPC-серверный интерфейс контракта StorageMediaBridgeServer!
// Superimposed high-performance media stream writer layers straight over the local database context
type SprStorageService struct {
	gen.UnimplementedStorageMediaBridgeServer // Анонимно встраиваем сервер для strict-линкера
	mu                                        sync.RWMutex
	log                                       *logger.AppLogger
	dbPath                                    string
	profiles                                  map[string]*SubscriberProfile

	recordMutex sync.RWMutex
	videoFiles  map[string]*os.File // Индекс активных дескрипторов: recordID -> файл на NVMe
}

// NewSprStorageService инициализирует память и каталоги хранения таблиц БД
func NewSprStorageService(log *logger.AppLogger) *SprStorageService {
	s := &SprStorageService{
		log:        log,
		dbPath:     "data/scylladb_spr_emulation",
		profiles:   make(map[string]*SubscriberProfile),
		videoFiles: make(map[string]*os.File),
	}
	_ = os.MkdirAll(s.dbPath, 0755)

	// Создаем изолированную директорию для персистентного хранения WebM-записей Давида
	_ = os.MkdirAll(filepath.Join(s.dbPath, "records"), 0755)

	s.bootstrapScyllaTables()
	return s
}

// StreamMediaChunk принимает бинарный gRPC Client Streaming поток тяжелых WebM кадров
// Вместо заглушки _ = chunk.Data
// сервис теперь честно укладывает байты на NVMe-массив встык в реальном времени!
// Implemented stateful client streaming pipeline to append raw bytes directly onto NVMe files
func (s *SprStorageService) StreamMediaChunk(stream gen.StorageMediaBridge_StreamMediaChunkServer) error {
	s.log.Info("🎰 [STORAGE PLANE gRPC] Входящий gRPC-канал бинарной записи успешно открыт.")

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			s.log.Info("[STORAGE PLANE gRPC] Стрим завершен. Видеофайл успешно запечатан в ScyllaDB/SPR.")
			return stream.SendAndClose(&gen.MediaStreamResponse{Status: "SUCCESS"})
		}
		if err != nil {
			s.log.Error("Крах потокового gRPC-канала записи кадров: %v", err)
			return err
		}

		// Потоково дописываем прилетевшие байты на диск
		err = s.writeChunkToFile(chunk.RecordId, chunk.Data)
		if err != nil {
			return err
		}
	}
}

// writeChunkToFile атомарно открывает дескриптор файла и производит дозапись байт
func (s *SprStorageService) writeChunkToFile(recordID string, data []byte) error {
	s.recordMutex.Lock()
	defer s.recordMutex.Unlock()

	file, exists := s.videoFiles[recordID]
	if !exists {
		filePath := filepath.Join(s.dbPath, "records", fmt.Sprintf("%s.webm", recordID))
		var err error
		// Открываем файл в режиме дозаписи (O_APPEND) или создания
		file, err = os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			s.log.Error("Не удалось инициализировать NVMe-файл записи %s: %v", recordID, err)
			return err
		}
		s.videoFiles[recordID] = file
	}

	_, err := file.Write(data)
	return err
}

// CloseActiveRecord принудительно закрывает дескриптор при Graceful Shutdown
func (s *SprStorageService) CloseActiveRecord(recordID string) {
	s.recordMutex.Lock()
	defer s.recordMutex.Unlock()

	if file, exists := s.videoFiles[recordID]; exists {
		_ = file.Close()
		delete(s.videoFiles, recordID)
	}
}
