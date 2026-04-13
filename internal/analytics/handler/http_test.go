package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	authdomain "github.com/nurtidev/medcore/internal/auth/domain"
	"github.com/nurtidev/medcore/internal/analytics/domain"
	"github.com/nurtidev/medcore/internal/analytics/handler"
)

// ─── Mock service ─────────────────────────────────────────────────────────────

type mockSvc struct{ mock.Mock }

func (m *mockSvc) GetDoctorWorkload(ctx context.Context, req domain.WorkloadRequest) ([]*domain.DoctorWorkload, error) {
	args := m.Called(ctx, req)
	return args.Get(0).([]*domain.DoctorWorkload), args.Error(1)
}

func (m *mockSvc) GetClinicRevenue(ctx context.Context, req domain.RevenueRequest) (*domain.ClinicRevenue, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*domain.ClinicRevenue), args.Error(1)
}

func (m *mockSvc) GetScheduleFillRate(ctx context.Context, req domain.FillRateRequest) (*domain.ScheduleFillRate, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*domain.ScheduleFillRate), args.Error(1)
}

func (m *mockSvc) GetPatientFunnel(ctx context.Context, req domain.FunnelRequest) (*domain.PatientFunnel, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*domain.PatientFunnel), args.Error(1)
}

func (m *mockSvc) GetDashboard(ctx context.Context, clinicID uuid.UUID, period string) (*domain.Dashboard, error) {
	args := m.Called(ctx, clinicID, period)
	return args.Get(0).(*domain.Dashboard), args.Error(1)
}

func (m *mockSvc) ExportToExcel(ctx context.Context, req domain.ExportRequest) ([]byte, error) {
	args := m.Called(ctx, req)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *mockSvc) ExportToCSV(ctx context.Context, req domain.ExportRequest) ([]byte, error) {
	args := m.Called(ctx, req)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *mockSvc) RecordEvent(ctx context.Context, event *domain.ClinicEvent) error {
	return m.Called(ctx, event).Error(0)
}

func (m *mockSvc) RecordEventBatch(ctx context.Context, events []*domain.ClinicEvent) error {
	return m.Called(ctx, events).Error(0)
}

// ─── Test helpers ─────────────────────────────────────────────────────────────

var testSecret = []byte("test-secret-key")

func makeToken(role authdomain.Role, clinicID uuid.UUID) string {
	claims := authdomain.Claims{
		UserID:   uuid.New(),
		ClinicID: clinicID,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	tok, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(testSecret)
	return tok
}

func newHandler(svc domain.AnalyticsService) http.Handler {
	return handler.NewHTTP(svc, testSecret, zerolog.Nop())
}

// ─── Tests ────────────────────────────────────────────────────────────────────

func TestHealth(t *testing.T) {
	svc := &mockSvc{}
	h := newHandler(svc)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "ok", body["status"])
}

func TestGetDashboard_Unauthorized(t *testing.T) {
	svc := &mockSvc{}
	h := newHandler(svc)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/dashboard?clinic_id="+uuid.New().String()+"&period=2025-04", nil)
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGetDashboard_AdminSuccess(t *testing.T) {
	clinicID := uuid.New()
	svc := &mockSvc{}

	expected := &domain.Dashboard{ClinicID: clinicID.String(), Period: "2025-04"}
	svc.On("GetDashboard", mock.Anything, clinicID, "2025-04").Return(expected, nil)

	h := newHandler(svc)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/dashboard?clinic_id="+clinicID.String()+"&period=2025-04", nil)
	r.Header.Set("Authorization", "Bearer "+makeToken(authdomain.RoleAdmin, clinicID))
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestGetDashboard_AdminForbiddenOtherClinic(t *testing.T) {
	myClinic := uuid.New()
	otherClinic := uuid.New()

	svc := &mockSvc{}
	h := newHandler(svc)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/dashboard?clinic_id="+otherClinic.String()+"&period=2025-04", nil)
	r.Header.Set("Authorization", "Bearer "+makeToken(authdomain.RoleAdmin, myClinic))
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestGetDashboard_SuperAdminAllClinics(t *testing.T) {
	anyClinic := uuid.New()
	svc := &mockSvc{}

	expected := &domain.Dashboard{ClinicID: anyClinic.String(), Period: "2025-04"}
	svc.On("GetDashboard", mock.Anything, anyClinic, "2025-04").Return(expected, nil)

	h := newHandler(svc)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/dashboard?clinic_id="+anyClinic.String()+"&period=2025-04", nil)
	// super_admin has a different clinic ID — should still succeed.
	r.Header.Set("Authorization", "Bearer "+makeToken(authdomain.RoleSuperAdmin, uuid.New()))
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestGetRevenue_MissingParams(t *testing.T) {
	clinicID := uuid.New()
	svc := &mockSvc{}
	h := newHandler(svc)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/revenue?clinic_id="+clinicID.String(), nil)
	r.Header.Set("Authorization", "Bearer "+makeToken(authdomain.RoleAdmin, clinicID))
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestExportCSV_Success(t *testing.T) {
	clinicID := uuid.New()
	svc := &mockSvc{}

	csvData := []byte("Date,Revenue,Count\n2025-04-01,5000.00,1\n")
	svc.On("ExportToCSV", mock.Anything, domain.ExportRequest{
		ClinicID: clinicID,
		Period:   "2025-04",
		Type:     "revenue",
	}).Return(csvData, nil)

	h := newHandler(svc)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/export/csv?clinic_id="+clinicID.String()+"&period=2025-04&type=revenue", nil)
	r.Header.Set("Authorization", "Bearer "+makeToken(authdomain.RoleAdmin, clinicID))
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/csv")
	svc.AssertExpectations(t)
}
