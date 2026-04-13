package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type PaymentProvider string

const (
	ProviderKaspi  PaymentProvider = "kaspi"
	ProviderStripe PaymentProvider = "stripe"
)

type PaymentStatus string

const (
	PaymentStatusPending    PaymentStatus = "pending"
	PaymentStatusProcessing PaymentStatus = "processing"
	PaymentStatusCompleted  PaymentStatus = "completed"
	PaymentStatusFailed     PaymentStatus = "failed"
	PaymentStatusRefunded   PaymentStatus = "refunded"
)

type Payment struct {
	ID             uuid.UUID
	InvoiceID      uuid.UUID
	ClinicID       uuid.UUID
	PatientID      uuid.UUID
	IdempotencyKey string
	Provider       PaymentProvider
	ExternalID     string
	Amount         decimal.Decimal
	Currency       string
	Status         PaymentStatus
	FailureReason  string
	Metadata       map[string]any
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
