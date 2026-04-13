package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ─── KPI structs ──────────────────────────────────────────────────────────────

type DoctorWorkload struct {
	DoctorID          string
	DoctorName        string
	Period            string // "2025-04"
	TotalAppointments int64
	CompletedCount    int64
	NoShowCount       int64
	CancelledCount    int64
	WorkloadPercent   float64 // completed / scheduled slots
	NoShowRate        float64 // no_show / total
}

type ClinicRevenue struct {
	ClinicID     string
	Period       string
	TotalRevenue float64
	Currency     string
	PaymentCount int64
	AvgCheck     float64
	RevenueByDay []DailyRevenue
}

type DailyRevenue struct {
	Date    string
	Revenue float64
	Count   int64
}

type ScheduleFillRate struct {
	ClinicID        string
	Period          string
	TotalSlots      int64
	FilledSlots     int64
	FillRatePercent float64
}

type PatientFunnel struct {
	ClinicID       string
	Period         string
	NewPatients    int64
	ReturnPatients int64
	RetentionRate  float64
}

// Dashboard aggregates all KPIs for one clinic+period in a single response.
type Dashboard struct {
	ClinicID     string
	Period       string
	Workloads    []*DoctorWorkload
	Revenue      *ClinicRevenue
	FillRate     *ScheduleFillRate
	PatientStats *PatientFunnel
}

// ─── Request types ────────────────────────────────────────────────────────────

type WorkloadRequest struct {
	ClinicID uuid.UUID
	Period   string     // "2025-04"
	DoctorID *uuid.UUID // nil → all doctors
}

type RevenueRequest struct {
	ClinicID  uuid.UUID
	StartDate time.Time
	EndDate   time.Time
	Grouping  string // "day", "week", "month"
}

type FillRateRequest struct {
	ClinicID uuid.UUID
	Period   string
}

type FunnelRequest struct {
	ClinicID uuid.UUID
	Period   string
}

type ExportRequest struct {
	ClinicID uuid.UUID
	Period   string
	Type     string // "revenue", "workload", "fill_rate", "funnel"
}

// ─── Service interface ────────────────────────────────────────────────────────

type AnalyticsService interface {
	// KPI dashboards (≤2s — TZ requirement)
	GetDoctorWorkload(ctx context.Context, req WorkloadRequest) ([]*DoctorWorkload, error)
	GetClinicRevenue(ctx context.Context, req RevenueRequest) (*ClinicRevenue, error)
	GetScheduleFillRate(ctx context.Context, req FillRateRequest) (*ScheduleFillRate, error)
	GetPatientFunnel(ctx context.Context, req FunnelRequest) (*PatientFunnel, error)

	// Aggregated dashboard (all KPIs in one call)
	GetDashboard(ctx context.Context, clinicID uuid.UUID, period string) (*Dashboard, error)

	// Export (TZ: PDF/Excel)
	ExportToExcel(ctx context.Context, req ExportRequest) ([]byte, error)
	ExportToCSV(ctx context.Context, req ExportRequest) ([]byte, error)

	// Event ingestion (called by Kafka consumer)
	RecordEvent(ctx context.Context, event *ClinicEvent) error
	RecordEventBatch(ctx context.Context, events []*ClinicEvent) error
}
