package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nurtidev/medcore/internal/billing/domain"
	"github.com/nurtidev/medcore/internal/billing/repository"
)

func (s *billingServiceImpl) CreateInvoice(ctx context.Context, req CreateInvoiceRequest) (*domain.Invoice, error) {
	inv := &domain.Invoice{
		ClinicID:    req.ClinicID,
		ServiceName: req.ServiceName,
		Amount:      req.Amount,
		Currency:    req.Currency,
		Status:      domain.InvoiceStatusDraft,
		DueAt:       req.DueAt,
	}
	if req.PatientID != nil {
		inv.PatientID = *req.PatientID
	}

	if err := s.invoiceRepo.Create(ctx, inv); err != nil {
		return nil, fmt.Errorf("billing.CreateInvoice: %w", err)
	}
	return inv, nil
}

func (s *billingServiceImpl) GetInvoice(ctx context.Context, invoiceID uuid.UUID) (*domain.Invoice, error) {
	inv, err := s.invoiceRepo.GetByID(ctx, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("billing.GetInvoice: %w", err)
	}
	return inv, nil
}

func (s *billingServiceImpl) ListInvoices(
	ctx context.Context,
	clinicID uuid.UUID,
	filter repository.InvoiceFilter,
) ([]*domain.Invoice, error) {
	invoices, err := s.invoiceRepo.List(ctx, clinicID, filter)
	if err != nil {
		return nil, fmt.Errorf("billing.ListInvoices: %w", err)
	}
	return invoices, nil
}

// MarkOverdueInvoices is called by the daily CRON job.
func (s *billingServiceImpl) MarkOverdueInvoices(ctx context.Context) error {
	n, err := s.invoiceRepo.MarkOverdue(ctx)
	if err != nil {
		return fmt.Errorf("billing.MarkOverdueInvoices: %w", err)
	}
	if n > 0 {
		_ = n // could log or emit a metric here
	}
	return nil
}
