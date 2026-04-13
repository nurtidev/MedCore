package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"

	"github.com/nurtidev/medcore/internal/analytics/handler"
	"github.com/nurtidev/medcore/internal/analytics/repository"
	"github.com/nurtidev/medcore/internal/analytics/service"
	"github.com/nurtidev/medcore/internal/analytics/worker"
	"github.com/nurtidev/medcore/internal/shared/database"
	"github.com/nurtidev/medcore/internal/shared/kafka"
	"github.com/nurtidev/medcore/internal/shared/logger"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// ── Config ────────────────────────────────────────────────────────────────
	cfgPath := envOr("CONFIG_PATH", "configs/analytics.yaml")
	cfg := loadConfig(cfgPath)

	// ── Logger ────────────────────────────────────────────────────────────────
	log := logger.New(cfg.logLevel, cfg.logFormat)
	ctx = logger.WithContext(ctx, log)

	// ── ClickHouse ────────────────────────────────────────────────────────────
	chConn, err := database.NewClickHouseConn(ctx, database.ClickHouseConfig{
		DSN:          cfg.clickhouseDSN,
		MaxOpenConns: cfg.clickhouseMaxConns,
		DialTimeout:  cfg.clickhouseDialTimeout,
	})
	must(log, err, "connect clickhouse")
	defer chConn.Close()

	// ── Redis ─────────────────────────────────────────────────────────────────
	rdb, err := database.NewRedisClient(ctx, database.RedisConfig{
		Addr:     cfg.redisAddr,
		Password: cfg.redisPassword,
		DB:       cfg.redisDB,
	})
	must(log, err, "connect redis")
	defer rdb.Close()

	// ── Kafka consumer ────────────────────────────────────────────────────────
	kafkaConsumer, err := kafka.NewConsumer(cfg.kafkaBrokers, cfg.kafkaGroupID)
	must(log, err, "create kafka consumer")
	defer kafkaConsumer.Close()

	if err := kafkaConsumer.Subscribe(cfg.kafkaTopics); err != nil {
		log.Fatal().Err(err).Msg("subscribe kafka topics")
	}

	// ── Service ───────────────────────────────────────────────────────────────
	repo := repository.NewClickHouseRepo(chConn)
	svc := service.New(repo, rdb)

	// ── HTTP server ───────────────────────────────────────────────────────────
	jwtSecret := []byte(cfg.jwtSecret)
	httpHandler := handler.NewHTTP(svc, jwtSecret, log)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/", httpHandler)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.httpPort),
		Handler:      mux,
		ReadTimeout:  cfg.readTimeout,
		WriteTimeout: cfg.writeTimeout,
	}

	// ── Workers ───────────────────────────────────────────────────────────────
	consumer := worker.NewKafkaConsumer(
		kafkaConsumer,
		svc,
		log,
		cfg.consumerBatchSize,
		cfg.consumerFlushInterval,
	)

	cronWorker := worker.NewCronWorker(svc, log)

	// ── Start ─────────────────────────────────────────────────────────────────
	go func() {
		log.Info().Int("port", cfg.httpPort).Msg("HTTP server starting")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("HTTP server failed")
		}
	}()

	go func() {
		log.Info().Msg("kafka consumer starting")
		if err := consumer.Run(ctx); err != nil && err != context.Canceled {
			log.Error().Err(err).Msg("kafka consumer exited")
		}
	}()

	go func() {
		cronWorker.Run(ctx)
	}()

	// ── Graceful shutdown ─────────────────────────────────────────────────────
	<-ctx.Done()
	log.Info().Msg("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("HTTP shutdown error")
	}

	log.Info().Msg("shutdown complete")
}

// ─── Config loading ───────────────────────────────────────────────────────────

type analyticsConfig struct {
	httpPort    int
	readTimeout  time.Duration
	writeTimeout time.Duration

	clickhouseDSN          string
	clickhouseMaxConns     int
	clickhouseDialTimeout  time.Duration

	redisAddr     string
	redisPassword string
	redisDB       int

	kafkaBrokers  string
	kafkaGroupID  string
	kafkaTopics   []string

	consumerBatchSize     int
	consumerFlushInterval time.Duration

	jwtSecret string

	logLevel  string
	logFormat string
}

func loadConfig(path string) analyticsConfig {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		l := zerolog.New(os.Stderr).With().Timestamp().Logger()
		l.Fatal().Err(err).Str("path", path).Msg("read config")
	}

	dialTimeout, _ := time.ParseDuration(v.GetString("clickhouse.dial_timeout"))
	if dialTimeout == 0 {
		dialTimeout = 10 * time.Second
	}
	flushInterval, _ := time.ParseDuration(v.GetString("consumer.flush_interval"))
	if flushInterval == 0 {
		flushInterval = 5 * time.Second
	}
	readTimeout, _ := time.ParseDuration(v.GetString("server.read_timeout"))
	if readTimeout == 0 {
		readTimeout = 30 * time.Second
	}
	writeTimeout, _ := time.ParseDuration(v.GetString("server.write_timeout"))
	if writeTimeout == 0 {
		writeTimeout = 30 * time.Second
	}

	httpPort := v.GetInt("server.http_port")
	if httpPort == 0 {
		httpPort = 8084
	}

	batchSize := v.GetInt("consumer.batch_size")
	if batchSize == 0 {
		batchSize = 100
	}

	return analyticsConfig{
		httpPort:     httpPort,
		readTimeout:  readTimeout,
		writeTimeout: writeTimeout,

		clickhouseDSN:         v.GetString("clickhouse.dsn"),
		clickhouseMaxConns:    v.GetInt("clickhouse.max_open_conns"),
		clickhouseDialTimeout: dialTimeout,

		redisAddr:     v.GetString("redis.addr"),
		redisPassword: v.GetString("redis.password"),
		redisDB:       v.GetInt("redis.db"),

		kafkaBrokers: v.GetString("kafka.brokers"),
		kafkaGroupID: v.GetString("kafka.group_id"),
		kafkaTopics:  v.GetStringSlice("kafka.topics"),

		consumerBatchSize:     batchSize,
		consumerFlushInterval: flushInterval,

		jwtSecret: v.GetString("jwt.secret"),

		logLevel:  v.GetString("log.level"),
		logFormat: v.GetString("log.format"),
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func must(log zerolog.Logger, err error, msg string) {
	if err != nil {
		log.Fatal().Err(err).Msg(msg)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
