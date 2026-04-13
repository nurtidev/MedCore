package domain

import (
	"time"

	"github.com/google/uuid"
)

// LabResultFormat — формат результата лаборатории.
type LabResultFormat string

const (
	FormatPDF  LabResultFormat = "pdf"
	FormatJSON LabResultFormat = "json"
	FormatXML  LabResultFormat = "xml"
)

// LabResult — результат анализа от лаборатории.
type LabResult struct {
	ID          uuid.UUID
	ClinicID    uuid.UUID
	PatientID   uuid.UUID
	ExternalID  string
	LabProvider string          // "olymp", "invivo"
	TestName    string
	Format      LabResultFormat
	FileURL     string         // для PDF
	Data        map[string]any // для JSON/XML
	ReceivedAt  time.Time
	AttachedAt  *time.Time // когда прикреплено к карте
}

// IntegrationConfig — конфигурация интеграции для клиники.
type IntegrationConfig struct {
	ID        uuid.UUID
	ClinicID  uuid.UUID
	Provider  string
	IsActive  bool
	Config    map[string]any // зашифрованные API ключи, URL
	CreatedAt time.Time
	UpdatedAt time.Time
}

// SyncLog — запись о синхронизации.
type SyncLog struct {
	ID               uuid.UUID
	ClinicID         uuid.UUID
	Provider         string
	Operation        string
	Status           string // "success", "failed", "partial"
	RecordsProcessed int
	ErrorMessage     string
	StartedAt        time.Time
	CompletedAt      *time.Time
	CreatedAt        time.Time
}

// UpsertConfigRequest — запрос на создание/обновление конфига интеграции.
type UpsertConfigRequest struct {
	ClinicID uuid.UUID
	Provider string
	IsActive bool
	Config   map[string]any
}
