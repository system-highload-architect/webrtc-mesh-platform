package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// GlobalConfig описывает эталонную промышленную матрицу настроек для всего WebRTC-кластера
type GlobalConfig struct {
	ServiceName       string `yaml:"service_name"`
	LogLevel          string `yaml:"log_level"`
	BindAddr          string `yaml:"bind_addr"`
	ShutdownTimeout   int    `yaml:"shutdown_timeout"`
	RoomCapacityLimit int    `yaml:"room_capacity_limit"` // Специфично для signaling-gateway (лимит 1000 комнат)
	DataDiskPath      string `yaml:"data_disk_path"`      // Специфично для chat-history-service (NVMe сегменты)
}

// LoadGlobalConfig атомарно читает YAML-профиль с диска по переданному b2b-пути
func LoadGlobalConfig(path string) *GlobalConfig {
	cfg := &GlobalConfig{
		ServiceName:       "webrtc-generic-node",
		BindAddr:          ":50050",
		ShutdownTimeout:   5,
		RoomCapacityLimit: 1000,
		DataDiskPath:      "data/chat_history_segments",
	}

	data, err := os.ReadFile(path)
	if err != nil {
		// Если файл не найден на диске, возвращаем безопасный дефолтный профиль
		return cfg
	}

	_ = yaml.Unmarshal(data, cfg)
	return cfg
}
