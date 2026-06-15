package shutdown

import (
	"os"
	"os/signal"
	"syscall"
	"time"
	"webrtc-mesh-platform/internal/pkg/logger"
)

type GracefulServer interface {
	GracefulStop()
}

func ListenSignals(log *logger.AppLogger, server GracefulServer, timeout time.Duration) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	sig := <-sigChan
	log.Info("Получен системный сигнал ядра Linux [%v]. Инициирован Graceful Shutdown...", sig)

	stopChan := make(chan struct{})
	go func() {
		server.GracefulStop()
		close(stopChan)
	}()

	select {
	case <-stopChan:
		log.Info("Все сетевые сокеты и gRPC соединения успешно закрыты. Сервер остановлен штатно.")
	case <-time.After(timeout):
		log.Error("Превышен лимит таймаута принудительной остановки. Экстренное завершение процессов.")
	}
}
