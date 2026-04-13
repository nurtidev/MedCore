package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/nurtidev/medcore/internal/integration/domain"
)

// SyncRepository manages integration configs and sync logs.
type SyncRepository interface {
	// Configs
	GetIntegrationConfig(ctx context.Context, clinicID uuid.UUID, provider string) (*domain.IntegrationConfig, error)
	ListIntegrationConfigs(ctx context.Context, clinicID uuid.UUID) ([]*domain.IntegrationConfig, error)
	UpsertIntegrationConfig(ctx context.Context, cfg *domain.IntegrationConfig) error

	// Sync logs
	CreateSyncLog(ctx context.Context, log *domain.SyncLog) error
	UpdateSyncLog(ctx context.Context, log *domain.SyncLog) error
	ListSyncLogs(ctx context.Context, clinicID uuid.UUID, limit, offset int) ([]*domain.SyncLog, error)
}
