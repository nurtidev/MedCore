package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/nurtidev/medcore/internal/billing/domain"
)

type InvoiceFilter struct {
	Status    *domain.InvoiceStatus
	PatientID *uuid.UUID
	Limit     int
	Offset    int
}

type InvoiceRepository interface {
	Create(ctx context.Context, invoice *domain.Invoice) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Invoice, error)
	List(ctx context.Context, clinicID uuid.UUID, filter InvoiceFilter) ([]*domain.Invoice, error)
	Update(ctx context.Context, invoice *domain.Invoice) error
	// MarkOverdue sets status=overdue for all invoices where due_at < NOW() and status != paid/voided.
	MarkOverdue(ctx context.Context) (int64, error)
}
