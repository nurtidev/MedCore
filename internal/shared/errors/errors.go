package errors

import (
	"errors"
	"fmt"
)

// Sentinel ошибки — используются во всех сервисах платформы.
var (
	// Auth
	ErrUnauthorized    = errors.New("unauthorized")
	ErrForbidden       = errors.New("forbidden")
	ErrTokenExpired    = errors.New("token expired")
	ErrTokenInvalid    = errors.New("token invalid")
	ErrUserNotFound    = errors.New("user not found")
	ErrUserInactive    = errors.New("user inactive")
	ErrUserExists      = errors.New("user already exists")
	ErrInvalidPassword = errors.New("invalid password")

	// Billing
	ErrInvoiceNotFound      = errors.New("invoice not found")
	ErrPaymentNotFound      = errors.New("payment not found")
	ErrPaymentAlreadyExists = errors.New("payment with this idempotency key already exists")
	ErrSubscriptionNotFound = errors.New("subscription not found")
	ErrSubscriptionExpired  = errors.New("subscription expired")
	ErrSubscriptionInactive = errors.New("subscription inactive")

	// Integration
	ErrIntegrationNotConfigured = errors.New("integration not configured")
	ErrExternalServiceUnavailable = errors.New("external service unavailable")
	ErrCircuitBreakerOpen       = errors.New("circuit breaker is open")
	ErrIINInvalid               = errors.New("invalid IIN")
	ErrLabResultNotFound        = errors.New("lab result not found")

	// General
	ErrNotFound        = errors.New("not found")
	ErrInvalidInput    = errors.New("invalid input")
	ErrInternal        = errors.New("internal error")
	ErrConflict        = errors.New("conflict")
)

// DomainError — ошибка с кодом и деталями для HTTP ответа.
type DomainError struct {
	Code    string
	Message string
	Err     error
}

func (e *DomainError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *DomainError) Unwrap() error {
	return e.Err
}

// New создаёт DomainError.
func New(code, message string, err error) *DomainError {
	return &DomainError{Code: code, Message: message, Err: err}
}

// Is проверяет sentinel ошибку через цепочку Unwrap.
func Is(err, target error) bool {
	return errors.Is(err, target)
}

// As приводит ошибку к нужному типу.
func As[T error](err error) (T, bool) {
	var target T
	ok := errors.As(err, &target)
	return target, ok
}
