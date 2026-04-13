package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	gatewaycfg "github.com/nurtidev/medcore/internal/gateway/config"
	"github.com/nurtidev/medcore/internal/gateway/handler"
	gatewaymw "github.com/nurtidev/medcore/internal/gateway/middleware"
	"github.com/nurtidev/medcore/internal/gateway/proxy"
	"github.com/nurtidev/medcore/internal/shared/database"
	"github.com/nurtidev/medcore/internal/shared/logger"
	authpb "github.com/nurtidev/medcore/pkg/proto/auth"
)

func main() {
	ctx := context.Background()

	// ── Config ──────────────────────────────────────────────────────────────────
	cfgPath := envOr("CONFIG_PATH", "configs/gateway.yaml")
	cfg, err := gatewaycfg.Load(cfgPath)
	must(err, "load config")

	// ── Logger ──────────────────────────────────────────────────────────────────
	log := logger.New(cfg.Log.Level, cfg.Log.Format)
	ctx = logger.WithContext(ctx, log)
	log.Info().Str("config", cfgPath).Msg("gateway starting")

	// ── Redis ───────────────────────────────────────────────────────────────────
	rdb, err := database.NewRedisClient(ctx, database.RedisConfig{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	must(err, "connect redis")
	defer rdb.Close()

	// ── gRPC connection to auth-service ─────────────────────────────────────────
	grpcConn, err := grpc.NewClient(
		cfg.AuthGRPC.Addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	must(err, "dial auth-service gRPC")
	defer grpcConn.Close()

	authClient := authpb.NewAuthServiceClient(grpcConn)

	// ── Router ──────────────────────────────────────────────────────────────────
	r := buildRouter(cfg, authClient, rdb, log)

	// ── HTTP server ──────────────────────────────────────────────────────────────
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.HTTPPort),
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	go func() {
		log.Info().Int("port", cfg.Server.HTTPPort).Msg("HTTP server starting")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("HTTP server failed")
		}
	}()

	// ── Graceful shutdown ────────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("shutting down gateway")

	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("HTTP shutdown error")
	}
	// grpcConn already closed by deferred call above

	log.Info().Msg("gateway stopped")
}

func buildRouter(
	cfg *gatewaycfg.Config,
	authClient authpb.AuthServiceClient,
	rdb *redis.Client,
	log zerolog.Logger,
) http.Handler {
	r := chi.NewRouter()

	// ── Global middleware ────────────────────────────────────────────────────────
	r.Use(gatewaymw.Logger(log))
	r.Use(gatewaymw.Tracing("medcore-gateway"))
	r.Use(gatewaymw.CORS(gatewaymw.CORSConfig{
		AllowedOrigins: cfg.CORS.AllowedOrigins,
		AllowedMethods: cfg.CORS.AllowedMethods,
		AllowedHeaders: cfg.CORS.AllowedHeaders,
	}))
	r.Use(gatewaymw.RateLimit(rdb, gatewaymw.RateLimitConfig{
		GlobalRPM:    cfg.RateLimit.GlobalRPM,
		LoginRPM:     cfg.RateLimit.LoginRPM,
		AnalyticsRPM: cfg.RateLimit.AnalyticsRPM,
	}))
	r.Use(gatewaymw.Auth(authClient, cfg.AuthGRPC.Timeout))

	// ── Gateway own endpoints ────────────────────────────────────────────────────
	r.Get("/health", handler.Health())
	r.Get("/ready", handler.Ready())
	r.Get("/api/v1/dashboard", handler.Dashboard(cfg.Upstream))

	// ── Auth service → :8081 ─────────────────────────────────────────────────────
	authProxy := proxy.NewAuthProxy(cfg.Upstream.Auth, cfg.Upstream.Timeouts.Default)
	r.Mount("/api/v1/auth", authProxy)
	r.Mount("/api/v1/users", authProxy)

	// ── Billing service → :8082 ──────────────────────────────────────────────────
	billingProxy := proxy.NewBillingProxy(cfg.Upstream.Billing, cfg.Upstream.Timeouts.Default)
	r.Mount("/api/v1/payments", billingProxy)
	r.Mount("/api/v1/invoices", billingProxy)
	r.Mount("/api/v1/subscriptions", billingProxy)
	r.Mount("/api/v1/plans", billingProxy)
	r.Mount("/webhooks/kaspi", billingProxy)
	r.Mount("/webhooks/stripe", billingProxy)

	// ── Integration service → :8083 ──────────────────────────────────────────────
	integrationProxy := proxy.NewIntegrationProxy(cfg.Upstream.Integration, cfg.Upstream.Timeouts.Default)
	r.Mount("/api/v1/gov", integrationProxy)
	r.Mount("/api/v1/sync", integrationProxy)
	r.Mount("/api/v1/lab-results", integrationProxy)
	r.Mount("/api/v1/integrations", integrationProxy)
	r.Mount("/webhooks/idoctor", integrationProxy)
	r.Mount("/webhooks/olymp", integrationProxy)
	r.Mount("/webhooks/invivo", integrationProxy)

	// ── Analytics service → :8084 ────────────────────────────────────────────────
	analyticsProxy := proxy.NewAnalyticsProxy(cfg.Upstream.Analytics, cfg.Upstream.Timeouts.Analytics)
	r.Mount("/api/v1/analytics", analyticsProxy)

	return r
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
