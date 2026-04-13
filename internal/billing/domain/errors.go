package domain

import "errors"

var (
	ErrInvoiceNotFound      = errors.New("invoice not found")
	ErrPaymentNotFound      = errors.New("payment not found")
	ErrPaymentDuplicate     = errors.New("payment with this idempotency key already exists")
	ErrSubscriptionNotFound = errors.New("subscription not found")
	ErrSubscriptionExpired  = errors.New("subscription expired")
	ErrSubscriptionInactive = errors.New("subscription inactive")
	ErrPlanNotFound         = errors.New("plan not found")
	ErrInvalidSignature     = errors.New("invalid webhook signature")
	ErrUnknownProvider      = errors.New("unknown payment provider")
	ErrPDFUnavailable       = errors.New("pdf service unavailable")
)
