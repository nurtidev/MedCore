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

type postgresSyncRepo struct {
	db *pgxpool.Pool
}

// NewPostgresSyncRepo creates a new PostgreSQL-backed SyncRepository.
func NewPostgresSyncRepo(db *pgxpool.Pool) SyncRepository {
	return &postgresSyncRepo{db: db}
}

// ── Integration Configs ───────────────────────────────────────────────────────

func (r *postgresSyncRepo) GetIntegrationConfig(ctx context.Context, clinicID uuid.UUID, provider string) (*domain.IntegrationConfig, error) {
	const q = `
		SELECT id, clinic_id, provider, is_active, config, created_at, updated_at
		FROM integration_configs
		WHERE clinic_id = $1 AND provider = $2`

	row := r.db.QueryRow(ctx, q, clinicID, provider)
	return scanIntegrationConfig(row)
}

func (r *postgresSyncRepo) ListIntegrationConfigs(ctx context.Context, clinicID uuid.UUID) ([]*domain.IntegrationConfig, error) {
	const q = `
		SELECT id, clinic_id, provider, is_active, config, created_at, updated_at
		FROM integration_configs
		WHERE clinic_id = $1
		ORDER BY provider`

	rows, err := r.db.Query(ctx, q, clinicID)
	if err != nil {
		return nil, fmt.Errorf("postgres_sync_repo.ListIntegrationConfigs: %w", err)
	}
	defer rows.Close()

	var configs []*domain.IntegrationConfig
	for rows.Next() {
		cfg, err := scanIntegrationConfig(rows)
		if err != nil {
			return nil, err
		}
		configs = append(configs, cfg)
	}
	return configs, rows.Err()
}

func (r *postgresSyncRepo) UpsertIntegrationConfig(ctx context.Context, cfg *domain.IntegrationConfig) error {
	configJSON, err := json.Marshal(cfg.Config)
	if err != nil {
		return fmt.Errorf("postgres_sync_repo.UpsertIntegrationConfig: marshal config: %w", err)
	}

	const q = `
		INSERT INTO integration_configs (id, clinic_id, provider, is_active, config, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (clinic_id, provider) DO UPDATE
		SET is_active = EXCLUDED.is_active,
		    config    = EXCLUDED.config,
		    updated_at = EXCLUDED.updated_at`

	now := time.Now()
	if cfg.ID == uuid.Nil {
		cfg.ID = uuid.New()
	}
	_, err = r.db.Exec(ctx, q,
		cfg.ID, cfg.ClinicID, cfg.Provider, cfg.IsActive,
		configJSON, now, now,
	)
	if err != nil {
		return fmt.Errorf("postgres_sync_repo.UpsertIntegrationConfig: %w", err)
	}
	return nil
}

// ── Sync Logs ─────────────────────────────────────────────────────────────────

func (r *postgresSyncRepo) CreateSyncLog(ctx context.Context, log *domain.SyncLog) error {
	const q = `
		INSERT INTO sync_logs (id, clinic_id, provider, operation, status, records_processed, error_message, started_at, completed_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	if log.ID == uuid.Nil {
		log.ID = uuid.New()
	}
	_, err := r.db.Exec(ctx, q,
		log.ID, log.ClinicID, log.Provider, log.Operation,
		log.Status, log.RecordsProcessed, nullString(log.ErrorMessage),
		log.StartedAt, log.CompletedAt, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("postgres_sync_repo.CreateSyncLog: %w", err)
	}
	return nil
}

func (r *postgresSyncRepo) UpdateSyncLog(ctx context.Context, log *domain.SyncLog) error {
	const q = `
		UPDATE sync_logs
		SET status = $2, records_processed = $3, error_message = $4, completed_at = $5
		WHERE id = $1`

	_, err := r.db.Exec(ctx, q,
		log.ID, log.Status, log.RecordsProcessed,
		nullString(log.ErrorMessage), log.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres_sync_repo.UpdateSyncLog: %w", err)
	}
	return nil
}

func (r *postgresSyncRepo) ListSyncLogs(ctx context.Context, clinicID uuid.UUID, limit, offset int) ([]*domain.SyncLog, error) {
	const q = `
		SELECT id, clinic_id, provider, operation, status, records_processed,
		       COALESCE(error_message, ''), started_at, completed_at, created_at
		FROM sync_logs
		WHERE clinic_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.Query(ctx, q, clinicID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("postgres_sync_repo.ListSyncLogs: %w", err)
	}
	defer rows.Close()

	var logs []*domain.SyncLog
	for rows.Next() {
		l := &domain.SyncLog{}
		if err := rows.Scan(
			&l.ID, &l.ClinicID, &l.Provider, &l.Operation, &l.Status,
			&l.RecordsProcessed, &l.ErrorMessage,
			&l.StartedAt, &l.CompletedAt, &l.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("postgres_sync_repo.ListSyncLogs: scan: %w", err)
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

// ── Helpers ───────────────────────────────────────────────────────────────────

type rowScanner interface {
	Scan(dest ...any) error
}

func scanIntegrationConfig(row rowScanner) (*domain.IntegrationConfig, error) {
	cfg := &domain.IntegrationConfig{}
	var configJSON []byte
	err := row.Scan(
		&cfg.ID, &cfg.ClinicID, &cfg.Provider, &cfg.IsActive,
		&configJSON, &cfg.CreatedAt, &cfg.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrIntegrationNotConfigured
		}
		return nil, fmt.Errorf("postgres_sync_repo.scanIntegrationConfig: %w", err)
	}
	if err := json.Unmarshal(configJSON, &cfg.Config); err != nil {
		return nil, fmt.Errorf("postgres_sync_repo.scanIntegrationConfig: unmarshal config: %w", err)
	}
	return cfg, nil
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
