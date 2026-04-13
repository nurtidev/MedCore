package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	authdomain "github.com/nurtidev/medcore/internal/auth/domain"
	"github.com/nurtidev/medcore/internal/analytics/domain"
	sharedlog "github.com/nurtidev/medcore/internal/shared/logger"
)

// ─── context key ─────────────────────────────────────────────────────────────

type ctxKey string

const ctxKeyClaims ctxKey = "analytics_claims"

// ─── HTTP handler ─────────────────────────────────────────────────────────────

type HTTP struct {
	svc       domain.AnalyticsService
	jwtSecret []byte
	log       zerolog.Logger
}

// NewHTTP wires all analytics routes.
func NewHTTP(svc domain.AnalyticsService, jwtSecret []byte, log zerolog.Logger) http.Handler {
	h := &HTTP{svc: svc, jwtSecret: jwtSecret, log: log}

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(sharedlog.HTTPMiddleware(log))

	r.Get("/health", h.health)
	r.Get("/ready", h.ready)

	r.Route("/api/v1/analytics", func(r chi.Router) {
		r.Use(h.jwtMiddleware)

		// All-KPI dashboard
		r.Get("/dashboard", h.getDashboard)

		// Individual KPI
		r.Get("/doctors/workload", h.getDoctorWorkload)
		r.Get("/revenue", h.getRevenue)
		r.Get("/schedule/fill-rate", h.getFillRate)
		r.Get("/patients/funnel", h.getPatientFunnel)

		// Export
		r.Get("/export/excel", h.exportExcel)
		r.Get("/export/csv", h.exportCSV)

		// BI sync
		r.Get("/bi-sync", h.biSync)
	})

	return r
}

// ─── Health ───────────────────────────────────────────────────────────────────

func (h *HTTP) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *HTTP) ready(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

// ─── Dashboard ────────────────────────────────────────────────────────────────

func (h *HTTP) getDashboard(w http.ResponseWriter, r *http.Request) {
	clinicID, period, ok := h.extractClinicPeriod(w, r)
	if !ok {
		return
	}
	if !h.authorizeClinic(w, r, clinicID) {
		return
	}

	dash, err := h.svc.GetDashboard(r.Context(), clinicID, period)
	if err != nil {
		h.handleServiceErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, dash)
}

// ─── Doctor workload ──────────────────────────────────────────────────────────

func (h *HTTP) getDoctorWorkload(w http.ResponseWriter, r *http.Request) {
	clinicID, period, ok := h.extractClinicPeriod(w, r)
	if !ok {
		return
	}
	if !h.authorizeClinic(w, r, clinicID) {
		return
	}

	req := domain.WorkloadRequest{ClinicID: clinicID, Period: period}

	// Doctor can filter by their own ID only.
	if rawDoctorID := r.URL.Query().Get("doctor_id"); rawDoctorID != "" {
		did, err := uuid.Parse(rawDoctorID)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_doctor_id", "doctor_id must be a UUID")
			return
		}
		req.DoctorID = &did
	} else {
		// Doctors can only see their own data.
		if !h.enforceDoctor(w, r, &req) {
			return
		}
	}

	result, err := h.svc.GetDoctorWorkload(r.Context(), req)
	if err != nil {
		h.handleServiceErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// ─── Revenue ──────────────────────────────────────────────────────────────────

func (h *HTTP) getRevenue(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	clinicID, err := uuid.Parse(q.Get("clinic_id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_clinic_id", "clinic_id must be a UUID")
		return
	}
	if !h.authorizeClinic(w, r, clinicID) {
		return
	}

	start, err := time.Parse("2006-01-02", q.Get("start"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_start", "start must be YYYY-MM-DD")
		return
	}
	end, err := time.Parse("2006-01-02", q.Get("end"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_end", "end must be YYYY-MM-DD")
		return
	}

	grouping := q.Get("grouping")
	if grouping == "" {
		grouping = "day"
	}

	result, err := h.svc.GetClinicRevenue(r.Context(), domain.RevenueRequest{
		ClinicID:  clinicID,
		StartDate: start,
		EndDate:   end,
		Grouping:  grouping,
	})
	if err != nil {
		h.handleServiceErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// ─── Fill rate ────────────────────────────────────────────────────────────────

func (h *HTTP) getFillRate(w http.ResponseWriter, r *http.Request) {
	clinicID, period, ok := h.extractClinicPeriod(w, r)
	if !ok {
		return
	}
	if !h.authorizeClinic(w, r, clinicID) {
		return
	}

	result, err := h.svc.GetScheduleFillRate(r.Context(), domain.FillRateRequest{
		ClinicID: clinicID,
		Period:   period,
	})
	if err != nil {
		h.handleServiceErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// ─── Patient funnel ───────────────────────────────────────────────────────────

func (h *HTTP) getPatientFunnel(w http.ResponseWriter, r *http.Request) {
	clinicID, period, ok := h.extractClinicPeriod(w, r)
	if !ok {
		return
	}
	if !h.authorizeClinic(w, r, clinicID) {
		return
	}

	result, err := h.svc.GetPatientFunnel(r.Context(), domain.FunnelRequest{
		ClinicID: clinicID,
		Period:   period,
	})
	if err != nil {
		h.handleServiceErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// ─── Export ───────────────────────────────────────────────────────────────────

func (h *HTTP) exportExcel(w http.ResponseWriter, r *http.Request) {
	req, ok := h.buildExportRequest(w, r)
	if !ok {
		return
	}

	data, err := h.svc.ExportToExcel(r.Context(), req)
	if err != nil {
		h.handleServiceErr(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=\"report.xlsx\"")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *HTTP) exportCSV(w http.ResponseWriter, r *http.Request) {
	req, ok := h.buildExportRequest(w, r)
	if !ok {
		return
	}

	data, err := h.svc.ExportToCSV(r.Context(), req)
	if err != nil {
		h.handleServiceErr(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\"report.csv\"")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *HTTP) buildExportRequest(w http.ResponseWriter, r *http.Request) (domain.ExportRequest, bool) {
	q := r.URL.Query()
	clinicID, period, ok := h.extractClinicPeriod(w, r)
	if !ok {
		return domain.ExportRequest{}, false
	}
	if !h.authorizeClinic(w, r, clinicID) {
		return domain.ExportRequest{}, false
	}
	exportType := q.Get("type")
	if exportType == "" {
		writeError(w, r, http.StatusBadRequest, "missing_type", "type is required")
		return domain.ExportRequest{}, false
	}
	return domain.ExportRequest{ClinicID: clinicID, Period: period, Type: exportType}, true
}

// ─── BI sync ──────────────────────────────────────────────────────────────────

// biSync returns raw event data as JSON for external BI tools.
func (h *HTTP) biSync(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	clinicID, err := uuid.Parse(q.Get("clinic_id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_clinic_id", "clinic_id must be a UUID")
		return
	}
	if !h.authorizeClinic(w, r, clinicID) {
		return
	}

	from, err := time.Parse("2006-01-02", q.Get("from"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_from", "from must be YYYY-MM-DD")
		return
	}
	to, err := time.Parse("2006-01-02", q.Get("to"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_to", "to must be YYYY-MM-DD")
		return
	}

	// Return aggregated revenue as BI-sync payload.
	revenue, err := h.svc.GetClinicRevenue(r.Context(), domain.RevenueRequest{
		ClinicID:  clinicID,
		StartDate: from,
		EndDate:   to,
		Grouping:  "day",
	})
	if err != nil {
		h.handleServiceErr(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"clinic_id": clinicID,
		"from":      from.Format("2006-01-02"),
		"to":        to.Format("2006-01-02"),
		"revenue":   revenue,
	})
}

// ─── JWT middleware ───────────────────────────────────────────────────────────

func (h *HTTP) jwtMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := extractBearer(r)
		if tokenStr == "" {
			writeError(w, r, http.StatusUnauthorized, "unauthorized", "missing bearer token")
			return
		}

		claims := &authdomain.Claims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return h.jwtSecret, nil
		})
		if err != nil || !token.Valid {
			writeError(w, r, http.StatusUnauthorized, "token_invalid", "invalid or expired token")
			return
		}

		ctx := context.WithValue(r.Context(), ctxKeyClaims, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func claimsFromCtx(ctx context.Context) (*authdomain.Claims, bool) {
	c, ok := ctx.Value(ctxKeyClaims).(*authdomain.Claims)
	return c, ok
}

// ─── RBAC helpers ─────────────────────────────────────────────────────────────

// authorizeClinic returns true if the caller may access analytics for clinicID.
// super_admin → all clinics; admin/doctor → own clinic only.
func (h *HTTP) authorizeClinic(w http.ResponseWriter, r *http.Request, clinicID uuid.UUID) bool {
	claims, ok := claimsFromCtx(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthorized", "missing claims")
		return false
	}

	switch claims.Role {
	case authdomain.RoleSuperAdmin:
		return true
	case authdomain.RoleAdmin, authdomain.RoleDoctor:
		if claims.ClinicID != clinicID {
			writeError(w, r, http.StatusForbidden, "forbidden", "access to this clinic is not allowed")
			return false
		}
		return true
	default:
		writeError(w, r, http.StatusForbidden, "forbidden", "analytics access requires admin or doctor role")
		return false
	}
}

// enforceDoctor sets DoctorID on the request to the caller's UserID when role=doctor.
// Returns false (and writes error) if role checks fail.
func (h *HTTP) enforceDoctor(w http.ResponseWriter, r *http.Request, req *domain.WorkloadRequest) bool {
	claims, ok := claimsFromCtx(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthorized", "missing claims")
		return false
	}
	if claims.Role == authdomain.RoleDoctor {
		req.DoctorID = &claims.UserID
	}
	return true
}

// ─── Query helpers ────────────────────────────────────────────────────────────

func (h *HTTP) extractClinicPeriod(w http.ResponseWriter, r *http.Request) (uuid.UUID, string, bool) {
	q := r.URL.Query()

	clinicID, err := uuid.Parse(q.Get("clinic_id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_clinic_id", "clinic_id must be a UUID")
		return uuid.Nil, "", false
	}

	period := q.Get("period")
	if period == "" {
		writeError(w, r, http.StatusBadRequest, "missing_period", "period is required (YYYY-MM)")
		return uuid.Nil, "", false
	}

	return clinicID, period, true
}

func (h *HTTP) handleServiceErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidPeriod):
		writeError(w, r, http.StatusBadRequest, "invalid_period", err.Error())
	case errors.Is(err, domain.ErrInvalidGrouping):
		writeError(w, r, http.StatusBadRequest, "invalid_grouping", err.Error())
	case errors.Is(err, domain.ErrNoData):
		writeJSON(w, http.StatusOK, map[string]any{"data": nil, "message": "no data for requested period"})
	default:
		h.log.Error().Err(err).Str("path", r.URL.Path).Msg("service error")
		writeError(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

// ─── JSON helpers ─────────────────────────────────────────────────────────────

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

func extractBearer(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
