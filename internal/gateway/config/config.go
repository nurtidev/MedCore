package config

import (
	"fmt"
	"strings"
	"time"

	sharedcfg "github.com/nurtidev/medcore/internal/shared/config"
	"github.com/spf13/viper"
)

// Config holds all configuration for the API gateway.
type Config struct {
	Server    sharedcfg.ServerConfig `mapstructure:"server"`
	Redis     sharedcfg.RedisConfig  `mapstructure:"redis"`
	Log       sharedcfg.LogConfig    `mapstructure:"log"`
	Upstream  UpstreamConfig         `mapstructure:"upstream"`
	AuthGRPC  AuthGRPCConfig         `mapstructure:"auth_grpc"`
	RateLimit RateLimitConfig        `mapstructure:"rate_limit"`
	CORS      CORSConfig             `mapstructure:"cors"`
}

type UpstreamConfig struct {
	Auth        string         `mapstructure:"auth"`
	Billing     string         `mapstructure:"billing"`
	Integration string         `mapstructure:"integration"`
	Analytics   string         `mapstructure:"analytics"`
	Timeouts    TimeoutsConfig `mapstructure:"timeouts"`
}

type TimeoutsConfig struct {
	Default   time.Duration `mapstructure:"default"`
	Analytics time.Duration `mapstructure:"analytics"`
}

type AuthGRPCConfig struct {
	Addr    string        `mapstructure:"addr"`
	Timeout time.Duration `mapstructure:"timeout"`
}

type RateLimitConfig struct {
	GlobalRPM    int `mapstructure:"global_rpm"`
	LoginRPM     int `mapstructure:"login_rpm"`
	AnalyticsRPM int `mapstructure:"analytics_rpm"`
}

type CORSConfig struct {
	AllowedOrigins []string `mapstructure:"allowed_origins"`
	AllowedMethods []string `mapstructure:"allowed_methods"`
	AllowedHeaders []string `mapstructure:"allowed_headers"`
}

func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("gateway/config.Load: read: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("gateway/config.Load: unmarshal: %w", err)
	}

	return &cfg, nil
}
