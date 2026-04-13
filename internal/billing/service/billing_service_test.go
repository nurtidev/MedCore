package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nurtidev/medcore/internal/billing/domain"
	"github.com/nurtidev/medcore/internal/billing/provider"
	"github.com/nurtidev/medcore/internal/billing/repository"
	"github.com/nurtidev/medcore/internal/billing/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockPDFRenderer struct {
	mock.Mock
}

func (m *mockPDFRenderer) HTMLtoPDF(ctx context.Context, html string) ([]byte, error) {
	args := m.Called(ctx, html)
	if v, ok := args.Get(0).([]byte); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}

// ── Mock: PaymentRepository ────────────────────────────────────────────────

type mockPaymentRepo struct{ mock.Mock }

func (m *mockPaymentRepo) Create(ctx context.Context, p *domain.Payment) error {
	return m.Called(ctx, p).Error(0)
}
func (m *mockPaymentRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Payment, error) {
	args := m.Called(ctx, id)
	if p, ok := args.Get(0).(*domain.Payment); ok {
		return p, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockPaymentRepo) GetByIdempotencyKey(ctx context.Context, key string) (*domain.Payment, error) {
	args := m.Called(ctx, key)
	if p, ok := args.Get(0).(*domain.Payment); ok {
		return p, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockPaymentRepo) GetByExternalID(ctx context.Context, externalID string) (*domain.Payment, error) {
	args := m.Called(ctx, externalID)
	if p, ok := args.Get(0).(*domain.Payment); ok {
		return p, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockPaymentRepo) Update(ctx context.Context, p *domain.Payment) error {
	return m.Called(ctx, p).Error(0)
}

// ── Mock: InvoiceRepository ────────────────────────────────────────────────

type mockInvoiceRepo struct{ mock.Mock }

func (m *mockInvoiceRepo) Create(ctx context.Context, inv *domain.Invoice) error {
	return m.Called(ctx, inv).Error(0)
}
func (m *mockInvoiceRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Invoice, error) {
	args := m.Called(ctx, id)
	if inv, ok := args.Get(0).(*domain.Invoice); ok {
		return inv, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockInvoiceRepo) List(ctx context.Context, clinicID uuid.UUID, filter repository.InvoiceFilter) ([]*domain.Invoice, error) {
	args := m.Called(ctx, clinicID, filter)
	if v, ok := args.Get(0).([]*domain.Invoice); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockInvoiceRepo) Update(ctx context.Context, inv *domain.Invoice) error {
	return m.Called(ctx, inv).Error(0)
}
func (m *mockInvoiceRepo) MarkOverdue(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

// ── Mock: SubscriptionRepository ──────────────────────────────────────────

type mockSubRepo struct{ mock.Mock }

func (m *mockSubRepo) Create(ctx context.Context, sub *domain.Subscription) error {
	return m.Called(ctx, sub).Error(0)
}
func (m *mockSubRepo) GetByClinicID(ctx context.Context, clinicID uuid.UUID) (*domain.Subscription, error) {
	args := m.Called(ctx, clinicID)
	if s, ok := args.Get(0).(*domain.Subscription); ok {
		return s, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockSubRepo) GetExpired(ctx context.Context) ([]*domain.Subscription, error) {
	args := m.Called(ctx)
	if v, ok := args.Get(0).([]*domain.Subscription); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockSubRepo) Update(ctx context.Context, sub *domain.Subscription) error {
	return m.Called(ctx, sub).Error(0)
}
func (m *mockSubRepo) GetPlanByID(ctx context.Context, planID uuid.UUID) (*domain.Plan, error) {
	args := m.Called(ctx, planID)
	if p, ok := args.Get(0).(*domain.Plan); ok {
		return p, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockSubRepo) ListPlans(ctx context.Context) ([]*domain.Plan, error) {
	args := m.Called(ctx)
	if v, ok := args.Get(0).([]*domain.Plan); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}

// ── Mock: PaymentProvider ──────────────────────────────────────────────────

type mockProvider struct{ mock.Mock }

func (m *mockProvider) Name() string { return "mock" }
func (m *mockProvider) CreatePaymentLink(ctx context.Context, req provider.PaymentLinkRequest) (string, error) {
	args := m.Called(ctx, req)
	return args.String(0), args.Error(1)
}
func (m *mockProvider) VerifyWebhookSignature(payload []byte, signature string) bool {
	return m.Called(payload, signature).Bool(0)
}
func (m *mockProvider) ParseWebhookEvent(payload []byte) (*provider.WebhookEvent, error) {
	args := m.Called(payload)
	if e, ok := args.Get(0).(*provider.WebhookEvent); ok {
		return e, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockProvider) RefundPayment(ctx context.Context, externalID string, amount decimal.Decimal) error {
	return m.Called(ctx, externalID, amount).Error(0)
}

// ── Helpers ────────────────────────────────────────────────────────────────

func buildService(
	payRepo *mockPaymentRepo,
	invRepo *mockInvoiceRepo,
	subRepo *mockSubRepo,
	prov provider.PaymentProvider,
) service.BillingService {
	providers := map[domain.PaymentProvider]provider.PaymentProvider{}
	if prov != nil {
		providers[domain.ProviderKaspi] = prov
	}
	return service.New(payRepo, invRepo, subRepo, providers, nil, service.ServiceConfig{
		PaymentCompletedTopic:    "payment.completed",
		SubscriptionExpiredTopic: "subscription.expired",
	}, nil)
}

func buildServiceWithPDFRenderer(
	payRepo *mockPaymentRepo,
	invRepo *mockInvoiceRepo,
	subRepo *mockSubRepo,
	prov provider.PaymentProvider,
	renderer *mockPDFRenderer,
) service.BillingService {
	providers := map[domain.PaymentProvider]provider.PaymentProvider{}
	if prov != nil {
		providers[domain.ProviderKaspi] = prov
	}
	return service.New(payRepo, invRepo, subRepo, providers, nil, service.ServiceConfig{
		PaymentCompletedTopic:    "payment.completed",
		SubscriptionExpiredTopic: "subscription.expired",
	}, renderer)
}

// ── Tests ──────────────────────────────────────────────────────────────────

func TestCreatePaymentLink_Success(t *testing.T) {
	ctx := context.Background()
	payRepo := new(mockPaymentRepo)
	invRepo := new(mockInvoiceRepo)
	subRepo := new(mockSubRepo)
	prov := new(mockProvider)

	// No existing payment for this idempotency key
	payRepo.On("GetByIdempotencyKey", ctx, "idem-1").Return(nil, domain.ErrPaymentNotFound)
	prov.On("CreatePaymentLink", ctx, mock.Anything).Return("https://pay.kaspi.kz/abc123", nil)
	payRepo.On("Create", ctx, mock.Anything).Return(nil)

	svc := buildService(payRepo, invRepo, subRepo, prov)

	link, err := svc.CreatePaymentLink(ctx, service.CreatePaymentRequest{
		InvoiceID:      uuid.New(),
		ClinicID:       uuid.New(),
		PatientID:      uuid.New(),
		Amount:         decimal.NewFromFloat(5000),
		Currency:       "KZT",
		Provider:       domain.ProviderKaspi,
		IdempotencyKey: "idem-1",
		ReturnURL:      "https://app.example.com/return",
		Description:    "Консультация",
	})

	require.NoError(t, err)
	assert.Equal(t, "https://pay.kaspi.kz/abc123", link.URL)
	assert.NotEqual(t, uuid.Nil, link.PaymentID)
	payRepo.AssertExpectations(t)
	prov.AssertExpectations(t)
}

func TestCreatePaymentLink_IdempotencyKey_ReturnsSamePayment(t *testing.T) {
	ctx := context.Background()
	payRepo := new(mockPaymentRepo)
	invRepo := new(mockInvoiceRepo)
	subRepo := new(mockSubRepo)

	existingPayment := &domain.Payment{
		ID:             uuid.New(),
		IdempotencyKey: "idem-dup",
		Status:         domain.PaymentStatusPending,
		CreatedAt:      time.Now(),
	}
	payRepo.On("GetByIdempotencyKey", ctx, "idem-dup").Return(existingPayment, nil)

	svc := buildService(payRepo, invRepo, subRepo, nil)

	link, err := svc.CreatePaymentLink(ctx, service.CreatePaymentRequest{
		InvoiceID:      uuid.New(),
		ClinicID:       uuid.New(),
		Amount:         decimal.NewFromFloat(5000),
		Currency:       "KZT",
		Provider:       domain.ProviderKaspi,
		IdempotencyKey: "idem-dup",
	})

	require.NoError(t, err)
	assert.Equal(t, existingPayment.ID, link.PaymentID)
	// Provider must NOT be called for duplicate requests
	payRepo.AssertNumberOfCalls(t, "Create", 0)
}

func TestProcessWebhook_Kaspi_Success(t *testing.T) {
	ctx := context.Background()
	payRepo := new(mockPaymentRepo)
	invRepo := new(mockInvoiceRepo)
	subRepo := new(mockSubRepo)
	prov := new(mockProvider)

	paymentID := uuid.New()
	invoiceID := uuid.New()
	payload := []byte(`{"transactionId":"tx-ext-1","status":"SUCCESS","amount":"5000","currency":"KZT"}`)

	existingPayment := &domain.Payment{
		ID:        paymentID,
		InvoiceID: invoiceID,
		Status:    domain.PaymentStatusPending,
	}
	existingInvoice := &domain.Invoice{
		ID:     invoiceID,
		Status: domain.InvoiceStatusSent,
	}

	prov.On("VerifyWebhookSignature", payload, "valid-sig").Return(true)
	prov.On("ParseWebhookEvent", payload).Return(&provider.WebhookEvent{
		Type:       "payment.completed",
		ExternalID: "tx-ext-1",
		Status:     "completed",
	}, nil)
	payRepo.On("GetByExternalID", ctx, "tx-ext-1").Return(existingPayment, nil)
	payRepo.On("Update", ctx, mock.MatchedBy(func(p *domain.Payment) bool {
		return p.Status == domain.PaymentStatusCompleted
	})).Return(nil)
	invRepo.On("GetByID", ctx, invoiceID).Return(existingInvoice, nil)
	invRepo.On("Update", ctx, mock.MatchedBy(func(inv *domain.Invoice) bool {
		return inv.Status == domain.InvoiceStatusPaid
	})).Return(nil)

	svc := buildService(payRepo, invRepo, subRepo, prov)

	err := svc.ProcessWebhook(ctx, domain.ProviderKaspi, payload, "valid-sig")
	require.NoError(t, err)
	payRepo.AssertExpectations(t)
	invRepo.AssertExpectations(t)
}

func TestGenerateInvoicePDF_Success(t *testing.T) {
	ctx := context.Background()
	payRepo := new(mockPaymentRepo)
	invRepo := new(mockInvoiceRepo)
	subRepo := new(mockSubRepo)
	renderer := new(mockPDFRenderer)

	invoiceID := uuid.New()
	invoice := &domain.Invoice{
		ID:          invoiceID,
		ClinicID:    uuid.New(),
		PatientID:   uuid.New(),
		ServiceName: "Первичная консультация",
		Amount:      decimal.NewFromInt(49900),
		Currency:    "KZT",
		Status:      domain.InvoiceStatusSent,
		DueAt:       time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC),
		CreatedAt:   time.Now(),
	}
	expectedPDF := []byte("%PDF-1.7 mock")

	invRepo.On("GetByID", ctx, invoiceID).Return(invoice, nil)
	renderer.On("HTMLtoPDF", ctx, mock.MatchedBy(func(html string) bool {
		return strings.Contains(html, invoice.ServiceName) &&
			strings.Contains(html, invoice.Amount.String()) &&
			strings.Contains(html, invoice.Currency)
	})).Return(expectedPDF, nil)

	svc := buildServiceWithPDFRenderer(payRepo, invRepo, subRepo, nil, renderer)

	pdf, err := svc.GenerateInvoicePDF(ctx, invoiceID)
	require.NoError(t, err)
	assert.Equal(t, expectedPDF, pdf)
	invRepo.AssertExpectations(t)
	renderer.AssertExpectations(t)
}

func TestGenerateInvoicePDF_ReturnsErrorWhenRendererUnavailable(t *testing.T) {
	ctx := context.Background()
	payRepo := new(mockPaymentRepo)
	invRepo := new(mockInvoiceRepo)
	subRepo := new(mockSubRepo)

	invoiceID := uuid.New()
	invoice := &domain.Invoice{
		ID:          invoiceID,
		ClinicID:    uuid.New(),
		ServiceName: "УЗИ",
		Amount:      decimal.NewFromInt(15000),
		Currency:    "KZT",
		Status:      domain.InvoiceStatusSent,
		DueAt:       time.Now().Add(24 * time.Hour),
	}

	invRepo.On("GetByID", ctx, invoiceID).Return(invoice, nil)

	svc := buildService(payRepo, invRepo, subRepo, nil)

	_, err := svc.GenerateInvoicePDF(ctx, invoiceID)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrPDFUnavailable)
	invRepo.AssertExpectations(t)
}

func TestProcessWebhook_InvalidSignature_Returns401(t *testing.T) {
	ctx := context.Background()
	payRepo := new(mockPaymentRepo)
	invRepo := new(mockInvoiceRepo)
	subRepo := new(mockSubRepo)
	prov := new(mockProvider)

	payload := []byte(`{"transactionId":"x"}`)
	prov.On("VerifyWebhookSignature", payload, "bad-sig").Return(false)

	svc := buildService(payRepo, invRepo, subRepo, prov)

	err := svc.ProcessWebhook(ctx, domain.ProviderKaspi, payload, "bad-sig")
	assert.True(t, errors.Is(err, domain.ErrInvalidSignature))
}

func TestCreateSubscription_Success(t *testing.T) {
	ctx := context.Background()
	payRepo := new(mockPaymentRepo)
	invRepo := new(mockInvoiceRepo)
	subRepo := new(mockSubRepo)

	clinicID := uuid.New()
	planID := uuid.New()

	subRepo.On("GetPlanByID", ctx, planID).Return(&domain.Plan{ID: planID}, nil)
	subRepo.On("GetByClinicID", ctx, clinicID).Return(nil, domain.ErrSubscriptionNotFound)
	subRepo.On("Create", ctx, mock.Anything).Return(nil)

	svc := buildService(payRepo, invRepo, subRepo, nil)

	sub, err := svc.CreateSubscription(ctx, clinicID, planID)
	require.NoError(t, err)
	assert.Equal(t, clinicID, sub.ClinicID)
	assert.Equal(t, planID, sub.PlanID)
	assert.Equal(t, domain.SubStatusActive, sub.Status)
	assert.True(t, sub.CurrentPeriodEnd.After(time.Now()))
}

func TestCheckSubscriptionAccess_Expired_ReturnsFalse(t *testing.T) {
	ctx := context.Background()
	payRepo := new(mockPaymentRepo)
	invRepo := new(mockInvoiceRepo)
	subRepo := new(mockSubRepo)

	clinicID := uuid.New()
	expiredSub := &domain.Subscription{
		ID:               uuid.New(),
		ClinicID:         clinicID,
		Status:           domain.SubStatusActive,
		CurrentPeriodEnd: time.Now().Add(-24 * time.Hour), // already expired
	}
	subRepo.On("GetByClinicID", ctx, clinicID).Return(expiredSub, nil)

	svc := buildService(payRepo, invRepo, subRepo, nil)

	active, err := svc.CheckSubscriptionAccess(ctx, clinicID)
	require.NoError(t, err)
	assert.False(t, active)
}
