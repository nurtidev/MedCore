// Package kaspi implements the Kaspi Pay payment provider.
// API reference: Kaspi Pay Merchant API (private docs).
package kaspi

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
	"time"

	"github.com/nurtidev/medcore/internal/billing/provider"
	"github.com/shopspring/decimal"
)

const providerName = "kaspi"

type Client struct {
	apiURL     string
	merchantID string
	secretKey  string
	http       *http.Client
}

func New(apiURL, merchantID, secretKey string) *Client {
	return &Client{
		apiURL:     apiURL,
		merchantID: merchantID,
		secretKey:  secretKey,
		http:       &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) Name() string { return providerName }

// CreatePaymentLink calls Kaspi Pay API to generate a hosted payment page URL.
func (c *Client) CreatePaymentLink(ctx context.Context, req provider.PaymentLinkRequest) (string, error) {
	body := map[string]any{
		"merchantId":  c.merchantID,
		"orderId":     req.IdempotencyKey,
		"amount":      req.Amount.String(),
		"currency":    req.Currency,
		"description": req.Description,
		"returnUrl":   req.ReturnURL,
		"paymentId":   req.PaymentID.String(),
	}

	respBody, err := c.post(ctx, "/payment/create", body)
	if err != nil {
		return "", fmt.Errorf("kaspi.CreatePaymentLink: %w", err)
	}

	var resp struct {
		PaymentURL string `json:"paymentUrl"`
		Error      string `json:"error"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("kaspi.CreatePaymentLink: decode response: %w", err)
	}
	if resp.Error != "" {
		return "", fmt.Errorf("kaspi.CreatePaymentLink: provider error: %s", resp.Error)
	}
	return resp.PaymentURL, nil
}

// VerifyWebhookSignature checks the HMAC-SHA256 signature from the X-Kaspi-Signature header.
func (c *Client) VerifyWebhookSignature(payload []byte, signature string) bool {
	mac := hmac.New(sha256.New, []byte(c.secretKey))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

// ParseWebhookEvent parses a Kaspi Pay webhook callback into a normalised event.
func (c *Client) ParseWebhookEvent(payload []byte) (*provider.WebhookEvent, error) {
	var raw struct {
		TransactionID string `json:"transactionId"`
		OrderID       string `json:"orderId"`
		Amount        string `json:"amount"`
		Currency      string `json:"currency"`
		Status        string `json:"status"` // "SUCCESS" | "FAIL"
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, fmt.Errorf("kaspi.ParseWebhookEvent: decode: %w", err)
	}

	amount, _ := decimal.NewFromString(raw.Amount)

	eventType := "payment.failed"
	status := "failed"
	if raw.Status == "SUCCESS" {
		eventType = "payment.completed"
		status = "completed"
	}

	var rawMap map[string]any
	_ = json.Unmarshal(payload, &rawMap)

	return &provider.WebhookEvent{
		Type:       eventType,
		ExternalID: raw.TransactionID,
		Amount:     amount,
		Currency:   raw.Currency,
		Status:     status,
		RawPayload: rawMap,
		OccurredAt: time.Now(),
	}, nil
}

// RefundPayment requests a refund from Kaspi Pay.
func (c *Client) RefundPayment(ctx context.Context, externalID string, amount decimal.Decimal) error {
	body := map[string]any{
		"transactionId": externalID,
		"amount":        amount.String(),
	}
	if _, err := c.post(ctx, "/payment/refund", body); err != nil {
		return fmt.Errorf("kaspi.RefundPayment: %w", err)
	}
	return nil
}

func (c *Client) post(ctx context.Context, path string, body any) ([]byte, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL+path, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Merchant-ID", c.merchantID)

	// Sign request body
	mac := hmac.New(sha256.New, []byte(c.secretKey))
	mac.Write(payload)
	req.Header.Set("X-Signature", hex.EncodeToString(mac.Sum(nil)))

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("kaspi API error %d: %s", resp.StatusCode, string(respBody))
	}
	return respBody, nil
}
