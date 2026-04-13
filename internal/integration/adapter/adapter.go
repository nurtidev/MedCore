package adapter

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nurtidev/medcore/internal/integration/domain"
)

// GovAPIAdapter — интерфейс для работы с государственными API (eGov, DAMUMED).
type GovAPIAdapter interface {
	ValidateIIN(ctx context.Context, iin string) (*domain.PatientInfo, error)
	GetPatientStatus(ctx context.Context, iin string) (string, error)
}

// AggregatorAdapter — интерфейс для агрегаторов записей (iDoctor).
type AggregatorAdapter interface {
	GetNewAppointments(ctx context.Context, clinicID string, since time.Time) ([]*domain.ExternalAppointment, error)
	UpdateAppointmentStatus(ctx context.Context, externalID, status string) error
	GetDoctorMapping(ctx context.Context, clinicID string) (map[string]uuid.UUID, error)
}

// LaboratoryAdapter — интерфейс для лабораторий (Олимп, Инвиво).
type LaboratoryAdapter interface {
	GetPendingResults(ctx context.Context, clinicID string) ([]*domain.LabResult, error)
	AcknowledgeResult(ctx context.Context, externalID string) error
}
