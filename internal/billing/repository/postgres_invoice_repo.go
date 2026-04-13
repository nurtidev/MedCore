package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nurtidev/medcore/internal/billing/domain"
	"github.com/shopspring/decimal"
)

type postgresInvoiceRepo struct {
	db *pgxpool.Pool
}

func NewPostgresInvoiceRepo(db *pgxpool.Pool) InvoiceRepository {
	return &postgresInvoiceRepo{db: db}
}

func (r *postgresInvoiceRepo) Create(ctx context.Context, inv *domain.Invoice) error {
	inv.ID = uuid.New()
	now := time.Now()
	inv.CreatedAt = now
	inv.UpdatedAt = now

	_, err := r.db.Exec(ctx, `
		INSERT INTO invoices
			(id, clinic_id, patient_id, service_name, amount, currency, status, due_at, paid_at, pdf_url, created_at, updated_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		inv.ID, inv.ClinicID, inv.PatientID, inv.ServiceName,
		inv.Amount.String(), inv.Currency, string(inv.Status),
		inv.DueAt, inv.PaidAt, inv.PDFUrl,
		inv.CreatedAt, inv.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("invoiceRepo.Create: %w", err)
	}
	return nil
}

func (r *postgresInvoiceRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Invoice, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, clinic_id, patient_id, service_name, amount, currency, status,
		       due_at, paid_at, pdf_url, created_at, updated_at
		FROM invoices WHERE id = $1`, id)

	inv, err := scanInvoice(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrInvoiceNotFound
		}
		return nil, fmt.Errorf("invoiceRepo.GetByID: %w", err)
	}
	return inv, nil
}

func (r *postgresInvoiceRepo) List(ctx context.Context, clinicID uuid.UUID, filter InvoiceFilter) ([]*domain.Invoice, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := filter.Offset

	args := []any{clinicID}
	query := `
		SELECT id, clinic_id, patient_id, service_name, amount, currency, status,
		       due_at, paid_at, pdf_url, created_at, updated_at
		FROM invoices WHERE clinic_id = $1`

	if filter.Status != nil {
		args = append(args, string(*filter.Status))
		query += fmt.Sprintf(" AND status = $%d", len(args))
	}
	if filter.PatientID != nil {
		args = append(args, *filter.PatientID)
		query += fmt.Sprintf(" AND patient_id = $%d", len(args))
	}

	args = append(args, limit, offset)
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("invoiceRepo.List: %w", err)
	}
	defer rows.Close()

	var invoices []*domain.Invoice
	for rows.Next() {
		inv, err := scanInvoice(rows)
		if err != nil {
			return nil, fmt.Errorf("invoiceRepo.List: scan: %w", err)
		}
		invoices = append(invoices, inv)
	}
	return invoices, rows.Err()
}

func (r *postgresInvoiceRepo) Update(ctx context.Context, inv *domain.Invoice) error {
	inv.UpdatedAt = time.Now()
	_, err := r.db.Exec(ctx, `
		UPDATE invoices SET
			service_name = $2, amount = $3, currency = $4, status = $5,
			due_at = $6, paid_at = $7, pdf_url = $8, updated_at = $9
		WHERE id = $1`,
		inv.ID, inv.ServiceName, inv.Amount.String(), inv.Currency, string(inv.Status),
		inv.DueAt, inv.PaidAt, inv.PDFUrl, inv.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("invoiceRepo.Update: %w", err)
	}
	return nil
}

func (r *postgresInvoiceRepo) MarkOverdue(ctx context.Context) (int64, error) {
	tag, err := r.db.Exec(ctx, `
		UPDATE invoices SET status = 'overdue', updated_at = NOW()
		WHERE due_at < NOW()
		  AND status NOT IN ('paid', 'voided', 'overdue')`)
	if err != nil {
		return 0, fmt.Errorf("invoiceRepo.MarkOverdue: %w", err)
	}
	return tag.RowsAffected(), nil
}

// scanInvoice scans a row into an Invoice. Works for both pgx.Row and pgx.Rows.
func scanInvoice(row interface {
	Scan(dest ...any) error
}) (*domain.Invoice, error) {
	var inv domain.Invoice
	var amountStr string
	var status string

	err := row.Scan(
		&inv.ID, &inv.ClinicID, &inv.PatientID, &inv.ServiceName,
		&amountStr, &inv.Currency, &status,
		&inv.DueAt, &inv.PaidAt, &inv.PDFUrl,
		&inv.CreatedAt, &inv.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	inv.Amount, err = decimal.NewFromString(amountStr)
	if err != nil {
		return nil, fmt.Errorf("parse amount: %w", err)
	}
	inv.Status = domain.InvoiceStatus(status)
	return &inv, nil
}
