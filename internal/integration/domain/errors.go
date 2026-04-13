package domain

import "errors"

var (
	ErrIntegrationNotConfigured   = errors.New("integration not configured")
	ErrExternalServiceUnavailable = errors.New("external service unavailable")
	ErrCircuitBreakerOpen         = errors.New("circuit breaker is open")
	ErrIINInvalid                 = errors.New("invalid IIN")
	ErrLabResultNotFound          = errors.New("lab result not found")
	ErrInvalidSignature           = errors.New("invalid webhook signature")
	ErrNotFound                   = errors.New("not found")
	ErrInvalidInput               = errors.New("invalid input")
)
