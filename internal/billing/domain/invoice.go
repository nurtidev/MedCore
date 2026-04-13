package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type InvoiceStatus string

const (
	InvoiceStatusDraft   InvoiceStatus = "draft"
	InvoiceStatusSent    InvoiceStatus = "sent"
	InvoiceStatusPaid    InvoiceStatus = "paid"
	InvoiceStatusOverdue InvoiceStatus = "overdue"
	InvoiceStatusVoided  InvoiceStatus = "voided"
)

type Invoice struct {
	ID          uuid.UUID
	ClinicID    uuid.UUID
	PatientID   uuid.UUID
	ServiceName string
	Amount      decimal.Decimal
	Currency    string
	Status      InvoiceStatus
	DueAt       time.Time
	PaidAt      *time.Time
	PDFUrl      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
