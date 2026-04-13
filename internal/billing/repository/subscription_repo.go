package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/nurtidev/medcore/internal/billing/domain"
)

type SubscriptionRepository interface {
	Create(ctx context.Context, sub *domain.Subscription) error
	GetByClinicID(ctx context.Context, clinicID uuid.UUID) (*domain.Subscription, error)
	// GetExpired returns active subscriptions whose current_period_end < NOW().
	GetExpired(ctx context.Context) ([]*domain.Subscription, error)
	Update(ctx context.Context, sub *domain.Subscription) error
	GetPlanByID(ctx context.Context, planID uuid.UUID) (*domain.Plan, error)
	ListPlans(ctx context.Context) ([]*domain.Plan, error)
}
