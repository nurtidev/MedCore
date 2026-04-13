package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nurtidev/medcore/internal/billing/domain"
	"github.com/shopspring/decimal"
)

type postgresPaymentRepo struct {
	db *pgxpool.Pool
}

func NewPostgresPaymentRepo(db *pgxpool.Pool) PaymentRepository {
	return &postgresPaymentRepo{db: db}
}

func (r *postgresPaymentRepo) Create(ctx context.Context, p *domain.Payment) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	now := time.Now()
	p.CreatedAt = now
	p.UpdatedAt = now

	metaJSON, err := json.Marshal(p.Metadata)
	if err != nil {
		return fmt.Errorf("paymentRepo.Create: marshal metadata: %w", err)
	}

	_, err = r.db.Exec(ctx, `
		INSERT INTO payments
			(id, invoice_id, clinic_id, patient_id, idempotency_key, provider, external_id,
			 amount, currency, status, failure_reason, metadata, created_at, updated_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		p.ID, p.InvoiceID, p.ClinicID, p.PatientID, p.IdempotencyKey,
		string(p.Provider), p.ExternalID,
		p.Amount.String(), p.Currency, string(p.Status),
		p.FailureReason, metaJSON,
		p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("paymentRepo.Create: %w", err)
	}
	return nil
}

func (r *postgresPaymentRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Payment, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, invoice_id, clinic_id, patient_id, idempotency_key, provider, external_id,
		       amount, currency, status, failure_reason, metadata, created_at, updated_at
		FROM payments WHERE id = $1`, id)

	p, err := scanPayment(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrPaymentNotFound
		}
		return nil, fmt.Errorf("paymentRepo.GetByID: %w", err)
	}
	return p, nil
}

func (r *postgresPaymentRepo) GetByIdempotencyKey(ctx context.Context, key string) (*domain.Payment, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, invoice_id, clinic_id, patient_id, idempotency_key, provider, external_id,
		       amount, currency, status, failure_reason, metadata, created_at, updated_at
		FROM payments WHERE idempotency_key = $1`, key)

	p, err := scanPayment(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrPaymentNotFound
		}
		return nil, fmt.Errorf("paymentRepo.GetByIdempotencyKey: %w", err)
	}
	return p, nil
}

func (r *postgresPaymentRepo) GetByExternalID(ctx context.Context, externalID string) (*domain.Payment, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, invoice_id, clinic_id, patient_id, idempotency_key, provider, external_id,
		       amount, currency, status, failure_reason, metadata, created_at, updated_at
		FROM payments WHERE external_id = $1`, externalID)

	p, err := scanPayment(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrPaymentNotFound
		}
		return nil, fmt.Errorf("paymentRepo.GetByExternalID: %w", err)
	}
	return p, nil
}

func (r *postgresPaymentRepo) Update(ctx context.Context, p *domain.Payment) error {
	p.UpdatedAt = time.Now()
	metaJSON, _ := json.Marshal(p.Metadata)

	_, err := r.db.Exec(ctx, `
		UPDATE payments SET
			external_id = $2, status = $3, failure_reason = $4,
			metadata = $5, updated_at = $6
		WHERE id = $1`,
		p.ID, p.ExternalID, string(p.Status),
		p.FailureReason, metaJSON, p.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("paymentRepo.Update: %w", err)
	}
	return nil
}

func scanPayment(row interface {
	Scan(dest ...any) error
}) (*domain.Payment, error) {
	var p domain.Payment
	var amountStr, provider, status string
	var metaJSON []byte

	err := row.Scan(
		&p.ID, &p.InvoiceID, &p.ClinicID, &p.PatientID,
		&p.IdempotencyKey, &provider, &p.ExternalID,
		&amountStr, &p.Currency, &status,
		&p.FailureReason, &metaJSON,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	p.Amount, err = decimal.NewFromString(amountStr)
	if err != nil {
		return nil, fmt.Errorf("parse amount: %w", err)
	}
	p.Provider = domain.PaymentProvider(provider)
	p.Status = domain.PaymentStatus(status)

	if len(metaJSON) > 0 {
		if err := json.Unmarshal(metaJSON, &p.Metadata); err != nil {
			return nil, fmt.Errorf("parse metadata: %w", err)
		}
	}
	return &p, nil
}
