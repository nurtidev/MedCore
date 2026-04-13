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
	"github.com/nurtidev/medcore/internal/integration/domain"
)

type postgresLabResultRepo struct {
	db *pgxpool.Pool
}

// NewPostgresLabResultRepo creates a new PostgreSQL-backed LabResultRepository.
func NewPostgresLabResultRepo(db *pgxpool.Pool) LabResultRepository {
	return &postgresLabResultRepo{db: db}
}

func (r *postgresLabResultRepo) Create(ctx context.Context, result *domain.LabResult) error {
	dataJSON, err := json.Marshal(result.Data)
	if err != nil {
		return fmt.Errorf("postgres_lab_result_repo.Create: marshal data: %w", err)
	}

	const q = `
		INSERT INTO lab_results
		    (id, clinic_id, patient_id, external_id, lab_provider, test_name, format, file_url, data, received_at, attached_at, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`

	if result.ID == uuid.Nil {
		result.ID = uuid.New()
	}
	_, err = r.db.Exec(ctx, q,
		result.ID, result.ClinicID, result.PatientID, result.ExternalID,
		result.LabProvider, result.TestName, string(result.Format),
		nullStrPtr(result.FileURL), dataJSON,
		result.ReceivedAt, result.AttachedAt, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("postgres_lab_result_repo.Create: %w", err)
	}
	return nil
}

func (r *postgresLabResultRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.LabResult, error) {
	const q = `
		SELECT id, clinic_id, patient_id, external_id, lab_provider, test_name, format,
		       COALESCE(file_url,''), data, received_at, attached_at
		FROM lab_results
		WHERE id = $1`

	row := r.db.QueryRow(ctx, q, id)
	result, err := scanLabResult(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrLabResultNotFound
		}
		return nil, fmt.Errorf("postgres_lab_result_repo.GetByID: %w", err)
	}
	return result, nil
}

func (r *postgresLabResultRepo) ListByClinic(ctx context.Context, clinicID uuid.UUID, limit, offset int) ([]*domain.LabResult, error) {
	const q = `
		SELECT id, clinic_id, patient_id, external_id, lab_provider, test_name, format,
		       COALESCE(file_url,''), data, received_at, attached_at
		FROM lab_results
		WHERE clinic_id = $1
		ORDER BY received_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.Query(ctx, q, clinicID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("postgres_lab_result_repo.ListByClinic: %w", err)
	}
	defer rows.Close()

	var results []*domain.LabResult
	for rows.Next() {
		res, err := scanLabResult(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres_lab_result_repo.ListByClinic: scan: %w", err)
		}
		results = append(results, res)
	}
	return results, rows.Err()
}

func (r *postgresLabResultRepo) AttachToPatient(ctx context.Context, resultID uuid.UUID, patientID uuid.UUID) error {
	const q = `
		UPDATE lab_results
		SET patient_id = $2, attached_at = $3
		WHERE id = $1`

	now := time.Now()
	tag, err := r.db.Exec(ctx, q, resultID, patientID, now)
	if err != nil {
		return fmt.Errorf("postgres_lab_result_repo.AttachToPatient: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrLabResultNotFound
	}
	return nil
}

func (r *postgresLabResultRepo) ExistsByExternalID(ctx context.Context, externalID, provider string) (bool, error) {
	const q = `SELECT EXISTS(SELECT 1 FROM lab_results WHERE external_id=$1 AND lab_provider=$2)`
	var exists bool
	if err := r.db.QueryRow(ctx, q, externalID, provider).Scan(&exists); err != nil {
		return false, fmt.Errorf("postgres_lab_result_repo.ExistsByExternalID: %w", err)
	}
	return exists, nil
}

func scanLabResult(row rowScanner) (*domain.LabResult, error) {
	result := &domain.LabResult{}
	var dataJSON []byte
	var format string
	err := row.Scan(
		&result.ID, &result.ClinicID, &result.PatientID, &result.ExternalID,
		&result.LabProvider, &result.TestName, &format,
		&result.FileURL, &dataJSON, &result.ReceivedAt, &result.AttachedAt,
	)
	if err != nil {
		return nil, err
	}
	result.Format = domain.LabResultFormat(format)
	if len(dataJSON) > 0 && string(dataJSON) != "null" {
		if err := json.Unmarshal(dataJSON, &result.Data); err != nil {
			return nil, fmt.Errorf("scanLabResult: unmarshal data: %w", err)
		}
	}
	return result, nil
}

func nullStrPtr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
