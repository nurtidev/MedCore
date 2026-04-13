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

type postgresSubscriptionRepo struct {
	db *pgxpool.Pool
}

func NewPostgresSubscriptionRepo(db *pgxpool.Pool) SubscriptionRepository {
	return &postgresSubscriptionRepo{db: db}
}

func (r *postgresSubscriptionRepo) Create(ctx context.Context, sub *domain.Subscription) error {
	if sub.ID == uuid.Nil {
		sub.ID = uuid.New()
	}
	now := time.Now()
	sub.CreatedAt = now
	sub.UpdatedAt = now

	_, err := r.db.Exec(ctx, `
		INSERT INTO subscriptions
			(id, clinic_id, plan_id, status, current_period_start, current_period_end, cancelled_at, created_at, updated_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		sub.ID, sub.ClinicID, sub.PlanID, string(sub.Status),
		sub.CurrentPeriodStart, sub.CurrentPeriodEnd, sub.CancelledAt,
		sub.CreatedAt, sub.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("subscriptionRepo.Create: %w", err)
	}
	return nil
}

func (r *postgresSubscriptionRepo) GetByClinicID(ctx context.Context, clinicID uuid.UUID) (*domain.Subscription, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, clinic_id, plan_id, status, current_period_start, current_period_end,
		       cancelled_at, created_at, updated_at
		FROM subscriptions
		WHERE clinic_id = $1
		ORDER BY created_at DESC LIMIT 1`, clinicID)

	sub, err := scanSubscription(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrSubscriptionNotFound
		}
		return nil, fmt.Errorf("subscriptionRepo.GetByClinicID: %w", err)
	}
	return sub, nil
}

func (r *postgresSubscriptionRepo) GetExpired(ctx context.Context) ([]*domain.Subscription, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, clinic_id, plan_id, status, current_period_start, current_period_end,
		       cancelled_at, created_at, updated_at
		FROM subscriptions
		WHERE current_period_end < NOW()
		  AND status = 'active'`)
	if err != nil {
		return nil, fmt.Errorf("subscriptionRepo.GetExpired: %w", err)
	}
	defer rows.Close()

	var subs []*domain.Subscription
	for rows.Next() {
		sub, err := scanSubscription(rows)
		if err != nil {
			return nil, fmt.Errorf("subscriptionRepo.GetExpired: scan: %w", err)
		}
		subs = append(subs, sub)
	}
	return subs, rows.Err()
}

func (r *postgresSubscriptionRepo) Update(ctx context.Context, sub *domain.Subscription) error {
	sub.UpdatedAt = time.Now()
	_, err := r.db.Exec(ctx, `
		UPDATE subscriptions SET
			plan_id = $2, status = $3,
			current_period_start = $4, current_period_end = $5,
			cancelled_at = $6, updated_at = $7
		WHERE id = $1`,
		sub.ID, sub.PlanID, string(sub.Status),
		sub.CurrentPeriodStart, sub.CurrentPeriodEnd,
		sub.CancelledAt, sub.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("subscriptionRepo.Update: %w", err)
	}
	return nil
}

func (r *postgresSubscriptionRepo) GetPlanByID(ctx context.Context, planID uuid.UUID) (*domain.Plan, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, tier, name, price_monthly, currency, max_doctors, max_patients, features
		FROM subscription_plans WHERE id = $1 AND is_active = true`, planID)

	plan, err := scanPlan(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrPlanNotFound
		}
		return nil, fmt.Errorf("subscriptionRepo.GetPlanByID: %w", err)
	}
	return plan, nil
}

func (r *postgresSubscriptionRepo) ListPlans(ctx context.Context) ([]*domain.Plan, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, tier, name, price_monthly, currency, max_doctors, max_patients, features
		FROM subscription_plans WHERE is_active = true
		ORDER BY price_monthly ASC`)
	if err != nil {
		return nil, fmt.Errorf("subscriptionRepo.ListPlans: %w", err)
	}
	defer rows.Close()

	var plans []*domain.Plan
	for rows.Next() {
		plan, err := scanPlan(rows)
		if err != nil {
			return nil, fmt.Errorf("subscriptionRepo.ListPlans: scan: %w", err)
		}
		plans = append(plans, plan)
	}
	return plans, rows.Err()
}

func scanSubscription(row interface {
	Scan(dest ...any) error
}) (*domain.Subscription, error) {
	var sub domain.Subscription
	var status string

	err := row.Scan(
		&sub.ID, &sub.ClinicID, &sub.PlanID, &status,
		&sub.CurrentPeriodStart, &sub.CurrentPeriodEnd,
		&sub.CancelledAt, &sub.CreatedAt, &sub.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	sub.Status = domain.SubscriptionStatus(status)
	return &sub, nil
}

func scanPlan(row interface {
	Scan(dest ...any) error
}) (*domain.Plan, error) {
	var p domain.Plan
	var priceStr, tier string
	var featuresJSON []byte

	err := row.Scan(
		&p.ID, &tier, &p.Name, &priceStr, &p.Currency,
		&p.MaxDoctors, &p.MaxPatients, &featuresJSON,
	)
	if err != nil {
		return nil, err
	}

	p.Tier = domain.PlanTier(tier)
	p.PriceMonthly, err = decimal.NewFromString(priceStr)
	if err != nil {
		return nil, fmt.Errorf("parse price: %w", err)
	}
	if len(featuresJSON) > 0 {
		if err := json.Unmarshal(featuresJSON, &p.Features); err != nil {
			return nil, fmt.Errorf("parse features: %w", err)
		}
	}
	return &p, nil
}
