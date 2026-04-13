package handler

import (
	"io"
	"net/http"

	"github.com/nurtidev/medcore/internal/billing/domain"
	"github.com/nurtidev/medcore/internal/billing/service"
	"github.com/nurtidev/medcore/internal/shared/logger"
)

// WebhookHandler processes incoming callbacks from Kaspi Pay and Stripe.
type WebhookHandler struct {
	svc service.BillingService
}

func NewWebhookHandler(svc service.BillingService) *WebhookHandler {
	return &WebhookHandler{svc: svc}
}

// KaspiWebhook handles POST /api/v1/webhooks/kaspi
//
// Processing steps:
//  1. Read raw body (needed for HMAC verification)
//  2. Verify HMAC-SHA256 signature from X-Kaspi-Signature header
//  3. Parse event and update payment/invoice status atomically in the service
//  4. Publish payment.completed Kafka event
//  5. Return 200 OK immediately (Kaspi expects a fast response)
func (h *WebhookHandler) KaspiWebhook(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context())

	payload, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MB limit
	if err != nil {
		log.Error().Err(err).Msg("kaspi webhook: read body")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	signature := r.Header.Get("X-Kaspi-Signature")

	if err := h.svc.ProcessWebhook(r.Context(), domain.ProviderKaspi, payload, signature); err != nil {
		log.Error().Err(err).Msg("kaspi webhook: process")
		if err == domain.ErrInvalidSignature {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// For all other errors return 200 to prevent Kaspi from retrying malformed events
		w.WriteHeader(http.StatusOK)
		return
	}

	log.Info().Msg("kaspi webhook: processed successfully")
	w.WriteHeader(http.StatusOK)
}

// StripeWebhook handles POST /api/v1/webhooks/stripe
//
// Processing steps:
//  1. Read raw body (required for signature verification)
//  2. Verify signature via Stripe-Signature header (HMAC-SHA256 with timestamp)
//  3. Handle events: checkout.session.completed, payment_intent.payment_failed
//  4. Idempotent — duplicate webhooks are safe to process
func (h *WebhookHandler) StripeWebhook(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context())

	payload, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		log.Error().Err(err).Msg("stripe webhook: read body")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	signature := r.Header.Get("Stripe-Signature")

	if err := h.svc.ProcessWebhook(r.Context(), domain.ProviderStripe, payload, signature); err != nil {
		log.Error().Err(err).Msg("stripe webhook: process")
		if err == domain.ErrInvalidSignature {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// Return 200 for unrecognised event types to stop Stripe retrying
		w.WriteHeader(http.StatusOK)
		return
	}

	log.Info().Msg("stripe webhook: processed successfully")
	w.WriteHeader(http.StatusOK)
}
