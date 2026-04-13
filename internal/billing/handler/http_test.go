package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/nurtidev/medcore/internal/billing/domain"
	"github.com/nurtidev/medcore/internal/billing/handler"
	"github.com/nurtidev/medcore/internal/billing/repository"
	"github.com/nurtidev/medcore/internal/billing/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const testJWTSecret = "test-secret"

// ── Mock: BillingService ───────────────────────────────────────────────────

type mockBillingService struct{ mock.Mock }

func (m *mockBillingService) CreatePaymentLink(ctx context.Context, req service.CreatePaymentRequest) (*service.PaymentLink, error) {
	args := m.Called(ctx, req)
	if v, ok := args.Get(0).(*service.PaymentLink); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockBillingService) GetPayment(ctx context.Context, id uuid.UUID) (*domain.Payment, error) {
	args := m.Called(ctx, id)
	if v, ok := args.Get(0).(*domain.Payment); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockBillingService) ProcessWebhook(ctx context.Context, prov domain.PaymentProvider, payload []byte, sig string) error {
	return m.Called(ctx, prov, payload, sig).Error(0)
}
func (m *mockBillingService) CreateInvoice(ctx context.Context, req service.CreateInvoiceRequest) (*domain.Invoice, error) {
	args := m.Called(ctx, req)
	if v, ok := args.Get(0).(*domain.Invoice); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockBillingService) GetInvoice(ctx context.Context, id uuid.UUID) (*domain.Invoice, error) {
	args := m.Called(ctx, id)
	if v, ok := args.Get(0).(*domain.Invoice); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockBillingService) ListInvoices(ctx context.Context, clinicID uuid.UUID, filter repository.InvoiceFilter) ([]*domain.Invoice, error) {
	args := m.Called(ctx, clinicID, filter)
	if v, ok := args.Get(0).([]*domain.Invoice); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockBillingService) GenerateInvoicePDF(ctx context.Context, id uuid.UUID) ([]byte, error) {
	args := m.Called(ctx, id)
	return args.Get(0).([]byte), args.Error(1)
}
func (m *mockBillingService) GetSubscription(ctx context.Context, clinicID uuid.UUID) (*domain.Subscription, error) {
	args := m.Called(ctx, clinicID)
	if v, ok := args.Get(0).(*domain.Subscription); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockBillingService) CreateSubscription(ctx context.Context, clinicID, planID uuid.UUID) (*domain.Subscription, error) {
	args := m.Called(ctx, clinicID, planID)
	if v, ok := args.Get(0).(*domain.Subscription); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockBillingService) CancelSubscription(ctx context.Context, clinicID uuid.UUID) error {
	return m.Called(ctx, clinicID).Error(0)
}
func (m *mockBillingService) CheckSubscriptionAccess(ctx context.Context, clinicID uuid.UUID) (bool, error) {
	args := m.Called(ctx, clinicID)
	return args.Bool(0), args.Error(1)
}
func (m *mockBillingService) ListPlans(ctx context.Context) ([]*domain.Plan, error) {
	args := m.Called(ctx)
	if v, ok := args.Get(0).([]*domain.Plan); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockBillingService) ProcessExpiredSubscriptions(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}
func (m *mockBillingService) MarkOverdueInvoices(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}

// Compile-time check that the mock satisfies the interface.
var _ service.BillingService = (*mockBillingService)(nil)

// ── Helpers ────────────────────────────────────────────────────────────────

// makeToken generates a valid HS256 JWT signed with testJWTSecret.
func makeToken(clinicID, role string) string {
	claims := jwt.MapClaims{
		"sub":       uuid.New().String(),
		"clinic_id": clinicID,
		"role":      role,
		"exp":       time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString([]byte(testJWTSecret))
	return signed
}

func newServer(svc service.BillingService) http.Handler {
	return handler.NewHTTPHandler(svc, testJWTSecret).Router()
}

func authedReq(method, url, body, clinicID string) *http.Request {
	var req *http.Request
	if body != "" {
		req, _ = http.NewRequest(method, url, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, _ = http.NewRequest(method, url, nil)
	}
	req.Header.Set("Authorization", "Bearer "+makeToken(clinicID, "admin"))
	return req
}

// ── Webhook tests ──────────────────────────────────────────────────────────

func TestPaymentWebhookHandler_Kaspi(t *testing.T) {
	svc := new(mockBillingService)
	srv := newServer(svc)

	payload := []byte(`{"transactionId":"tx-1","status":"SUCCESS","amount":"5000","currency":"KZT"}`)
	svc.On("ProcessWebhook", mock.Anything, domain.ProviderKaspi, payload, "valid-hmac").Return(nil)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/webhooks/kaspi", bytes.NewReader(payload))
	req.Header.Set("X-Kaspi-Signature", "valid-hmac")

	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	svc.AssertExpectations(t)
}

func TestPaymentWebhookHandler_Stripe(t *testing.T) {
	svc := new(mockBillingService)
	srv := newServer(svc)

	payload := []byte(`{"type":"checkout.session.completed","data":{"object":{}}}`)
	stripeSig := "t=1234,v1=abc"
	svc.On("ProcessWebhook", mock.Anything, domain.ProviderStripe, payload, stripeSig).Return(nil)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/webhooks/stripe", bytes.NewReader(payload))
	req.Header.Set("Stripe-Signature", stripeSig)

	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	svc.AssertExpectations(t)
}

func TestPaymentWebhookHandler_Kaspi_InvalidSignature(t *testing.T) {
	svc := new(mockBillingService)
	srv := newServer(svc)

	payload := []byte(`{"transactionId":"tx-1"}`)
	svc.On("ProcessWebhook", mock.Anything, domain.ProviderKaspi, payload, "bad").
		Return(domain.ErrInvalidSignature)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/webhooks/kaspi", bytes.NewReader(payload))
	req.Header.Set("X-Kaspi-Signature", "bad")

	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// ── Invoice tests ──────────────────────────────────────────────────────────

func TestInvoiceListHandler_FilterByStatus(t *testing.T) {
	svc := new(mockBillingService)
	clinicID := uuid.New()

	paidStatus := domain.InvoiceStatusPaid
	invoices := []*domain.Invoice{
		{
			ID:          uuid.New(),
			ClinicID:    clinicID,
			ServiceName: "MRI Scan",
			Amount:      decimal.NewFromFloat(120000),
			Currency:    "KZT",
			Status:      domain.InvoiceStatusPaid,
		},
	}

	svc.On("ListInvoices", mock.Anything, clinicID, mock.MatchedBy(func(f repository.InvoiceFilter) bool {
		return f.Status != nil && *f.Status == paidStatus
	})).Return(invoices, nil)

	srv := newServer(svc)
	req := authedReq(http.MethodGet, "/api/v1/invoices?status=paid", "", clinicID.String())
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var got []*domain.Invoice
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	require.Len(t, got, 1)
	assert.Equal(t, domain.InvoiceStatusPaid, got[0].Status)
	svc.AssertExpectations(t)
}

func TestGenerateInvoicePDF(t *testing.T) {
	svc := new(mockBillingService)
	clinicID := uuid.New()
	invoiceID := uuid.New()
	pdfContent := []byte("INVOICE PDF " + invoiceID.String())

	svc.On("GenerateInvoicePDF", mock.Anything, invoiceID).Return(pdfContent, nil)

	srv := newServer(svc)
	req := authedReq(http.MethodGet, "/api/v1/invoices/"+invoiceID.String()+"/pdf", "", clinicID.String())
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/pdf", rr.Header().Get("Content-Type"))
	assert.True(t, strings.Contains(rr.Header().Get("Content-Disposition"), invoiceID.String()))
	assert.Equal(t, pdfContent, rr.Body.Bytes())
	svc.AssertExpectations(t)
}

func TestGetInvoice_NotFound(t *testing.T) {
	svc := new(mockBillingService)
	clinicID := uuid.New()
	invoiceID := uuid.New()

	svc.On("GetInvoice", mock.Anything, invoiceID).Return(nil, domain.ErrInvoiceNotFound)

	srv := newServer(svc)
	req := authedReq(http.MethodGet, "/api/v1/invoices/"+invoiceID.String(), "", clinicID.String())
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestListPlans_Unauthenticated(t *testing.T) {
	svc := new(mockBillingService)
	srv := newServer(svc)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/plans", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	// No auth header → 401
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}
