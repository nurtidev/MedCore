package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nurtidev/medcore/internal/analytics/domain"
	"github.com/nurtidev/medcore/internal/analytics/service"
)

// ─── Mock repository ──────────────────────────────────────────────────────────

type mockRepo struct{ mock.Mock }

func (m *mockRepo) SaveEvents(ctx context.Context, events []*domain.ClinicEvent) error {
	args := m.Called(ctx, events)
	return args.Error(0)
}

func (m *mockRepo) GetDoctorWorkload(ctx context.Context, req domain.WorkloadRequest) ([]*domain.DoctorWorkload, error) {
	args := m.Called(ctx, req)
	return args.Get(0).([]*domain.DoctorWorkload), args.Error(1)
}

func (m *mockRepo) GetClinicRevenue(ctx context.Context, req domain.RevenueRequest) (*domain.ClinicRevenue, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*domain.ClinicRevenue), args.Error(1)
}

func (m *mockRepo) GetScheduleFillRate(ctx context.Context, req domain.FillRateRequest) (*domain.ScheduleFillRate, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*domain.ScheduleFillRate), args.Error(1)
}

func (m *mockRepo) GetPatientFunnel(ctx context.Context, req domain.FunnelRequest) (*domain.PatientFunnel, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*domain.PatientFunnel), args.Error(1)
}

// newService creates a service with a nil Redis client (cache is skipped on error).
func newService(repo *mockRepo) domain.AnalyticsService {
	return service.New(repo, nil)
}

// ─── Tests ────────────────────────────────────────────────────────────────────

func TestGetDoctorWorkload_Success(t *testing.T) {
	ctx := context.Background()
	clinicID := uuid.New()

	expected := []*domain.DoctorWorkload{
		{
			DoctorID:          uuid.New().String(),
			Period:            "2025-04",
			TotalAppointments: 40,
			CompletedCount:    32,
			NoShowCount:       4,
			CancelledCount:    4,
			WorkloadPercent:   80.0,
			NoShowRate:        10.0,
		},
	}

	repo := &mockRepo{}
	req := domain.WorkloadRequest{ClinicID: clinicID, Period: "2025-04"}
	repo.On("GetDoctorWorkload", ctx, req).Return(expected, nil)

	svc := newService(repo)
	result, err := svc.GetDoctorWorkload(ctx, req)

	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, int64(40), result[0].TotalAppointments)
	assert.Equal(t, int64(32), result[0].CompletedCount)
	assert.InDelta(t, 80.0, result[0].WorkloadPercent, 0.001)
	repo.AssertExpectations(t)
}

func TestGetDoctorWorkload_EmptyPeriod(t *testing.T) {
	ctx := context.Background()
	clinicID := uuid.New()

	repo := &mockRepo{}
	req := domain.WorkloadRequest{ClinicID: clinicID, Period: "2025-04"}
	repo.On("GetDoctorWorkload", ctx, req).Return([]*domain.DoctorWorkload{}, nil)

	svc := newService(repo)
	result, err := svc.GetDoctorWorkload(ctx, req)

	require.NoError(t, err)
	assert.Empty(t, result)
	repo.AssertExpectations(t)
}

func TestGetClinicRevenue_GroupByDay(t *testing.T) {
	ctx := context.Background()
	clinicID := uuid.New()
	start := time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC)

	expected := &domain.ClinicRevenue{
		ClinicID:     clinicID.String(),
		TotalRevenue: 150000.0,
		Currency:     "KZT",
		PaymentCount: 30,
		AvgCheck:     5000.0,
		RevenueByDay: []domain.DailyRevenue{
			{Date: "2025-04-01", Revenue: 5000, Count: 1},
		},
	}

	repo := &mockRepo{}
	req := domain.RevenueRequest{ClinicID: clinicID, StartDate: start, EndDate: end, Grouping: "day"}
	repo.On("GetClinicRevenue", ctx, req).Return(expected, nil)

	svc := newService(repo)
	result, err := svc.GetClinicRevenue(ctx, req)

	require.NoError(t, err)
	assert.InDelta(t, 150000.0, result.TotalRevenue, 0.001)
	assert.Equal(t, "KZT", result.Currency)
	assert.InDelta(t, 5000.0, result.AvgCheck, 0.001)
	repo.AssertExpectations(t)
}

func TestGetClinicRevenue_GroupByMonth(t *testing.T) {
	ctx := context.Background()
	clinicID := uuid.New()
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	expected := &domain.ClinicRevenue{
		ClinicID:     clinicID.String(),
		TotalRevenue: 1_800_000.0,
		Currency:     "KZT",
		PaymentCount: 360,
	}

	repo := &mockRepo{}
	req := domain.RevenueRequest{ClinicID: clinicID, StartDate: start, EndDate: end, Grouping: "month"}
	repo.On("GetClinicRevenue", ctx, req).Return(expected, nil)

	svc := newService(repo)
	result, err := svc.GetClinicRevenue(ctx, req)

	require.NoError(t, err)
	assert.InDelta(t, 1_800_000.0, result.TotalRevenue, 0.001)
	assert.Equal(t, int64(360), result.PaymentCount)
	repo.AssertExpectations(t)
}

func TestGetDashboard_UnderTwoSeconds(t *testing.T) {
	ctx := context.Background()
	clinicID := uuid.New()
	period := "2025-04"

	periodStart := time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := periodStart.AddDate(0, 1, 0)

	repo := &mockRepo{}
	repo.On("GetDoctorWorkload", ctx, domain.WorkloadRequest{ClinicID: clinicID, Period: period}).
		Return([]*domain.DoctorWorkload{}, nil)
	repo.On("GetClinicRevenue", ctx, domain.RevenueRequest{
		ClinicID: clinicID, StartDate: periodStart, EndDate: periodEnd, Grouping: "day",
	}).Return(&domain.ClinicRevenue{ClinicID: clinicID.String()}, nil)
	repo.On("GetScheduleFillRate", ctx, domain.FillRateRequest{ClinicID: clinicID, Period: period}).
		Return(&domain.ScheduleFillRate{ClinicID: clinicID.String()}, nil)
	repo.On("GetPatientFunnel", ctx, domain.FunnelRequest{ClinicID: clinicID, Period: period}).
		Return(&domain.PatientFunnel{ClinicID: clinicID.String()}, nil)

	svc := newService(repo)

	start := time.Now()
	dash, err := svc.GetDashboard(ctx, clinicID, period)
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.NotNil(t, dash)
	assert.Less(t, elapsed, 2*time.Second, "dashboard must load in ≤2s")
	repo.AssertExpectations(t)
}

func TestRecordEventBatch_Success(t *testing.T) {
	ctx := context.Background()

	events := []*domain.ClinicEvent{
		{
			EventID:   uuid.New().String(),
			ClinicID:  uuid.New().String(),
			EventType: domain.EventPaymentCompleted,
			Amount:    5000.0,
			Currency:  "KZT",
			CreatedAt: time.Now(),
		},
		{
			EventID:   uuid.New().String(),
			ClinicID:  uuid.New().String(),
			EventType: domain.EventAppointmentCompleted,
			CreatedAt: time.Now(),
		},
	}

	repo := &mockRepo{}
	repo.On("SaveEvents", ctx, events).Return(nil)

	svc := newService(repo)
	err := svc.RecordEventBatch(ctx, events)

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestExportToExcel_Success(t *testing.T) {
	ctx := context.Background()
	clinicID := uuid.New()

	expected := &domain.ClinicRevenue{
		ClinicID:     clinicID.String(),
		TotalRevenue: 10000.0,
		Currency:     "KZT",
		PaymentCount: 2,
		RevenueByDay: []domain.DailyRevenue{
			{Date: "2025-04-01", Revenue: 5000, Count: 1},
			{Date: "2025-04-02", Revenue: 5000, Count: 1},
		},
	}

	repo := &mockRepo{}
	repo.On("GetClinicRevenue", ctx, mock.MatchedBy(func(r domain.RevenueRequest) bool {
		return r.ClinicID == clinicID
	})).Return(expected, nil)

	svc := newService(repo)
	data, err := svc.ExportToExcel(ctx, domain.ExportRequest{
		ClinicID: clinicID,
		Period:   "2025-04",
		Type:     "revenue",
	})

	require.NoError(t, err)
	assert.NotEmpty(t, data)
	// Check UTF-8 BOM is present (Excel marker).
	assert.Equal(t, byte(0xEF), data[0])
	assert.Equal(t, byte(0xBB), data[1])
	assert.Equal(t, byte(0xBF), data[2])
	repo.AssertExpectations(t)
}
