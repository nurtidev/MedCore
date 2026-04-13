package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type PlanTier string

const (
	PlanBasic      PlanTier = "basic"
	PlanPro        PlanTier = "pro"
	PlanEnterprise PlanTier = "enterprise"
)

type SubscriptionStatus string

const (
	SubStatusActive    SubscriptionStatus = "active"
	SubStatusPastDue   SubscriptionStatus = "past_due"
	SubStatusCancelled SubscriptionStatus = "cancelled"
	SubStatusExpired   SubscriptionStatus = "expired"
)

type Plan struct {
	ID           uuid.UUID
	Tier         PlanTier
	Name         string
	PriceMonthly decimal.Decimal
	Currency     string
	MaxDoctors   int
	MaxPatients  int
	Features     []string
}

type Subscription struct {
	ID                 uuid.UUID
	ClinicID           uuid.UUID
	PlanID             uuid.UUID
	Status             SubscriptionStatus
	CurrentPeriodStart time.Time
	CurrentPeriodEnd   time.Time
	CancelledAt        *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}
