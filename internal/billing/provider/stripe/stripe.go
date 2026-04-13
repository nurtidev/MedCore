// Package stripe implements the Stripe payment provider.
// Uses Stripe Checkout Sessions API. Webhook verification is done manually
// (no stripe-go SDK dependency) using Stripe's documented HMAC-SHA256 scheme.
package stripe

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nurtidev/medcore/internal/billing/provider"
	"github.com/shopspring/decimal"
)

const (
	providerName = "stripe"
	stripeAPIURL = "https://api.stripe.com/v1"
)

type Client struct {
	secretKey     string
	webhookSecret string
	http          *http.Client
}

func New(secretKey, webhookSecret string) *Client {
	return &Client{
		secretKey:     secretKey,
		webhookSecret: webhookSecret,
		http:          &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) Name() string { return providerName }

// CreatePaymentLink creates a Stripe Checkout Session and returns the hosted URL.
func (c *Client) CreatePaymentLink(ctx context.Context, req provider.PaymentLinkRequest) (string, error) {
	amountCents := req.Amount.Mul(decimal.NewFromInt(100)).IntPart()

	params := url.Values{}
	params.Set("payment_method_types[]", "card")
	params.Set("mode", "payment")
	params.Set("success_url", req.ReturnURL+"?status=success&session_id={CHECKOUT_SESSION_ID}")
	params.Set("cancel_url", req.ReturnURL+"?status=cancel")
	params.Set("line_items[0][price_data][currency]", strings.ToLower(req.Currency))
	params.Set("line_items[0][price_data][unit_amount]", fmt.Sprintf("%d", amountCents))
	params.Set("line_items[0][price_data][product_data][name]", req.Description)
	params.Set("line_items[0][quantity]", "1")
	params.Set("client_reference_id", req.IdempotencyKey)
	params.Set("metadata[payment_id]", req.PaymentID.String())
	params.Set("metadata[invoice_id]", req.InvoiceID.String())

	body, err := c.post(ctx, "/checkout/sessions", params.Encode())
	if err != nil {
		return "", fmt.Errorf("stripe.CreatePaymentLink: %w", err)
	}

	var resp struct {
		URL   string `json:"url"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("stripe.CreatePaymentLink: decode: %w", err)
	}
	if resp.Error != nil {
		return "", fmt.Errorf("stripe.CreatePaymentLink: api error: %s", resp.Error.Message)
	}
	return resp.URL, nil
}

// VerifyWebhookSignature verifies the Stripe-Signature header.
// Format: t=timestamp,v1=sig1,v1=sig2
func (c *Client) VerifyWebhookSignature(payload []byte, signature string) bool {
	timestamp, v1Sigs := parseStripeSignatureHeader(signature)
	if timestamp == "" {
		return false
	}

	signedPayload := timestamp + "." + string(payload)
	mac := hmac.New(sha256.New, []byte(c.webhookSecret))
	mac.Write([]byte(signedPayload))
	expected := hex.EncodeToString(mac.Sum(nil))

	for _, sig := range v1Sigs {
		if hmac.Equal([]byte(expected), []byte(sig)) {
			return true
		}
	}
	return false
}

// ParseWebhookEvent parses a Stripe webhook event payload.
// Handles: checkout.session.completed, payment_intent.payment_failed
func (c *Client) ParseWebhookEvent(payload []byte) (*provider.WebhookEvent, error) {
	var raw struct {
		Type string `json:"type"`
		Data struct {
			Object struct {
				ID             string         `json:"id"`
				PaymentIntent  string         `json:"payment_intent"`
				AmountTotal    int64          `json:"amount_total"`    // cents
				Currency       string         `json:"currency"`
				PaymentStatus  string         `json:"payment_status"`
				ClientReference string        `json:"client_reference_id"`
				Metadata       map[string]any `json:"metadata"`
			} `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, fmt.Errorf("stripe.ParseWebhookEvent: decode: %w", err)
	}

	obj := raw.Data.Object
	amount := decimal.NewFromInt(obj.AmountTotal).Div(decimal.NewFromInt(100))

	var rawMap map[string]any
	_ = json.Unmarshal(payload, &rawMap)

	event := &provider.WebhookEvent{
		ExternalID: obj.PaymentIntent,
		Amount:     amount,
		Currency:   strings.ToUpper(obj.Currency),
		RawPayload: rawMap,
		OccurredAt: time.Now(),
	}

	switch raw.Type {
	case "checkout.session.completed":
		event.Type = "payment.completed"
		event.Status = "completed"
	case "payment_intent.payment_failed":
		event.Type = "payment.failed"
		event.Status = "failed"
	default:
		// Unhandled event type — treat as no-op
		event.Type = raw.Type
		event.Status = "unknown"
	}

	return event, nil
}

// RefundPayment creates a Stripe Refund for the given PaymentIntent.
func (c *Client) RefundPayment(ctx context.Context, externalID string, amount decimal.Decimal) error {
	amountCents := amount.Mul(decimal.NewFromInt(100)).IntPart()

	params := url.Values{}
	params.Set("payment_intent", externalID)
	params.Set("amount", fmt.Sprintf("%d", amountCents))

	if _, err := c.post(ctx, "/refunds", params.Encode()); err != nil {
		return fmt.Errorf("stripe.RefundPayment: %w", err)
	}
	return nil
}

func (c *Client) post(ctx context.Context, path, formBody string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		stripeAPIURL+path, bytes.NewBufferString(formBody))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(c.secretKey, "")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("stripe API error %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// parseStripeSignatureHeader parses "t=...,v1=...,v1=..." into timestamp and v1 signatures.
func parseStripeSignatureHeader(header string) (timestamp string, v1Sigs []string) {
	parts := strings.Split(header, ",")
	for _, part := range parts {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "t":
			timestamp = kv[1]
		case "v1":
			v1Sigs = append(v1Sigs, kv[1])
		}
	}
	return
}
