package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type BaseConfig struct {
	ServiceName     string `yaml:"service_name"`
	LogLevel        string `yaml:"log_level"`
	BindAddr        string `yaml:"bind_addr"`
	ShutdownTimeout int    `yaml:"shutdown_timeout"`
}

type Config struct {
	BaseConfig        `yaml:",inline"`
	RoomCapacityLimit int `yaml:"room_capacity_limit"`
}

func LoadConfig(path string) *Config {
	cfg := &Config{
		RoomCapacityLimit: 1000,
	}

	data, err := os.ReadFile(path)
	if err != nil {
		cfg.ServiceName = "webrtc-signaling-gateway"
		cfg.BindAddr = ":50055"
		cfg.ShutdownTimeout = 5
		return cfg
	}

	_ = yaml.Unmarshal(data, cfg)
	return cfg
}
