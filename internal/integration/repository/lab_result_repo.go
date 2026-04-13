package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/nurtidev/medcore/internal/integration/domain"
)

// LabResultRepository manages lab results.
type LabResultRepository interface {
	Create(ctx context.Context, result *domain.LabResult) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.LabResult, error)
	ListByClinic(ctx context.Context, clinicID uuid.UUID, limit, offset int) ([]*domain.LabResult, error)
	AttachToPatient(ctx context.Context, resultID uuid.UUID, patientID uuid.UUID) error
	ExistsByExternalID(ctx context.Context, externalID, provider string) (bool, error)
}
