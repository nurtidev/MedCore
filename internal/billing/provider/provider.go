package provider

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// PaymentLinkRequest contains all data needed to create a hosted payment link.
type PaymentLinkRequest struct {
	PaymentID      uuid.UUID
	InvoiceID      uuid.UUID
	Amount         decimal.Decimal
	Currency       string
	IdempotencyKey string
	ReturnURL      string
	Description    string
}

// WebhookEvent is the normalised event parsed from a provider webhook payload.
type WebhookEvent struct {
	Type       string         // "payment.completed" | "payment.failed"
	ExternalID string         // provider-side transaction ID
	Amount     decimal.Decimal
	Currency   string
	Status     string
	RawPayload map[string]any
	OccurredAt time.Time
}

// PaymentProvider is the interface every payment gateway must implement.
type PaymentProvider interface {
	// Name returns the provider identifier (e.g. "kaspi", "stripe").
	Name() string
	// CreatePaymentLink creates a hosted checkout URL and returns it.
	CreatePaymentLink(ctx context.Context, req PaymentLinkRequest) (string, error)
	// VerifyWebhookSignature validates the HMAC/signature of an incoming webhook.
	VerifyWebhookSignature(payload []byte, signature string) bool
	// ParseWebhookEvent parses the raw webhook body into a normalised WebhookEvent.
	ParseWebhookEvent(payload []byte) (*WebhookEvent, error)
	// RefundPayment issues a full or partial refund for an existing transaction.
	RefundPayment(ctx context.Context, externalID string, amount decimal.Decimal) error
}
