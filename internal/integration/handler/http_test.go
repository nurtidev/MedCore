package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nurtidev/medcore/internal/integration/domain"
	"github.com/nurtidev/medcore/internal/integration/handler"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ── Mock сервиса ──────────────────────────────────────────────────────────────

type mockSvc struct{ mock.Mock }

func (m *mockSvc) ValidateIIN(ctx context.Context, iin string) (*domain.PatientInfo, error) {
	args := m.Called(ctx, iin)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.PatientInfo), args.Error(1)
}

func (m *mockSvc) GetPatientStatus(ctx context.Context, iin string) (string, error) {
	args := m.Called(ctx, iin)
	return args.String(0), args.Error(1)
}

func (m *mockSvc) SyncAppointments(ctx context.Context, clinicID uuid.UUID) (*domain.SyncResult, error) {
	args := m.Called(ctx, clinicID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SyncResult), args.Error(1)
}

func (m *mockSvc) HandleIncomingAppointment(ctx context.Context, payload domain.WebhookPayload) error {
	return m.Called(ctx, payload).Error(0)
}

func (m *mockSvc) FetchLabResults(ctx context.Context, clinicID uuid.UUID) ([]*domain.LabResult, error) {
	args := m.Called(ctx, clinicID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.LabResult), args.Error(1)
}

func (m *mockSvc) AttachResultToPatient(ctx context.Context, resultID, patientID uuid.UUID) error {
	return m.Called(ctx, resultID, patientID).Error(0)
}

func (m *mockSvc) HandleLabWebhook(ctx context.Context, provider string, payload []byte) error {
	return m.Called(ctx, provider, payload).Error(0)
}

func (m *mockSvc) GetIntegrationConfig(ctx context.Context, clinicID uuid.UUID, provider string) (*domain.IntegrationConfig, error) {
	args := m.Called(ctx, clinicID, provider)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.IntegrationConfig), args.Error(1)
}

func (m *mockSvc) UpsertIntegrationConfig(ctx context.Context, req domain.UpsertConfigRequest) error {
	return m.Called(ctx, req).Error(0)
}

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestHTTP_Health(t *testing.T) {
	svc := &mockSvc{}
	h := handler.NewHTTP(svc, zerolog.Nop())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHTTP_ValidateIIN_Success(t *testing.T) {
	svc := &mockSvc{}
	h := handler.NewHTTP(svc, zerolog.Nop())

	info := &domain.PatientInfo{IIN: "860101123456", IsValid: true, FirstName: "Асель"}
	svc.On("ValidateIIN", mock.Anything, "860101123456").Return(info, nil)

	body, _ := json.Marshal(map[string]string{"iin": "860101123456"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/gov/validate-iin", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp domain.PatientInfo
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "860101123456", resp.IIN)
}

func TestHTTP_ValidateIIN_InvalidIIN(t *testing.T) {
	svc := &mockSvc{}
	h := handler.NewHTTP(svc, zerolog.Nop())

	svc.On("ValidateIIN", mock.Anything, "123").Return(nil, domain.ErrIINInvalid)

	body, _ := json.Marshal(map[string]string{"iin": "123"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/gov/validate-iin", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHTTP_SyncAppointments_Success(t *testing.T) {
	svc := &mockSvc{}
	h := handler.NewHTTP(svc, zerolog.Nop())

	clinicID := uuid.New()
	result := &domain.SyncResult{
		Provider:   "idoctor",
		ClinicID:   clinicID,
		Created:    5,
		StartedAt:  time.Now(),
		FinishedAt: time.Now(),
	}
	svc.On("SyncAppointments", mock.Anything, clinicID).Return(result, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sync/appointments/"+clinicID.String(), nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHTTP_SyncAppointments_IntegrationNotConfigured(t *testing.T) {
	svc := &mockSvc{}
	h := handler.NewHTTP(svc, zerolog.Nop())

	clinicID := uuid.New()
	svc.On("SyncAppointments", mock.Anything, clinicID).Return(nil, domain.ErrIntegrationNotConfigured)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sync/appointments/"+clinicID.String(), nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHTTP_AttachLabResult_Success(t *testing.T) {
	svc := &mockSvc{}
	h := handler.NewHTTP(svc, zerolog.Nop())

	resultID := uuid.New()
	patientID := uuid.New()
	svc.On("AttachResultToPatient", mock.Anything, resultID, patientID).Return(nil)

	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/lab-results/"+resultID.String()+"/attach/"+patientID.String(), nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestHTTP_UpsertIntegration(t *testing.T) {
	svc := &mockSvc{}
	h := handler.NewHTTP(svc, zerolog.Nop())

	clinicID := uuid.New()
	svc.On("UpsertIntegrationConfig", mock.Anything, mock.AnythingOfType("domain.UpsertConfigRequest")).Return(nil)

	body, _ := json.Marshal(map[string]any{
		"is_active": true,
		"config":    map[string]any{"api_key": "test"},
	})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/integrations/"+clinicID.String()+"/idoctor", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}
