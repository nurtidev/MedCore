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
	"github.com/nurtidev/medcore/internal/billing/domain"
	"github.com/nurtidev/medcore/internal/billing/repository"
	"github.com/nurtidev/medcore/internal/billing/service"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/shopspring/decimal"
)

type ctxKey string

const (
	ctxClinicID ctxKey = "clinic_id"
	ctxUserID   ctxKey = "user_id"
	ctxRole     ctxKey = "role"
)

// HTTPHandler holds all REST handlers for the billing service.
type HTTPHandler struct {
	svc       service.BillingService
	jwtSecret string
}

func NewHTTPHandler(svc service.BillingService, jwtSecret string) *HTTPHandler {
	return &HTTPHandler{svc: svc, jwtSecret: jwtSecret}
}

// Router builds and returns the chi router with all routes mounted.
func (h *HTTPHandler) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	r.Get("/ready", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	r.Handle("/metrics", promhttp.Handler())

	// Webhooks — no JWT auth, signature verified in handler
	wh := &WebhookHandler{svc: h.svc}
	r.Post("/api/v1/webhooks/kaspi", wh.KaspiWebhook)
	r.Post("/api/v1/webhooks/stripe", wh.StripeWebhook)

	r.Group(func(r chi.Router) {
		r.Use(h.jwtMiddleware)

		r.Post("/api/v1/payments", h.CreatePayment)
		r.Get("/api/v1/payments/{id}", h.GetPayment)

		r.Post("/api/v1/invoices", h.CreateInvoice)
		r.Get("/api/v1/invoices", h.ListInvoices)
		r.Get("/api/v1/invoices/{id}", h.GetInvoice)
		r.Get("/api/v1/invoices/{id}/pdf", h.GetInvoicePDF)

		r.Get("/api/v1/plans", h.ListPlans)
		r.Get("/api/v1/subscriptions/current", h.GetSubscription)
		r.Post("/api/v1/subscriptions", h.CreateSubscription)
		r.Delete("/api/v1/subscriptions/current", h.CancelSubscription)
	})

	return r
}

// ── JWT middleware ─────────────────────────────────────────────────────────

func (h *HTTPHandler) jwtMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			writeError(w, http.StatusUnauthorized, "missing or invalid Authorization header")
			return
		}
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return []byte(h.jwtSecret), nil
		})
		if err != nil || !token.Valid {
			writeError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			writeError(w, http.StatusUnauthorized, "invalid token claims")
			return
		}

		ctx := r.Context()
		if v, ok := claims["clinic_id"].(string); ok {
			ctx = context.WithValue(ctx, ctxClinicID, v)
		}
		if v, ok := claims["sub"].(string); ok {
			ctx = context.WithValue(ctx, ctxUserID, v)
		}
		if v, ok := claims["role"].(string); ok {
			ctx = context.WithValue(ctx, ctxRole, v)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ── Payment handlers ───────────────────────────────────────────────────────

func (h *HTTPHandler) CreatePayment(w http.ResponseWriter, r *http.Request) {
	var req struct {
		InvoiceID      string `json:"invoice_id"`
		Provider       string `json:"provider"`
		IdempotencyKey string `json:"idempotency_key"`
		ReturnURL      string `json:"return_url"`
		Description    string `json:"description"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.IdempotencyKey == "" {
		writeError(w, http.StatusBadRequest, "idempotency_key is required")
		return
	}

	clinicID, err := clinicIDFromCtx(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "clinic_id not in token")
		return
	}
	invoiceID, err := uuid.Parse(req.InvoiceID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid invoice_id")
		return
	}

	invoice, err := h.svc.GetInvoice(r.Context(), invoiceID)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	link, err := h.svc.CreatePaymentLink(r.Context(), service.CreatePaymentRequest{
		InvoiceID:      invoiceID,
		ClinicID:       clinicID,
		PatientID:      invoice.PatientID,
		Amount:         invoice.Amount,
		Currency:       invoice.Currency,
		Provider:       domain.PaymentProvider(req.Provider),
		IdempotencyKey: req.IdempotencyKey,
		ReturnURL:      req.ReturnURL,
		Description:    req.Description,
	})
	if err != nil {
		handleDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, link)
}

func (h *HTTPHandler) GetPayment(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid payment id")
		return
	}
	p, err := h.svc.GetPayment(r.Context(), id)
	if err != nil {
		handleDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// ── Invoice handlers ───────────────────────────────────────────────────────

func (h *HTTPHandler) CreateInvoice(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PatientID   string `json:"patient_id"`
		ServiceName string `json:"service_name"`
		Amount      string `json:"amount"`
		Currency    string `json:"currency"`
		DueAt       string `json:"due_at"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	clinicID, err := clinicIDFromCtx(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "clinic_id not in token")
		return
	}
	amount, err := decimal.NewFromString(req.Amount)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid amount")
		return
	}
	dueAt, err := time.Parse(time.RFC3339, req.DueAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid due_at, expected RFC3339")
		return
	}

	svcReq := service.CreateInvoiceRequest{
		ClinicID:    clinicID,
		ServiceName: req.ServiceName,
		Amount:      amount,
		Currency:    req.Currency,
		DueAt:       dueAt,
	}
	if req.PatientID != "" {
		pid, err := uuid.Parse(req.PatientID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid patient_id")
			return
		}
		svcReq.PatientID = &pid
	}

	inv, err := h.svc.CreateInvoice(r.Context(), svcReq)
	if err != nil {
		handleDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, inv)
}

func (h *HTTPHandler) ListInvoices(w http.ResponseWriter, r *http.Request) {
	clinicID, err := clinicIDFromCtx(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "clinic_id not in token")
		return
	}

	filter := repository.InvoiceFilter{Limit: 50}
	if s := r.URL.Query().Get("status"); s != "" {
		st := domain.InvoiceStatus(s)
		filter.Status = &st
	}
	if p := r.URL.Query().Get("patient_id"); p != "" {
		if pid, err := uuid.Parse(p); err == nil {
			filter.PatientID = &pid
		}
	}

	invoices, err := h.svc.ListInvoices(r.Context(), clinicID, filter)
	if err != nil {
		handleDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, invoices)
}

func (h *HTTPHandler) GetInvoice(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid invoice id")
		return
	}
	inv, err := h.svc.GetInvoice(r.Context(), id)
	if err != nil {
		handleDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, inv)
}

func (h *HTTPHandler) GetInvoicePDF(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid invoice id")
		return
	}
	data, err := h.svc.GenerateInvoicePDF(r.Context(), id)
	if err != nil {
		handleDomainError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "attachment; filename=invoice-"+id.String()+".pdf")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// ── Subscription handlers ──────────────────────────────────────────────────

func (h *HTTPHandler) GetSubscription(w http.ResponseWriter, r *http.Request) {
	clinicID, err := clinicIDFromCtx(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "clinic_id not in token")
		return
	}
	sub, err := h.svc.GetSubscription(r.Context(), clinicID)
	if err != nil {
		handleDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sub)
}

func (h *HTTPHandler) CreateSubscription(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PlanID string `json:"plan_id"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	clinicID, err := clinicIDFromCtx(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "clinic_id not in token")
		return
	}
	planID, err := uuid.Parse(req.PlanID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid plan_id")
		return
	}
	sub, err := h.svc.CreateSubscription(r.Context(), clinicID, planID)
	if err != nil {
		handleDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, sub)
}

func (h *HTTPHandler) CancelSubscription(w http.ResponseWriter, r *http.Request) {
	clinicID, err := clinicIDFromCtx(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "clinic_id not in token")
		return
	}
	if err := h.svc.CancelSubscription(r.Context(), clinicID); err != nil {
		handleDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *HTTPHandler) ListPlans(w http.ResponseWriter, r *http.Request) {
	plans, err := h.svc.ListPlans(r.Context())
	if err != nil {
		handleDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, plans)
}

// ── Helpers ────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return false
	}
	return true
}

func parseUUIDParam(r *http.Request, param string) (uuid.UUID, error) {
	return uuid.Parse(chi.URLParam(r, param))
}

func clinicIDFromCtx(r *http.Request) (uuid.UUID, error) {
	v, ok := r.Context().Value(ctxClinicID).(string)
	if !ok || v == "" {
		return uuid.Nil, errors.New("clinic_id missing from context")
	}
	return uuid.Parse(v)
}

func handleDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInvoiceNotFound),
		errors.Is(err, domain.ErrPaymentNotFound),
		errors.Is(err, domain.ErrSubscriptionNotFound),
		errors.Is(err, domain.ErrPlanNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrInvalidSignature):
		writeError(w, http.StatusUnauthorized, err.Error())
	case errors.Is(err, domain.ErrSubscriptionExpired),
		errors.Is(err, domain.ErrSubscriptionInactive):
		writeError(w, http.StatusPaymentRequired, err.Error())
	case errors.Is(err, domain.ErrPaymentDuplicate):
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}
