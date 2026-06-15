package logger

import (
	"fmt"
	"log"
	"os"
	"time"
)

type AppLogger struct {
	serviceName string
	level       string
}

func NewAppLogger(serviceName, level string) *AppLogger {
	log.SetFlags(0)
	return &AppLogger{serviceName: serviceName, level: level}
}

func (l *AppLogger) logFormat(prefix, format string, v ...any) {
	timestamp := time.Now().Format("2006/01/02 15:04:05")
	msg := fmt.Sprintf(format, v...)
	fmt.Printf("%s [%s] %s\n", timestamp, prefix, msg)
}

func (l *AppLogger) Info(format string, v ...any)  { l.logFormat("INFO", format, v...) }
func (l *AppLogger) Error(format string, v ...any) { l.logFormat("ERROR", format, v...) }
func (l *AppLogger) Fatal(format string, v ...any) {
	l.logFormat("FATAL", format, v...)
	os.Exit(1)
}
