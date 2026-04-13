package config

import (
	"fmt"
	"strings"

	sharedcfg "github.com/nurtidev/medcore/internal/shared/config"
	"github.com/spf13/viper"
)

type KaspiConfig struct {
	APIURL     string `mapstructure:"api_url"`
	MerchantID string `mapstructure:"merchant_id"`
	SecretKey  string `mapstructure:"secret_key"`
}

type StripeConfig struct {
	SecretKey     string `mapstructure:"secret_key"`
	WebhookSecret string `mapstructure:"webhook_secret"`
}

type AuthGRPCConfig struct {
	Addr string `mapstructure:"addr"`
}

// Config holds all configuration for the billing service.
// Reuses shared sub-configs for consistency across services.
type Config struct {
	Server   sharedcfg.ServerConfig   `mapstructure:"server"`
	Database sharedcfg.DatabaseConfig `mapstructure:"database"`
	Kafka    sharedcfg.KafkaConfig    `mapstructure:"kafka"`
	Log      sharedcfg.LogConfig      `mapstructure:"log"`
	Kaspi    KaspiConfig              `mapstructure:"kaspi"`
	Stripe   StripeConfig             `mapstructure:"stripe"`
	AuthGRPC AuthGRPCConfig           `mapstructure:"auth_grpc"`
}

func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("billing/config.Load: read: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("billing/config.Load: unmarshal: %w", err)
	}

	return &cfg, nil
}
