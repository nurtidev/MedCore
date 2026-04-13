package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/nurtidev/medcore/internal/integration/domain"
	"github.com/nurtidev/medcore/internal/integration/service"
	"github.com/rs/zerolog"
)

// HTTP — HTTP-хендлер integration-service.
type HTTP struct {
	svc service.IntegrationService
	log zerolog.Logger
}

// NewHTTP регистрирует все маршруты и возвращает http.Handler.
func NewHTTP(svc service.IntegrationService, log zerolog.Logger) http.Handler {
	h := &HTTP{svc: svc, log: log}

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	r.Get("/health", h.health)
	r.Get("/ready", h.ready)

	r.Route("/api/v1", func(r chi.Router) {
		// ГосAPI
		r.Post("/gov/validate-iin", h.validateIIN)
		r.Get("/gov/patient-status/{iin}", h.patientStatus)

		// Агрегаторы
		r.Post("/sync/appointments/{clinic_id}", h.syncAppointments)
		r.Get("/sync/logs/{clinic_id}", h.listSyncLogs)

		// Лаборатории
		r.Get("/lab-results/{clinic_id}", h.listLabResults)
		r.Post("/lab-results/{id}/attach/{patient_id}", h.attachLabResult)

		// Конфиги интеграций
		r.Get("/integrations/{clinic_id}", h.listIntegrations)
		r.Put("/integrations/{clinic_id}/{provider}", h.upsertIntegration)
	})

	return r
}

// ── Health ────────────────────────────────────────────────────────────────────

func (h *HTTP) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *HTTP) ready(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

// ── ГосAPI ────────────────────────────────────────────────────────────────────

func (h *HTTP) validateIIN(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IIN string `json:"iin"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.IIN == "" {
		writeError(w, r, http.StatusBadRequest, "invalid_input", "iin is required")
		return
	}

	info, err := h.svc.ValidateIIN(r.Context(), req.IIN)
	if err != nil {
		h.handleErr(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, info)
}

func (h *HTTP) patientStatus(w http.ResponseWriter, r *http.Request) {
	iin := chi.URLParam(r, "iin")
	if iin == "" {
		writeError(w, r, http.StatusBadRequest, "invalid_input", "iin is required")
		return
	}

	status, err := h.svc.GetPatientStatus(r.Context(), iin)
	if err != nil {
		h.handleErr(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"iin": iin, "status": status})
}

// ── Агрегаторы ────────────────────────────────────────────────────────────────

func (h *HTTP) syncAppointments(w http.ResponseWriter, r *http.Request) {
	clinicID, err := uuid.Parse(chi.URLParam(r, "clinic_id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_input", "invalid clinic_id")
		return
	}

	result, err := h.svc.SyncAppointments(r.Context(), clinicID)
	if err != nil {
		h.handleErr(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *HTTP) listSyncLogs(w http.ResponseWriter, r *http.Request) {
	clinicID, err := uuid.Parse(chi.URLParam(r, "clinic_id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_input", "invalid clinic_id")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	// SyncLogs доступны через repo — здесь возвращаем пустой список как placeholder
	// В production добавить метод GetSyncLogs в IntegrationService
	_ = clinicID
	_ = limit
	_ = offset
	writeJSON(w, http.StatusOK, map[string]any{"logs": []any{}, "total": 0})
}

// ── Лаборатории ───────────────────────────────────────────────────────────────

func (h *HTTP) listLabResults(w http.ResponseWriter, r *http.Request) {
	clinicID, err := uuid.Parse(chi.URLParam(r, "clinic_id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_input", "invalid clinic_id")
		return
	}

	results, err := h.svc.FetchLabResults(r.Context(), clinicID)
	if err != nil {
		h.handleErr(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

func (h *HTTP) attachLabResult(w http.ResponseWriter, r *http.Request) {
	resultID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_input", "invalid result id")
		return
	}
	patientID, err := uuid.Parse(chi.URLParam(r, "patient_id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_input", "invalid patient_id")
		return
	}

	if err := h.svc.AttachResultToPatient(r.Context(), resultID, patientID); err != nil {
		h.handleErr(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ── Конфиги ───────────────────────────────────────────────────────────────────

func (h *HTTP) listIntegrations(w http.ResponseWriter, r *http.Request) {
	clinicID, err := uuid.Parse(chi.URLParam(r, "clinic_id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_input", "invalid clinic_id")
		return
	}

	// Возвращаем конфиги по всем известным провайдерам
	providers := []string{"idoctor", "olymp", "invivo"}
	var configs []*domain.IntegrationConfig
	for _, p := range providers {
		cfg, err := h.svc.GetIntegrationConfig(r.Context(), clinicID, p)
		if err != nil {
			if errors.Is(err, domain.ErrIntegrationNotConfigured) {
				continue
			}
			h.handleErr(w, r, err)
			return
		}
		configs = append(configs, cfg)
	}

	writeJSON(w, http.StatusOK, map[string]any{"integrations": configs})
}

func (h *HTTP) upsertIntegration(w http.ResponseWriter, r *http.Request) {
	clinicID, err := uuid.Parse(chi.URLParam(r, "clinic_id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_input", "invalid clinic_id")
		return
	}
	provider := chi.URLParam(r, "provider")

	var body struct {
		IsActive bool           `json:"is_active"`
		Config   map[string]any `json:"config"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}

	req := domain.UpsertConfigRequest{
		ClinicID: clinicID,
		Provider: provider,
		IsActive: body.IsActive,
		Config:   body.Config,
	}
	if err := h.svc.UpsertIntegrationConfig(r.Context(), req); err != nil {
		h.handleErr(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (h *HTTP) handleErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrIINInvalid):
		writeError(w, r, http.StatusBadRequest, "invalid_iin", "invalid IIN format")
	case errors.Is(err, domain.ErrIntegrationNotConfigured):
		writeError(w, r, http.StatusNotFound, "not_configured", "integration not configured")
	case errors.Is(err, domain.ErrLabResultNotFound):
		writeError(w, r, http.StatusNotFound, "not_found", "lab result not found")
	case errors.Is(err, domain.ErrCircuitBreakerOpen):
		writeError(w, r, http.StatusServiceUnavailable, "service_unavailable", "external service temporarily unavailable")
	case errors.Is(err, domain.ErrExternalServiceUnavailable):
		writeError(w, r, http.StatusServiceUnavailable, "service_unavailable", "external service unavailable")
	case errors.Is(err, domain.ErrInvalidInput):
		writeError(w, r, http.StatusBadRequest, "invalid_input", err.Error())
	default:
		h.log.Error().Err(err).Str("request_id", r.Header.Get("X-Request-ID")).Msg("unhandled error")
		writeError(w, r, http.StatusInternalServerError, "internal", "internal server error")
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	writeJSON(w, status, map[string]string{
		"error":      code,
		"message":    message,
		"request_id": r.Header.Get("X-Request-ID"),
	})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_json", "invalid request body")
		return false
	}
	return true
}
