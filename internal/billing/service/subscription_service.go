package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nurtidev/medcore/internal/billing/domain"
)

func (s *billingServiceImpl) GetSubscription(ctx context.Context, clinicID uuid.UUID) (*domain.Subscription, error) {
	sub, err := s.subRepo.GetByClinicID(ctx, clinicID)
	if err != nil {
		return nil, fmt.Errorf("billing.GetSubscription: %w", err)
	}
	return sub, nil
}

func (s *billingServiceImpl) CreateSubscription(ctx context.Context, clinicID uuid.UUID, planID uuid.UUID) (*domain.Subscription, error) {
	// Validate plan exists
	if _, err := s.subRepo.GetPlanByID(ctx, planID); err != nil {
		return nil, fmt.Errorf("billing.CreateSubscription: invalid plan: %w", err)
	}

	// Cancel any existing active subscription
	existing, err := s.subRepo.GetByClinicID(ctx, clinicID)
	if err == nil && existing != nil && existing.Status == domain.SubStatusActive {
		now := time.Now()
		existing.Status = domain.SubStatusCancelled
		existing.CancelledAt = &now
		if err := s.subRepo.Update(ctx, existing); err != nil {
			return nil, fmt.Errorf("billing.CreateSubscription: cancel existing: %w", err)
		}
		s.metrics.subscriptionActive.Dec()
	}

	now := time.Now()
	sub := &domain.Subscription{
		ID:                 uuid.New(),
		ClinicID:           clinicID,
		PlanID:             planID,
		Status:             domain.SubStatusActive,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0), // +30 days
	}

	if err := s.subRepo.Create(ctx, sub); err != nil {
		return nil, fmt.Errorf("billing.CreateSubscription: %w", err)
	}
	s.metrics.subscriptionActive.Inc()
	return sub, nil
}

func (s *billingServiceImpl) CancelSubscription(ctx context.Context, clinicID uuid.UUID) error {
	sub, err := s.subRepo.GetByClinicID(ctx, clinicID)
	if err != nil {
		return fmt.Errorf("billing.CancelSubscription: %w", err)
	}
	if sub.Status == domain.SubStatusCancelled {
		return nil // already cancelled — idempotent
	}

	now := time.Now()
	sub.Status = domain.SubStatusCancelled
	sub.CancelledAt = &now
	if err := s.subRepo.Update(ctx, sub); err != nil {
		return fmt.Errorf("billing.CancelSubscription: update: %w", err)
	}
	s.metrics.subscriptionActive.Dec()
	return nil
}

func (s *billingServiceImpl) CheckSubscriptionAccess(ctx context.Context, clinicID uuid.UUID) (bool, error) {
	sub, err := s.subRepo.GetByClinicID(ctx, clinicID)
	if err != nil {
		if err == domain.ErrSubscriptionNotFound {
			return false, nil
		}
		return false, fmt.Errorf("billing.CheckSubscriptionAccess: %w", err)
	}
	active := sub.Status == domain.SubStatusActive && time.Now().Before(sub.CurrentPeriodEnd)
	return active, nil
}

// ProcessExpiredSubscriptions is called every 5 minutes by the CRON job.
// It marks active-but-overdue subscriptions as past_due and emits Kafka events.
func (s *billingServiceImpl) ProcessExpiredSubscriptions(ctx context.Context) error {
	expired, err := s.subRepo.GetExpired(ctx)
	if err != nil {
		return fmt.Errorf("billing.ProcessExpiredSubscriptions: fetch: %w", err)
	}

	for _, sub := range expired {
		sub.Status = domain.SubStatusPastDue
		if err := s.subRepo.Update(ctx, sub); err != nil {
			// log and continue; don't abort the whole batch
			continue
		}
		s.metrics.subscriptionActive.Dec()

		evt := SubscriptionExpiredEvent{
			SubscriptionID: sub.ID.String(),
			ClinicID:       sub.ClinicID.String(),
			ExpiredAt:      sub.CurrentPeriodEnd,
		}
		if s.kafka != nil {
			_ = s.kafka.Publish(ctx, s.cfg.SubscriptionExpiredTopic, sub.ClinicID.String(), evt)
		}
	}
	return nil
}
