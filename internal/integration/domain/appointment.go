package domain

import (
	"time"

	"github.com/google/uuid"
)

// ExternalAppointment — запись из внешнего агрегатора (iDoctor).
type ExternalAppointment struct {
	ExternalID       string
	ExternalSource   string     // "idoctor"
	DoctorID         string     // внешний ID врача
	InternalDoctorID *uuid.UUID // маппинг на внутренний ID
	PatientName      string
	PatientPhone     string
	PatientIIN       string
	ServiceName      string
	ScheduledAt      time.Time
	Status           string // "booked", "cancelled", "completed"
	CreatedAt        time.Time
}

// WebhookPayload — входящий payload от агрегатора.
type WebhookPayload struct {
	Provider  string         `json:"provider"`
	EventType string         `json:"event_type"`
	Raw       []byte         `json:"-"`
	Data      map[string]any `json:"data"`
}

// SyncResult — результат синхронизации расписания.
type SyncResult struct {
	Provider   string
	ClinicID   uuid.UUID
	Created    int
	Updated    int
	Failed     int
	StartedAt  time.Time
	FinishedAt time.Time
}
