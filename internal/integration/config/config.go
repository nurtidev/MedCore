package config

import (
	"fmt"
	"strings"
	"time"

	sharedcfg "github.com/nurtidev/medcore/internal/shared/config"
	"github.com/spf13/viper"
)

// EgovConfig — конфигурация eGov API.
type EgovConfig struct {
	APIURL  string        `mapstructure:"api_url"`
	APIKey  string        `mapstructure:"api_key"`
	Timeout time.Duration `mapstructure:"timeout"`
}

// DamumedConfig — конфигурация DAMUMED API.
type DamumedConfig struct {
	APIURL string `mapstructure:"api_url"`
	APIKey string `mapstructure:"api_key"`
}

// IDoctorConfig — конфигурация iDoctor агрегатора.
type IDoctorConfig struct {
	APIURL        string `mapstructure:"api_url"`
	WebhookSecret string `mapstructure:"webhook_secret"`
}

// OlympLabConfig — конфигурация лаборатории Олимп.
type OlympLabConfig struct {
	APIURL string `mapstructure:"api_url"`
	APIKey string `mapstructure:"api_key"`
}

// InvivoLabConfig — конфигурация лаборатории Инвиво.
type InvivoLabConfig struct {
	APIURL string `mapstructure:"api_url"`
	APIKey string `mapstructure:"api_key"`
}

// SyncConfig — конфигурация воркера синхронизации.
type SyncConfig struct {
	Interval time.Duration `mapstructure:"interval"`
}

// AuthGRPCConfig — адрес auth-service gRPC.
type AuthGRPCConfig struct {
	Addr string `mapstructure:"addr"`
}

// WebhookConfig — секреты для входящих webhooks.
type WebhookConfig struct {
	IDoctorSecret string `mapstructure:"idoctor_secret"`
	OlympSecret   string `mapstructure:"olymp_secret"`
	InvivoSecret  string `mapstructure:"invivo_secret"`
}

// Config — полная конфигурация integration-service.
type Config struct {
	Server   sharedcfg.ServerConfig   `mapstructure:"server"`
	Database sharedcfg.DatabaseConfig `mapstructure:"database"`
	Redis    sharedcfg.RedisConfig    `mapstructure:"redis"`
	Kafka    sharedcfg.KafkaConfig    `mapstructure:"kafka"`
	Log      sharedcfg.LogConfig      `mapstructure:"log"`

	Egov     EgovConfig    `mapstructure:"egov"`
	Damumed  DamumedConfig `mapstructure:"damumed"`
	IDoctor  IDoctorConfig `mapstructure:"idoctor"`
	OlympLab OlympLabConfig  `mapstructure:"olymp_lab"`
	InvivoLab InvivoLabConfig `mapstructure:"invivo_lab"`
	Sync     SyncConfig    `mapstructure:"sync"`
	AuthGRPC AuthGRPCConfig `mapstructure:"auth_grpc"`
	Webhook  WebhookConfig `mapstructure:"webhook"`
}

// Load загружает конфигурацию из YAML файла с поддержкой env-переменных.
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("integration/config.Load: read: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("integration/config.Load: unmarshal: %w", err)
	}

	return &cfg, nil
}
