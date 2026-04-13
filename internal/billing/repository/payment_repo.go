package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/nurtidev/medcore/internal/billing/domain"
)

type PaymentRepository interface {
	Create(ctx context.Context, payment *domain.Payment) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Payment, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*domain.Payment, error)
	GetByExternalID(ctx context.Context, externalID string) (*domain.Payment, error)
	Update(ctx context.Context, payment *domain.Payment) error
}
