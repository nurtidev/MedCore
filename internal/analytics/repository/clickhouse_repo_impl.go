package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"

	"github.com/nurtidev/medcore/internal/analytics/domain"
)

type clickhouseRepo struct {
	conn driver.Conn
}

// NewClickHouseRepo creates a ClickHouseRepository backed by clickhouse-go/v2.
func NewClickHouseRepo(conn driver.Conn) ClickHouseRepository {
	return &clickhouseRepo{conn: conn}
}

// ─── SaveEvents ───────────────────────────────────────────────────────────────

func (r *clickhouseRepo) SaveEvents(ctx context.Context, events []*domain.ClinicEvent) error {
	if len(events) == 0 {
		return nil
	}

	batch, err := r.conn.PrepareBatch(ctx, `
		INSERT INTO clinic_events
		(event_id, clinic_id, doctor_id, patient_id, event_type, amount, currency, created_at, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("clickhouseRepo.SaveEvents: prepare batch: %w", err)
	}

	for _, e := range events {
		eventID, err := uuid.Parse(e.EventID)
		if err != nil {
			eventID = uuid.New()
		}
		clinicID, _ := uuid.Parse(e.ClinicID)
		doctorID, _ := uuid.Parse(e.DoctorID)
		patientID, _ := uuid.Parse(e.PatientID)

		if err := batch.Append(
			eventID,
			clinicID,
			doctorID,
			patientID,
			string(e.EventType),
			e.Amount,
			e.Currency,
			e.CreatedAt,
			e.Metadata,
		); err != nil {
			return fmt.Errorf("clickhouseRepo.SaveEvents: append: %w", err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("clickhouseRepo.SaveEvents: send: %w", err)
	}
	return nil
}

// ─── GetDoctorWorkload ────────────────────────────────────────────────────────

func (r *clickhouseRepo) GetDoctorWorkload(ctx context.Context, req domain.WorkloadRequest) ([]*domain.DoctorWorkload, error) {
	// Parse period "YYYY-MM" → start/end of month for partition pruning.
	periodStart, err := parsePeriod(req.Period)
	if err != nil {
		return nil, domain.ErrInvalidPeriod
	}
	periodEnd := periodStart.AddDate(0, 1, 0)

	query := `
		SELECT
			doctor_id,
			period,
			sum(total_appointments) AS total_appointments,
			sum(completed_count)    AS completed_count,
			sum(no_show_count)      AS no_show_count,
			sum(cancelled_count)    AS cancelled_count
		FROM doctor_workload_mv
		WHERE clinic_id = ?
		  AND period >= ? AND period < ?
	`
	args := []any{req.ClinicID, periodStart, periodEnd}

	if req.DoctorID != nil {
		query += " AND doctor_id = ?"
		args = append(args, *req.DoctorID)
	}
	query += " GROUP BY doctor_id, period ORDER BY doctor_id"

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("clickhouseRepo.GetDoctorWorkload: query: %w", err)
	}
	defer rows.Close()

	var results []*domain.DoctorWorkload
	for rows.Next() {
		var (
			doctorID          uuid.UUID
			period            time.Time
			totalAppointments int64
			completedCount    int64
			noShowCount       int64
			cancelledCount    int64
		)
		if err := rows.Scan(&doctorID, &period, &totalAppointments, &completedCount, &noShowCount, &cancelledCount); err != nil {
			return nil, fmt.Errorf("clickhouseRepo.GetDoctorWorkload: scan: %w", err)
		}

		wl := &domain.DoctorWorkload{
			DoctorID:          doctorID.String(),
			Period:            period.Format("2006-01"),
			TotalAppointments: totalAppointments,
			CompletedCount:    completedCount,
			NoShowCount:       noShowCount,
			CancelledCount:    cancelledCount,
		}
		if totalAppointments > 0 {
			wl.WorkloadPercent = float64(completedCount) / float64(totalAppointments) * 100
			wl.NoShowRate = float64(noShowCount) / float64(totalAppointments) * 100
		}
		results = append(results, wl)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("clickhouseRepo.GetDoctorWorkload: rows: %w", err)
	}
	return results, nil
}

// ─── GetClinicRevenue ─────────────────────────────────────────────────────────

func (r *clickhouseRepo) GetClinicRevenue(ctx context.Context, req domain.RevenueRequest) (*domain.ClinicRevenue, error) {
	var groupBy string
	switch req.Grouping {
	case "week":
		groupBy = "toStartOfWeek(period)"
	case "month":
		groupBy = "toStartOfMonth(period)"
	default: // "day"
		groupBy = "period"
	}

	query := fmt.Sprintf(`
		SELECT
			%s                       AS bucket,
			currency,
			sum(total_revenue)       AS revenue,
			sum(payment_count)       AS payment_count
		FROM clinic_revenue_mv
		WHERE clinic_id = ?
		  AND period >= ? AND period < ?
		GROUP BY bucket, currency
		ORDER BY bucket
	`, groupBy)

	rows, err := r.conn.Query(ctx, query, req.ClinicID, req.StartDate, req.EndDate)
	if err != nil {
		return nil, fmt.Errorf("clickhouseRepo.GetClinicRevenue: query: %w", err)
	}
	defer rows.Close()

	result := &domain.ClinicRevenue{
		ClinicID: req.ClinicID.String(),
		Period:   req.StartDate.Format("2006-01-02") + "/" + req.EndDate.Format("2006-01-02"),
	}

	for rows.Next() {
		var (
			bucket       time.Time
			currency     string
			revenue      float64
			paymentCount int64
		)
		if err := rows.Scan(&bucket, &currency, &revenue, &paymentCount); err != nil {
			return nil, fmt.Errorf("clickhouseRepo.GetClinicRevenue: scan: %w", err)
		}

		result.TotalRevenue += revenue
		result.PaymentCount += paymentCount
		if result.Currency == "" {
			result.Currency = currency
		}
		result.RevenueByDay = append(result.RevenueByDay, domain.DailyRevenue{
			Date:    bucket.Format("2006-01-02"),
			Revenue: revenue,
			Count:   paymentCount,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("clickhouseRepo.GetClinicRevenue: rows: %w", err)
	}
	if result.PaymentCount > 0 {
		result.AvgCheck = result.TotalRevenue / float64(result.PaymentCount)
	}
	return result, nil
}

// ─── GetScheduleFillRate ──────────────────────────────────────────────────────

func (r *clickhouseRepo) GetScheduleFillRate(ctx context.Context, req domain.FillRateRequest) (*domain.ScheduleFillRate, error) {
	periodStart, err := parsePeriod(req.Period)
	if err != nil {
		return nil, domain.ErrInvalidPeriod
	}
	periodEnd := periodStart.AddDate(0, 1, 0)

	var totalSlots, filledSlots int64
	err = r.conn.QueryRow(ctx, `
		SELECT
			sum(total_slots)  AS total_slots,
			sum(filled_slots) AS filled_slots
		FROM schedule_fill_mv
		WHERE clinic_id = ?
		  AND period >= ? AND period < ?
	`, req.ClinicID, periodStart, periodEnd).Scan(&totalSlots, &filledSlots)
	if err != nil {
		return nil, fmt.Errorf("clickhouseRepo.GetScheduleFillRate: query: %w", err)
	}

	result := &domain.ScheduleFillRate{
		ClinicID:    req.ClinicID.String(),
		Period:      req.Period,
		TotalSlots:  totalSlots,
		FilledSlots: filledSlots,
	}
	if totalSlots > 0 {
		result.FillRatePercent = float64(filledSlots) / float64(totalSlots) * 100
	}
	return result, nil
}

// ─── GetPatientFunnel ─────────────────────────────────────────────────────────

func (r *clickhouseRepo) GetPatientFunnel(ctx context.Context, req domain.FunnelRequest) (*domain.PatientFunnel, error) {
	periodStart, err := parsePeriod(req.Period)
	if err != nil {
		return nil, domain.ErrInvalidPeriod
	}
	periodEnd := periodStart.AddDate(0, 1, 0)

	// New patients: first appointment in the requested period.
	// Return patients: had an appointment before periodStart AND also in this period.
	var newPatients, returnPatients int64
	err = r.conn.QueryRow(ctx, `
		SELECT
			countIf(first_seen >= ? AND first_seen < ?) AS new_patients,
			countIf(first_seen < ? AND last_seen >= ?)  AS return_patients
		FROM (
			SELECT
				patient_id,
				min(created_at) AS first_seen,
				max(created_at) AS last_seen
			FROM clinic_events
			WHERE clinic_id = ?
			  AND event_type IN ('appointment.created', 'appointment.completed')
			GROUP BY patient_id
		)
	`, periodStart, periodEnd, periodStart, periodStart, req.ClinicID).Scan(&newPatients, &returnPatients)
	if err != nil {
		return nil, fmt.Errorf("clickhouseRepo.GetPatientFunnel: query: %w", err)
	}

	result := &domain.PatientFunnel{
		ClinicID:       req.ClinicID.String(),
		Period:         req.Period,
		NewPatients:    newPatients,
		ReturnPatients: returnPatients,
	}
	total := newPatients + returnPatients
	if total > 0 {
		result.RetentionRate = float64(returnPatients) / float64(total) * 100
	}
	return result, nil
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func parsePeriod(period string) (time.Time, error) {
	t, err := time.Parse("2006-01", period)
	if err != nil {
		return time.Time{}, fmt.Errorf("parsePeriod: %w", err)
	}
	return t, nil
}
