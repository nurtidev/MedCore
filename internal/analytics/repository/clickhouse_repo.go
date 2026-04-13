package repository

import (
	"context"

	"github.com/nurtidev/medcore/internal/analytics/domain"
)

// ClickHouseRepository defines all ClickHouse operations for analytics.
type ClickHouseRepository interface {
	// SaveEvents performs a batch insert — never called with a single row.
	SaveEvents(ctx context.Context, events []*domain.ClinicEvent) error

	// GetDoctorWorkload reads from doctor_workload_mv materialized view.
	GetDoctorWorkload(ctx context.Context, req domain.WorkloadRequest) ([]*domain.DoctorWorkload, error)

	// GetClinicRevenue reads from clinic_revenue_mv and groups by req.Grouping.
	GetClinicRevenue(ctx context.Context, req domain.RevenueRequest) (*domain.ClinicRevenue, error)

	// GetScheduleFillRate reads from schedule_fill_mv.
	GetScheduleFillRate(ctx context.Context, req domain.FillRateRequest) (*domain.ScheduleFillRate, error)

	// GetPatientFunnel counts new vs returning patients from clinic_events.
	GetPatientFunnel(ctx context.Context, req domain.FunnelRequest) (*domain.PatientFunnel, error)
}
