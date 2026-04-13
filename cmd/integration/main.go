package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"

	integcfg "github.com/nurtidev/medcore/internal/integration/config"
	"github.com/nurtidev/medcore/internal/integration/adapter/damumed"
	"github.com/nurtidev/medcore/internal/integration/adapter/egov"
	"github.com/nurtidev/medcore/internal/integration/adapter/idoctor"
	"github.com/nurtidev/medcore/internal/integration/adapter/invivo"
	"github.com/nurtidev/medcore/internal/integration/adapter/olymp"
	"github.com/nurtidev/medcore/internal/integration/handler"
	"github.com/nurtidev/medcore/internal/integration/repository"
	"github.com/nurtidev/medcore/internal/integration/service"
	"github.com/nurtidev/medcore/internal/integration/worker"
	"github.com/nurtidev/medcore/internal/shared/database"
	"github.com/nurtidev/medcore/internal/shared/kafka"
	"github.com/nurtidev/medcore/internal/shared/logger"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// ── Config ──────────────────────────────────────────────────────────────
	cfgPath := envOr("CONFIG_PATH", "configs/integration.yaml")
	cfg, err := integcfg.Load(cfgPath)
	must(err, "load config")

	// ── Logger ──────────────────────────────────────────────────────────────
	log := logger.New(cfg.Log.Level, cfg.Log.Format)
	ctx = logger.WithContext(ctx, log)

	// ── Postgres ────────────────────────────────────────────────────────────
	pool, err := database.NewPostgresPool(ctx, database.PostgresConfig{
		DSN:          cfg.Database.DSN,
		MaxOpenConns: cfg.Database.MaxOpenConns,
		MaxIdleConns: cfg.Database.MaxIdleConns,
	})
	must(err, "connect postgres")
	defer pool.Close()

	// ── Migrations ──────────────────────────────────────────────────────────
	if err := database.RunMigrations(ctx, pool, "migrations"); err != nil {
		log.Fatal().Err(err).Msg("run migrations")
	}

	// ── Redis ───────────────────────────────────────────────────────────────
	rdb, err := database.NewRedisClient(ctx, database.RedisConfig{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	must(err, "connect redis")

	// ── Kafka ────────────────────────────────────────────────────────────────
	kafkaProducer, err := kafka.NewProducer(cfg.Kafka.Brokers)
	must(err, "create kafka producer")
	defer kafkaProducer.Close()

	kafkaConsumer, err := kafka.NewConsumer(cfg.Kafka.Brokers, cfg.Kafka.GroupID)
	must(err, "create kafka consumer")
	defer kafkaConsumer.Close()

	// ── Repositories ─────────────────────────────────────────────────────────
	syncRepo := repository.NewPostgresSyncRepo(pool)
	labRepo := repository.NewPostgresLabResultRepo(pool)

	// ── Adapters ─────────────────────────────────────────────────────────────
	egovClient := egov.New(cfg.Egov.APIURL, cfg.Egov.APIKey)
	_ = damumed.New(cfg.Damumed.APIURL, cfg.Damumed.APIKey) // available for future use
	idoctorClient := idoctor.New(cfg.IDoctor.APIURL, cfg.IDoctor.WebhookSecret)
	olympClient := olymp.New(cfg.OlympLab.APIURL, cfg.OlympLab.APIKey)
	invivoClient := invivo.New(cfg.InvivoLab.APIURL, cfg.InvivoLab.APIKey)

	// ── Service ───────────────────────────────────────────────────────────────
	svc := service.New(service.Deps{
		SyncRepo:    syncRepo,
		LabRepo:     labRepo,
		EgovAdapter: egovClient,
		IDoctorAdap: idoctorClient,
		OlympAdap:   olympClient,
		InvivoAdap:  invivoClient,
		Redis:       rdb,
		Kafka:       kafkaProducer,
		Log:         log,
	})

	// ── HTTP Server ───────────────────────────────────────────────────────────
	httpHandler := handler.NewHTTP(svc, log)
	webhookCfg := handler.WebhookConfig{
		IDoctorSecret: cfg.Webhook.IDoctorSecret,
		OlympSecret:   cfg.Webhook.OlympSecret,
		InvivoSecret:  cfg.Webhook.InvivoSecret,
	}
	webhookHandler := handler.NewWebhook(svc, webhookCfg, log)

	// Объединяем API и webhook обработчики
	mux := http.NewServeMux()
	mux.Handle("/", httpHandler)
	mux.Handle("/webhooks/", webhookHandler)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.HTTPPort),
		Handler:      mux,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// ── Workers ───────────────────────────────────────────────────────────────
	syncWorker := worker.NewSyncWorker(svc, syncRepo, cfg.Sync.Interval, log)
	kafkaWorker := worker.NewKafkaConsumer(kafkaConsumer, svc, log)

	// ── Start ─────────────────────────────────────────────────────────────────
	go func() {
		ln, err := net.Listen("tcp", httpServer.Addr)
		must(err, "listen http")
		log.Info().Str("addr", httpServer.Addr).Msg("http server started")
		if err := httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("http server error")
		}
	}()

	go syncWorker.Run(ctx)
	go kafkaWorker.Run(ctx)

	log.Info().Msg("integration-service started")
	<-ctx.Done()

	log.Info().Msg("shutting down...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("http server shutdown error")
	}

	log.Info().Msg("integration-service stopped")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func must(err error, msg string) {
	if err != nil {
		zerolog.Ctx(context.Background()).Fatal().Err(err).Msg(msg)
		os.Exit(1)
	}
}
