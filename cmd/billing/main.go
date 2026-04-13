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
	"google.golang.org/grpc"

	billingcfg "github.com/nurtidev/medcore/internal/billing/config"
	"github.com/nurtidev/medcore/internal/billing/domain"
	"github.com/nurtidev/medcore/internal/billing/handler"
	billingprov "github.com/nurtidev/medcore/internal/billing/provider"
	"github.com/nurtidev/medcore/internal/billing/provider/gotenberg"
	"github.com/nurtidev/medcore/internal/billing/provider/kaspi"
	"github.com/nurtidev/medcore/internal/billing/provider/stripe"
	"github.com/nurtidev/medcore/internal/billing/repository"
	"github.com/nurtidev/medcore/internal/billing/service"
	"github.com/nurtidev/medcore/internal/shared/database"
	"github.com/nurtidev/medcore/internal/shared/kafka"
	"github.com/nurtidev/medcore/internal/shared/logger"
)

func main() {
	ctx := context.Background()

	// ── Config ──────────────────────────────────────────────────────────────
	cfgPath := envOr("CONFIG_PATH", "configs/billing.yaml")
	cfg, err := billingcfg.Load(cfgPath)
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

	// ── Kafka Producer ───────────────────────────────────────────────────────
	kafkaProducer, err := kafka.NewProducer(cfg.Kafka.Brokers)
	must(err, "create kafka producer")
	defer kafkaProducer.Close()

	// ── Payment Providers ────────────────────────────────────────────────────
	kaspiClient := kaspi.New(cfg.Kaspi.APIURL, cfg.Kaspi.MerchantID, cfg.Kaspi.SecretKey)
	stripeClient := stripe.New(cfg.Stripe.SecretKey, cfg.Stripe.WebhookSecret)

	providers := map[domain.PaymentProvider]billingprov.PaymentProvider{
		domain.ProviderKaspi:  kaspiClient,
		domain.ProviderStripe: stripeClient,
	}

	// ── Repositories ─────────────────────────────────────────────────────────
	paymentRepo := repository.NewPostgresPaymentRepo(pool)
	invoiceRepo := repository.NewPostgresInvoiceRepo(pool)
	subRepo := repository.NewPostgresSubscriptionRepo(pool)

	// ── Gotenberg (PDF) ───────────────────────────────────────────────────────
	gotenbergURL := envOr("GOTENBERG_URL", "http://gotenberg:3000")
	gotenbergClient := gotenberg.New(gotenbergURL)

	// ── Service ───────────────────────────────────────────────────────────────
	svc := service.New(
		paymentRepo,
		invoiceRepo,
		subRepo,
		providers,
		kafkaProducer,
		service.ServiceConfig{
			PaymentCompletedTopic:    cfg.Kafka.Topics["payment_completed"],
			SubscriptionExpiredTopic: cfg.Kafka.Topics["subscription_expired"],
		},
		gotenbergClient,
	)

	// ── CRON: expired subscriptions (every 5 min) ────────────────────────────
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := svc.ProcessExpiredSubscriptions(ctx); err != nil {
					log.Error().Err(err).Msg("cron: process expired subscriptions")
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// ── CRON: overdue invoices (daily at 00:00) ───────────────────────────────
	go func() {
		for {
			now := time.Now()
			next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
			timer := time.NewTimer(next.Sub(now))
			select {
			case <-timer.C:
				if err := svc.MarkOverdueInvoices(ctx); err != nil {
					log.Error().Err(err).Msg("cron: mark overdue invoices")
				}
			case <-ctx.Done():
				timer.Stop()
				return
			}
		}
	}()

	// ── HTTP server ───────────────────────────────────────────────────────────
	httpHandler := handler.NewHTTPHandler(svc, cfg.JWT.Secret)
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.HTTPPort),
		Handler:      httpHandler.Router(),
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// ── gRPC server ───────────────────────────────────────────────────────────
	grpcServer := grpc.NewServer()
	handler.NewGRPC(svc).Register(grpcServer)

	grpcLis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Server.GRPCPort))
	must(err, "listen grpc")

	// ── Start ─────────────────────────────────────────────────────────────────
	go func() {
		log.Info().Int("port", cfg.Server.HTTPPort).Msg("billing HTTP server starting")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("billing HTTP server failed")
		}
	}()

	go func() {
		log.Info().Int("port", cfg.Server.GRPCPort).Msg("billing gRPC server starting")
		if err := grpcServer.Serve(grpcLis); err != nil {
			log.Fatal().Err(err).Msg("billing gRPC server failed")
		}
	}()

	// ── Graceful shutdown ─────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("billing service shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("HTTP shutdown error")
	}
	grpcServer.GracefulStop()

	log.Info().Msg("billing service shutdown complete")
}

func must(err error, msg string) {
	if err != nil {
		l := zerolog.New(os.Stderr)
		l.Fatal().Err(err).Msg(msg)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
