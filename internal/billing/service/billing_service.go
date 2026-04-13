package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nurtidev/medcore/internal/billing/domain"
	"github.com/nurtidev/medcore/internal/billing/provider"
	"github.com/nurtidev/medcore/internal/billing/repository"
	"github.com/nurtidev/medcore/internal/shared/kafka"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/shopspring/decimal"
)

// ── Request / Response types ───────────────────────────────────────────────

type CreatePaymentRequest struct {
	InvoiceID      uuid.UUID
	ClinicID       uuid.UUID
	PatientID      uuid.UUID
	Amount         decimal.Decimal
	Currency       string
	Provider       domain.PaymentProvider
	IdempotencyKey string
	ReturnURL      string
	Description    string
}

type PaymentLink struct {
	PaymentID uuid.UUID `json:"payment_id"`
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}

type CreateInvoiceRequest struct {
	ClinicID    uuid.UUID
	PatientID   *uuid.UUID
	ServiceName string
	Amount      decimal.Decimal
	Currency    string
	DueAt       time.Time
}

// ── Kafka event structs ────────────────────────────────────────────────────

type PaymentCompletedEvent struct {
	PaymentID  string          `json:"payment_id"`
	InvoiceID  string          `json:"invoice_id"`
	ClinicID   string          `json:"clinic_id"`
	PatientID  string          `json:"patient_id"`
	Amount     decimal.Decimal `json:"amount"`
	Currency   string          `json:"currency"`
	Provider   string          `json:"provider"`
	OccurredAt time.Time       `json:"occurred_at"`
}

type SubscriptionExpiredEvent struct {
	SubscriptionID string    `json:"subscription_id"`
	ClinicID       string    `json:"clinic_id"`
	ExpiredAt      time.Time `json:"expired_at"`
}

// ── Service interface ──────────────────────────────────────────────────────

type BillingService interface {
	// Payments
	CreatePaymentLink(ctx context.Context, req CreatePaymentRequest) (*PaymentLink, error)
	GetPayment(ctx context.Context, paymentID uuid.UUID) (*domain.Payment, error)
	ProcessWebhook(ctx context.Context, prov domain.PaymentProvider, payload []byte, signature string) error

	// Invoices
	CreateInvoice(ctx context.Context, req CreateInvoiceRequest) (*domain.Invoice, error)
	GetInvoice(ctx context.Context, invoiceID uuid.UUID) (*domain.Invoice, error)
	ListInvoices(ctx context.Context, clinicID uuid.UUID, filter repository.InvoiceFilter) ([]*domain.Invoice, error)
	GenerateInvoicePDF(ctx context.Context, invoiceID uuid.UUID) ([]byte, error)

	// Subscriptions
	GetSubscription(ctx context.Context, clinicID uuid.UUID) (*domain.Subscription, error)
	CreateSubscription(ctx context.Context, clinicID uuid.UUID, planID uuid.UUID) (*domain.Subscription, error)
	CancelSubscription(ctx context.Context, clinicID uuid.UUID) error
	CheckSubscriptionAccess(ctx context.Context, clinicID uuid.UUID) (bool, error)

	// Plans
	ListPlans(ctx context.Context) ([]*domain.Plan, error)

	// Background jobs
	ProcessExpiredSubscriptions(ctx context.Context) error
	MarkOverdueInvoices(ctx context.Context) error
}

// ── Prometheus metrics ─────────────────────────────────────────────────────

type billingMetrics struct {
	paymentRequestsTotal *prometheus.CounterVec
	paymentAmountTotal   *prometheus.CounterVec
	paymentDuration      *prometheus.HistogramVec
	subscriptionActive   prometheus.Gauge
}

func newBillingMetrics() *billingMetrics {
	return &billingMetrics{
		paymentRequestsTotal: mustRegisterCounterVec(prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "payment_requests_total",
			Help: "Total payment requests by provider and status",
		}, []string{"provider", "status"})),

		paymentAmountTotal: mustRegisterCounterVec(prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "payment_amount_total",
			Help: "Total payment amount by provider and currency",
		}, []string{"provider", "currency"})),

		paymentDuration: mustRegisterHistogramVec(prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "payment_duration_seconds",
			Help:    "Payment link creation duration in seconds",
			Buckets: prometheus.DefBuckets,
		}, []string{"provider"})),

		subscriptionActive: mustRegisterGauge(prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "subscription_active_total",
			Help: "Number of currently active clinic subscriptions",
		})),
	}
}

// mustRegisterCounterVec registers a CounterVec, returning the existing one if already registered.
func mustRegisterCounterVec(c *prometheus.CounterVec) *prometheus.CounterVec {
	if err := prometheus.DefaultRegisterer.Register(c); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			return are.ExistingCollector.(*prometheus.CounterVec)
		}
		panic(err)
	}
	return c
}

// mustRegisterHistogramVec registers a HistogramVec, returning the existing one if already registered.
func mustRegisterHistogramVec(h *prometheus.HistogramVec) *prometheus.HistogramVec {
	if err := prometheus.DefaultRegisterer.Register(h); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			return are.ExistingCollector.(*prometheus.HistogramVec)
		}
		panic(err)
	}
	return h
}

// mustRegisterGauge registers a Gauge, returning the existing one if already registered.
func mustRegisterGauge(g prometheus.Gauge) prometheus.Gauge {
	if err := prometheus.DefaultRegisterer.Register(g); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			return are.ExistingCollector.(prometheus.Gauge)
		}
		panic(err)
	}
	return g
}

// ── Service config ─────────────────────────────────────────────────────────

type ServiceConfig struct {
	PaymentCompletedTopic    string
	SubscriptionExpiredTopic string
}

// ── Implementation ─────────────────────────────────────────────────────────

type billingServiceImpl struct {
	paymentRepo repository.PaymentRepository
	invoiceRepo repository.InvoiceRepository
	subRepo     repository.SubscriptionRepository
	providers   map[domain.PaymentProvider]provider.PaymentProvider
	kafka       *kafka.Producer
	cfg         ServiceConfig
	metrics     *billingMetrics
	pdfRenderer pdfRenderer
}

type pdfRenderer interface {
	HTMLtoPDF(ctx context.Context, html string) ([]byte, error)
}

func New(
	paymentRepo repository.PaymentRepository,
	invoiceRepo repository.InvoiceRepository,
	subRepo repository.SubscriptionRepository,
	providers map[domain.PaymentProvider]provider.PaymentProvider,
	kafka *kafka.Producer,
	cfg ServiceConfig,
	pdfRenderer pdfRenderer,
) BillingService {
	return &billingServiceImpl{
		paymentRepo: paymentRepo,
		invoiceRepo: invoiceRepo,
		subRepo:     subRepo,
		providers:   providers,
		kafka:       kafka,
		cfg:         cfg,
		metrics:     newBillingMetrics(),
		pdfRenderer: pdfRenderer,
	}
}

// ── Payment methods ────────────────────────────────────────────────────────

func (s *billingServiceImpl) CreatePaymentLink(ctx context.Context, req CreatePaymentRequest) (*PaymentLink, error) {
	// Idempotency: return existing payment if key already seen
	existing, err := s.paymentRepo.GetByIdempotencyKey(ctx, req.IdempotencyKey)
	if err == nil && existing != nil {
		return &PaymentLink{
			PaymentID: existing.ID,
			URL:       "",
			ExpiresAt: existing.CreatedAt.Add(24 * time.Hour),
		}, nil
	}

	p, ok := s.providers[req.Provider]
	if !ok {
		return nil, domain.ErrUnknownProvider
	}

	payment := &domain.Payment{
		ID:             uuid.New(),
		InvoiceID:      req.InvoiceID,
		ClinicID:       req.ClinicID,
		PatientID:      req.PatientID,
		IdempotencyKey: req.IdempotencyKey,
		Provider:       req.Provider,
		Amount:         req.Amount,
		Currency:       req.Currency,
		Status:         domain.PaymentStatusPending,
	}

	start := time.Now()
	payURL, err := p.CreatePaymentLink(ctx, provider.PaymentLinkRequest{
		PaymentID:      payment.ID,
		InvoiceID:      req.InvoiceID,
		Amount:         req.Amount,
		Currency:       req.Currency,
		IdempotencyKey: req.IdempotencyKey,
		ReturnURL:      req.ReturnURL,
		Description:    req.Description,
	})
	s.metrics.paymentDuration.WithLabelValues(string(req.Provider)).Observe(time.Since(start).Seconds())

	if err != nil {
		s.metrics.paymentRequestsTotal.WithLabelValues(string(req.Provider), "error").Inc()
		return nil, fmt.Errorf("billing.CreatePaymentLink: provider: %w", err)
	}

	if err := s.paymentRepo.Create(ctx, payment); err != nil {
		return nil, fmt.Errorf("billing.CreatePaymentLink: save payment: %w", err)
	}

	s.metrics.paymentRequestsTotal.WithLabelValues(string(req.Provider), "created").Inc()
	s.metrics.paymentAmountTotal.WithLabelValues(string(req.Provider), req.Currency).
		Add(req.Amount.InexactFloat64())

	return &PaymentLink{
		PaymentID: payment.ID,
		URL:       payURL,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}, nil
}

func (s *billingServiceImpl) GetPayment(ctx context.Context, paymentID uuid.UUID) (*domain.Payment, error) {
	p, err := s.paymentRepo.GetByID(ctx, paymentID)
	if err != nil {
		return nil, fmt.Errorf("billing.GetPayment: %w", err)
	}
	return p, nil
}

func (s *billingServiceImpl) ProcessWebhook(
	ctx context.Context,
	prov domain.PaymentProvider,
	payload []byte,
	signature string,
) error {
	p, ok := s.providers[prov]
	if !ok {
		return domain.ErrUnknownProvider
	}

	if !p.VerifyWebhookSignature(payload, signature) {
		return domain.ErrInvalidSignature
	}

	event, err := p.ParseWebhookEvent(payload)
	if err != nil {
		return fmt.Errorf("billing.ProcessWebhook: parse: %w", err)
	}

	payment, err := s.paymentRepo.GetByExternalID(ctx, event.ExternalID)
	if err != nil {
		return fmt.Errorf("billing.ProcessWebhook: find payment: %w", err)
	}

	switch event.Status {
	case "completed":
		payment.Status = domain.PaymentStatusCompleted
		if err := s.paymentRepo.Update(ctx, payment); err != nil {
			return fmt.Errorf("billing.ProcessWebhook: update payment: %w", err)
		}

		invoice, err := s.invoiceRepo.GetByID(ctx, payment.InvoiceID)
		if err != nil {
			return fmt.Errorf("billing.ProcessWebhook: get invoice: %w", err)
		}
		now := time.Now()
		invoice.Status = domain.InvoiceStatusPaid
		invoice.PaidAt = &now
		if err := s.invoiceRepo.Update(ctx, invoice); err != nil {
			return fmt.Errorf("billing.ProcessWebhook: update invoice: %w", err)
		}

		s.metrics.paymentRequestsTotal.WithLabelValues(string(prov), "completed").Inc()
		s.publishPaymentCompleted(ctx, payment)

	case "failed":
		payment.Status = domain.PaymentStatusFailed
		if r, ok := event.RawPayload["failure_reason"]; ok {
			if reason, ok := r.(string); ok {
				payment.FailureReason = reason
			}
		}
		if err := s.paymentRepo.Update(ctx, payment); err != nil {
			return fmt.Errorf("billing.ProcessWebhook: update failed payment: %w", err)
		}
		s.metrics.paymentRequestsTotal.WithLabelValues(string(prov), "failed").Inc()
	}

	return nil
}

func (s *billingServiceImpl) publishPaymentCompleted(ctx context.Context, p *domain.Payment) {
	if s.kafka == nil {
		return
	}
	evt := PaymentCompletedEvent{
		PaymentID:  p.ID.String(),
		InvoiceID:  p.InvoiceID.String(),
		ClinicID:   p.ClinicID.String(),
		PatientID:  p.PatientID.String(),
		Amount:     p.Amount,
		Currency:   p.Currency,
		Provider:   string(p.Provider),
		OccurredAt: time.Now(),
	}
	// Best-effort: do not fail the request if Kafka is unavailable
	_ = s.kafka.Publish(ctx, s.cfg.PaymentCompletedTopic, p.ID.String(), evt)
}

// GenerateInvoicePDF renders the invoice as a PDF via the Gotenberg service.
func (s *billingServiceImpl) GenerateInvoicePDF(ctx context.Context, invoiceID uuid.UUID) ([]byte, error) {
	inv, err := s.invoiceRepo.GetByID(ctx, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("billing.GenerateInvoicePDF: %w", err)
	}
	if s.pdfRenderer == nil {
		return nil, fmt.Errorf("billing.GenerateInvoicePDF: %w", domain.ErrPDFUnavailable)
	}

	paidAt := "—"
	if inv.PaidAt != nil {
		paidAt = inv.PaidAt.Format("02.01.2006 15:04")
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="ru">
<head>
<meta charset="UTF-8">
<style>
  body { font-family: Arial, sans-serif; margin: 40px; color: #222; }
  h1   { color: #1a56db; margin-bottom: 4px; }
  .sub { color: #6b7280; font-size: 13px; margin-bottom: 32px; }
  table { width: 100%%; border-collapse: collapse; margin-top: 24px; }
  th { text-align: left; background: #f3f4f6; padding: 10px 14px; font-size: 12px; text-transform: uppercase; color: #6b7280; }
  td { padding: 10px 14px; border-bottom: 1px solid #e5e7eb; }
  .amount { font-size: 24px; font-weight: bold; color: #1a56db; margin-top: 32px; }
  .status-%s { display: inline-block; padding: 4px 12px; border-radius: 9999px; font-size: 12px;
               background: #d1fae5; color: #065f46; font-weight: 600; }
</style>
</head>
<body>
  <h1>MedCore</h1>
  <div class="sub">Счёт-фактура / Invoice</div>

  <table>
    <tr><th>Поле</th><th>Значение</th></tr>
    <tr><td>Номер счёта</td><td>%s</td></tr>
    <tr><td>Клиника</td><td>%s</td></tr>
    <tr><td>Пациент</td><td>%s</td></tr>
    <tr><td>Услуга</td><td>%s</td></tr>
    <tr><td>Статус</td><td><span class="status-%s">%s</span></td></tr>
    <tr><td>Срок оплаты</td><td>%s</td></tr>
    <tr><td>Оплачено</td><td>%s</td></tr>
  </table>

  <div class="amount">%s %s</div>
</body>
</html>`,
		string(inv.Status),
		inv.ID,
		inv.ClinicID,
		inv.PatientID,
		inv.ServiceName,
		string(inv.Status), string(inv.Status),
		inv.DueAt.Format("02.01.2006"),
		paidAt,
		inv.Amount.String(), inv.Currency,
	)

	pdf, err := s.pdfRenderer.HTMLtoPDF(ctx, html)
	if err != nil {
		return nil, fmt.Errorf("billing.GenerateInvoicePDF: gotenberg: %w", err)
	}
	return pdf, nil
}

// ListPlans delegates to the subscription repository.
func (s *billingServiceImpl) ListPlans(ctx context.Context) ([]*domain.Plan, error) {
	return s.subRepo.ListPlans(ctx)
}
