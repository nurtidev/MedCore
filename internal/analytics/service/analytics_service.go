package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/redis/go-redis/v9"

	"github.com/nurtidev/medcore/internal/analytics/domain"
	"github.com/nurtidev/medcore/internal/analytics/repository"
)

// ─── Prometheus metrics ───────────────────────────────────────────────────────

var (
	eventsIngested = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "analytics_events_ingested_total",
		Help: "Total number of analytics events ingested.",
	}, []string{"event_type"})

	queryDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "analytics_query_duration_seconds",
		Help:    "Duration of ClickHouse queries.",
		Buckets: prometheus.DefBuckets,
	}, []string{"query_type"})

	batchSize = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "analytics_clickhouse_batch_size",
		Help:    "Number of events per ClickHouse batch insert.",
		Buckets: []float64{1, 5, 10, 25, 50, 100, 200},
	})
)

// ─── Cache keys ───────────────────────────────────────────────────────────────

const (
	cacheDashboardTTL = time.Hour
	cacheRevenueTTL   = 30 * time.Minute
)

func dashboardCacheKey(clinicID, period string) string {
	return fmt.Sprintf("analytics:dashboard:%s:%s", clinicID, period)
}

func revenueCacheKey(clinicID, start, end, grouping string) string {
	return fmt.Sprintf("analytics:revenue:%s:%s:%s:%s", clinicID, start, end, grouping)
}

// ─── Service ──────────────────────────────────────────────────────────────────

type analyticsService struct {
	repo  repository.ClickHouseRepository
	redis *redis.Client
}

// New creates an AnalyticsService.
func New(repo repository.ClickHouseRepository, rdb *redis.Client) domain.AnalyticsService {
	return &analyticsService{repo: repo, redis: rdb}
}

// ─── GetDoctorWorkload ────────────────────────────────────────────────────────

func (s *analyticsService) GetDoctorWorkload(ctx context.Context, req domain.WorkloadRequest) ([]*domain.DoctorWorkload, error) {
	timer := prometheus.NewTimer(queryDuration.WithLabelValues("doctor_workload"))
	defer timer.ObserveDuration()

	return s.repo.GetDoctorWorkload(ctx, req)
}

// ─── GetClinicRevenue ─────────────────────────────────────────────────────────

func (s *analyticsService) GetClinicRevenue(ctx context.Context, req domain.RevenueRequest) (*domain.ClinicRevenue, error) {
	timer := prometheus.NewTimer(queryDuration.WithLabelValues("clinic_revenue"))
	defer timer.ObserveDuration()

	cacheKey := revenueCacheKey(
		req.ClinicID.String(),
		req.StartDate.Format("2006-01-02"),
		req.EndDate.Format("2006-01-02"),
		req.Grouping,
	)

	if s.redis != nil {
		if cached, err := s.redis.Get(ctx, cacheKey).Bytes(); err == nil {
			var result domain.ClinicRevenue
			if json.Unmarshal(cached, &result) == nil {
				return &result, nil
			}
		}
	}

	result, err := s.repo.GetClinicRevenue(ctx, req)
	if err != nil {
		return nil, err
	}

	if s.redis != nil {
		if data, err := json.Marshal(result); err == nil {
			_ = s.redis.Set(ctx, cacheKey, data, cacheRevenueTTL).Err()
		}
	}
	return result, nil
}

// ─── GetScheduleFillRate ──────────────────────────────────────────────────────

func (s *analyticsService) GetScheduleFillRate(ctx context.Context, req domain.FillRateRequest) (*domain.ScheduleFillRate, error) {
	timer := prometheus.NewTimer(queryDuration.WithLabelValues("schedule_fill_rate"))
	defer timer.ObserveDuration()

	return s.repo.GetScheduleFillRate(ctx, req)
}

// ─── GetPatientFunnel ─────────────────────────────────────────────────────────

func (s *analyticsService) GetPatientFunnel(ctx context.Context, req domain.FunnelRequest) (*domain.PatientFunnel, error) {
	timer := prometheus.NewTimer(queryDuration.WithLabelValues("patient_funnel"))
	defer timer.ObserveDuration()

	return s.repo.GetPatientFunnel(ctx, req)
}

// ─── GetDashboard ─────────────────────────────────────────────────────────────

func (s *analyticsService) GetDashboard(ctx context.Context, clinicID uuid.UUID, period string) (*domain.Dashboard, error) {
	timer := prometheus.NewTimer(queryDuration.WithLabelValues("dashboard"))
	defer timer.ObserveDuration()

	cacheKey := dashboardCacheKey(clinicID.String(), period)

	if s.redis != nil {
		if cached, err := s.redis.Get(ctx, cacheKey).Bytes(); err == nil {
			var dash domain.Dashboard
			if json.Unmarshal(cached, &dash) == nil {
				return &dash, nil
			}
		}
	}

	// Parse period to derive revenue date range.
	periodStart, err := time.Parse("2006-01", period)
	if err != nil {
		return nil, domain.ErrInvalidPeriod
	}
	periodEnd := periodStart.AddDate(0, 1, 0)

	workloads, err := s.repo.GetDoctorWorkload(ctx, domain.WorkloadRequest{
		ClinicID: clinicID,
		Period:   period,
	})
	if err != nil {
		return nil, fmt.Errorf("GetDashboard: workload: %w", err)
	}

	revenue, err := s.repo.GetClinicRevenue(ctx, domain.RevenueRequest{
		ClinicID:  clinicID,
		StartDate: periodStart,
		EndDate:   periodEnd,
		Grouping:  "day",
	})
	if err != nil {
		return nil, fmt.Errorf("GetDashboard: revenue: %w", err)
	}

	fillRate, err := s.repo.GetScheduleFillRate(ctx, domain.FillRateRequest{
		ClinicID: clinicID,
		Period:   period,
	})
	if err != nil {
		return nil, fmt.Errorf("GetDashboard: fill rate: %w", err)
	}

	funnel, err := s.repo.GetPatientFunnel(ctx, domain.FunnelRequest{
		ClinicID: clinicID,
		Period:   period,
	})
	if err != nil {
		return nil, fmt.Errorf("GetDashboard: funnel: %w", err)
	}

	dash := &domain.Dashboard{
		ClinicID:     clinicID.String(),
		Period:       period,
		Workloads:    workloads,
		Revenue:      revenue,
		FillRate:     fillRate,
		PatientStats: funnel,
	}

	if s.redis != nil {
		if data, err := json.Marshal(dash); err == nil {
			_ = s.redis.Set(ctx, cacheKey, data, cacheDashboardTTL).Err()
		}
	}
	return dash, nil
}

// ─── RecordEvent ──────────────────────────────────────────────────────────────

func (s *analyticsService) RecordEvent(ctx context.Context, event *domain.ClinicEvent) error {
	return s.RecordEventBatch(ctx, []*domain.ClinicEvent{event})
}

func (s *analyticsService) RecordEventBatch(ctx context.Context, events []*domain.ClinicEvent) error {
	if len(events) == 0 {
		return nil
	}

	batchSize.Observe(float64(len(events)))

	if err := s.repo.SaveEvents(ctx, events); err != nil {
		return fmt.Errorf("RecordEventBatch: %w", err)
	}

	// Invalidate dashboard cache for all affected clinic IDs.
	if s.redis != nil {
		clinics := uniqueClinicIDs(events)
		for _, cid := range clinics {
			pattern := fmt.Sprintf("analytics:dashboard:%s:*", cid)
			keys, _ := s.redis.Keys(ctx, pattern).Result()
			if len(keys) > 0 {
				_ = s.redis.Del(ctx, keys...).Err()
			}
		}
	}

	for _, e := range events {
		eventsIngested.WithLabelValues(string(e.EventType)).Inc()
	}
	return nil
}

// ─── ExportToCSV ─────────────────────────────────────────────────────────────

func (s *analyticsService) ExportToCSV(ctx context.Context, req domain.ExportRequest) ([]byte, error) {
	rows, headers, err := s.buildExportRows(ctx, req)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write(headers); err != nil {
		return nil, fmt.Errorf("ExportToCSV: write header: %w", err)
	}
	for _, row := range rows {
		if err := w.Write(row); err != nil {
			return nil, fmt.Errorf("ExportToCSV: write row: %w", err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("ExportToCSV: flush: %w", err)
	}
	return buf.Bytes(), nil
}

// ExportToExcel returns CSV with UTF-8 BOM, which Excel opens natively.
func (s *analyticsService) ExportToExcel(ctx context.Context, req domain.ExportRequest) ([]byte, error) {
	csvBytes, err := s.ExportToCSV(ctx, req)
	if err != nil {
		return nil, err
	}
	// UTF-8 BOM for Excel compatibility.
	bom := []byte{0xEF, 0xBB, 0xBF}
	return append(bom, csvBytes...), nil
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func (s *analyticsService) buildExportRows(ctx context.Context, req domain.ExportRequest) (rows [][]string, headers []string, err error) {
	periodStart, err := time.Parse("2006-01", req.Period)
	if err != nil {
		return nil, nil, domain.ErrInvalidPeriod
	}
	periodEnd := periodStart.AddDate(0, 1, 0)

	switch strings.ToLower(req.Type) {
	case "revenue":
		revenue, err := s.repo.GetClinicRevenue(ctx, domain.RevenueRequest{
			ClinicID:  req.ClinicID,
			StartDate: periodStart,
			EndDate:   periodEnd,
			Grouping:  "day",
		})
		if err != nil {
			return nil, nil, err
		}
		headers = []string{"Date", "Revenue", "Count"}
		for _, d := range revenue.RevenueByDay {
			rows = append(rows, []string{
				d.Date,
				fmt.Sprintf("%.2f", d.Revenue),
				fmt.Sprintf("%d", d.Count),
			})
		}

	case "workload":
		workloads, err := s.repo.GetDoctorWorkload(ctx, domain.WorkloadRequest{
			ClinicID: req.ClinicID,
			Period:   req.Period,
		})
		if err != nil {
			return nil, nil, err
		}
		headers = []string{"DoctorID", "Period", "Total", "Completed", "NoShow", "Cancelled", "WorkloadPct", "NoShowRate"}
		for _, w := range workloads {
			rows = append(rows, []string{
				w.DoctorID, w.Period,
				fmt.Sprintf("%d", w.TotalAppointments),
				fmt.Sprintf("%d", w.CompletedCount),
				fmt.Sprintf("%d", w.NoShowCount),
				fmt.Sprintf("%d", w.CancelledCount),
				fmt.Sprintf("%.2f", w.WorkloadPercent),
				fmt.Sprintf("%.2f", w.NoShowRate),
			})
		}

	case "fill_rate":
		fr, err := s.repo.GetScheduleFillRate(ctx, domain.FillRateRequest{
			ClinicID: req.ClinicID,
			Period:   req.Period,
		})
		if err != nil {
			return nil, nil, err
		}
		headers = []string{"ClinicID", "Period", "TotalSlots", "FilledSlots", "FillRatePct"}
		rows = [][]string{{
			fr.ClinicID, fr.Period,
			fmt.Sprintf("%d", fr.TotalSlots),
			fmt.Sprintf("%d", fr.FilledSlots),
			fmt.Sprintf("%.2f", fr.FillRatePercent),
		}}

	case "funnel":
		f, err := s.repo.GetPatientFunnel(ctx, domain.FunnelRequest{
			ClinicID: req.ClinicID,
			Period:   req.Period,
		})
		if err != nil {
			return nil, nil, err
		}
		headers = []string{"ClinicID", "Period", "NewPatients", "ReturnPatients", "RetentionRate"}
		rows = [][]string{{
			f.ClinicID, f.Period,
			fmt.Sprintf("%d", f.NewPatients),
			fmt.Sprintf("%d", f.ReturnPatients),
			fmt.Sprintf("%.2f", f.RetentionRate),
		}}

	default:
		return nil, nil, fmt.Errorf("unknown export type: %s", req.Type)
	}
	return rows, headers, nil
}

func uniqueClinicIDs(events []*domain.ClinicEvent) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, e := range events {
		if _, ok := seen[e.ClinicID]; !ok {
			seen[e.ClinicID] = struct{}{}
			out = append(out, e.ClinicID)
		}
	}
	return out
}
