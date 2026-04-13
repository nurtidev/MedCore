package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"

	"github.com/nurtidev/medcore/internal/auth/handler"
	"github.com/nurtidev/medcore/internal/auth/repository"
	"github.com/nurtidev/medcore/internal/auth/service"
	"github.com/nurtidev/medcore/internal/shared/config"
	"github.com/nurtidev/medcore/internal/shared/database"
	"github.com/nurtidev/medcore/internal/shared/logger"
	authpb "github.com/nurtidev/medcore/pkg/proto/auth"
)

func main() {
	ctx := context.Background()

	// ── Config ──────────────────────────────────────────────────────────────
	cfgPath := envOr("CONFIG_PATH", "configs/auth.yaml")
	cfg, err := config.Load(cfgPath)
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
	defer rdb.Close()

	// ── IIN encryption key ──────────────────────────────────────────────────
	iinKeyHex := os.Getenv("IIN_ENCRYPTION_KEY")
	if iinKeyHex == "" {
		log.Fatal().Msg("IIN_ENCRYPTION_KEY env is required")
	}
	iinKey, err := hex.DecodeString(iinKeyHex)
	must(err, "decode IIN_ENCRYPTION_KEY")
	if len(iinKey) != 32 {
		log.Fatal().Msg("IIN_ENCRYPTION_KEY must be 32 bytes (64 hex chars)")
	}

	// ── Service ─────────────────────────────────────────────────────────────
	userRepo := repository.NewPostgresUserRepo(pool)
	tokenRepo := repository.NewPostgresTokenRepo(pool)

	svc := service.New(userRepo, tokenRepo, service.Config{
		JWTSecret:  []byte(cfg.JWT.Secret),
		AccessTTL:  cfg.JWT.AccessTTL,
		RefreshTTL: cfg.JWT.RefreshTTL,
		IINKey:     iinKey,
	})

	// ── HTTP server ──────────────────────────────────────────────────────────
	httpHandler := handler.NewHTTP(svc, rdb, log)
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.HTTPPort),
		Handler:      httpHandler,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// ── gRPC server ──────────────────────────────────────────────────────────
	grpcServer := grpc.NewServer()
	authpb.RegisterAuthServiceServer(grpcServer, handler.NewGRPC(svc))

	grpcLis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Server.GRPCPort))
	must(err, "listen grpc")

	// ── Start ────────────────────────────────────────────────────────────────
	go func() {
		log.Info().Int("port", cfg.Server.HTTPPort).Msg("HTTP server starting")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("HTTP server failed")
		}
	}()

	go func() {
		log.Info().Int("port", cfg.Server.GRPCPort).Msg("gRPC server starting")
		if err := grpcServer.Serve(grpcLis); err != nil {
			log.Fatal().Err(err).Msg("gRPC server failed")
		}
	}()

	// ── Graceful shutdown ────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("shutting down")

	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("HTTP shutdown error")
	}
	grpcServer.GracefulStop()

	log.Info().Msg("shutdown complete")
}

func must(err error, msg string) {
	if err != nil {
		l := zerolog.New(os.Stderr).With().Timestamp().Logger()
		l.Fatal().Err(err).Msg(msg)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
