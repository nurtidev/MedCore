package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nurtidev/medcore/internal/integration/adapter"
	"github.com/nurtidev/medcore/internal/integration/domain"
	"github.com/nurtidev/medcore/internal/integration/repository"
	"github.com/nurtidev/medcore/internal/shared/kafka"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

const (
	cacheIINTTL       = 24 * time.Hour
	cacheStatusTTL    = time.Hour
	cacheMappingTTL   = 6 * time.Hour
	cacheConfigTTL    = 5 * time.Minute
)

// IntegrationService defines all integration operations.
type IntegrationService interface {
	// ГосAPI
	ValidateIIN(ctx context.Context, iin string) (*domain.PatientInfo, error)
	GetPatientStatus(ctx context.Context, iin string) (string, error)

	// Агрегаторы
	SyncAppointments(ctx context.Context, clinicID uuid.UUID) (*domain.SyncResult, error)
	HandleIncomingAppointment(ctx context.Context, payload domain.WebhookPayload) error

	// Лаборатории
	FetchLabResults(ctx context.Context, clinicID uuid.UUID) ([]*domain.LabResult, error)
	AttachResultToPatient(ctx context.Context, resultID uuid.UUID, patientID uuid.UUID) error
	HandleLabWebhook(ctx context.Context, provider string, payload []byte) error

	// Конфиги
	GetIntegrationConfig(ctx context.Context, clinicID uuid.UUID, provider string) (*domain.IntegrationConfig, error)
	UpsertIntegrationConfig(ctx context.Context, req domain.UpsertConfigRequest) error
}

// Deps объединяет все зависимости сервиса.
type Deps struct {
	SyncRepo    repository.SyncRepository
	LabRepo     repository.LabResultRepository
	EgovAdapter adapter.GovAPIAdapter
	IDoctorAdap adapter.AggregatorAdapter
	OlympAdap   adapter.LaboratoryAdapter
	InvivoAdap  adapter.LaboratoryAdapter
	Redis       *redis.Client
	Kafka       *kafka.Producer
	Log         zerolog.Logger
}

type integrationService struct {
	syncRepo    repository.SyncRepository
	labRepo     repository.LabResultRepository
	egov        adapter.GovAPIAdapter
	iDoctor     adapter.AggregatorAdapter
	olymp       adapter.LaboratoryAdapter
	invivo      adapter.LaboratoryAdapter
	redis       *redis.Client
	kafka       *kafka.Producer
	log         zerolog.Logger
}

// New creates a new IntegrationService.
func New(d Deps) IntegrationService {
	return &integrationService{
		syncRepo: d.SyncRepo,
		labRepo:  d.LabRepo,
		egov:     d.EgovAdapter,
		iDoctor:  d.IDoctorAdap,
		olymp:    d.OlympAdap,
		invivo:   d.InvivoAdap,
		redis:    d.Redis,
		kafka:    d.Kafka,
		log:      d.Log,
	}
}

// ── ГосAPI ────────────────────────────────────────────────────────────────────

func (s *integrationService) ValidateIIN(ctx context.Context, iin string) (*domain.PatientInfo, error) {
	cacheKey := fmt.Sprintf("integration:egov:iin:%s", iin)

	// cache hit
	if s.redis != nil {
		cached, err := s.redis.Get(ctx, cacheKey).Bytes()
		if err == nil {
			var info domain.PatientInfo
			if json.Unmarshal(cached, &info) == nil {
				return &info, nil
			}
		}
	}

	start := time.Now()
	info, err := s.egov.ValidateIIN(ctx, iin)
	s.log.Info().
		Str("provider", "egov").
		Str("operation", "validate_iin").
		Dur("duration", time.Since(start)).
		Err(err).
		Msg("external call")

	if err != nil {
		return nil, fmt.Errorf("integration.ValidateIIN: %w", err)
	}

	// cache set
	if s.redis != nil {
		if b, err := json.Marshal(info); err == nil {
			_ = s.redis.Set(ctx, cacheKey, b, cacheIINTTL).Err()
		}
	}

	return info, nil
}

func (s *integrationService) GetPatientStatus(ctx context.Context, iin string) (string, error) {
	cacheKey := fmt.Sprintf("integration:egov:status:%s", iin)

	if s.redis != nil {
		if cached, err := s.redis.Get(ctx, cacheKey).Result(); err == nil {
			return cached, nil
		}
	}

	start := time.Now()
	status, err := s.egov.GetPatientStatus(ctx, iin)
	s.log.Info().
		Str("provider", "egov").
		Str("operation", "get_patient_status").
		Dur("duration", time.Since(start)).
		Err(err).
		Msg("external call")

	if err != nil {
		return "", fmt.Errorf("integration.GetPatientStatus: %w", err)
	}

	if s.redis != nil {
		_ = s.redis.Set(ctx, cacheKey, status, cacheStatusTTL).Err()
	}

	return status, nil
}

// ── Агрегаторы ────────────────────────────────────────────────────────────────

func (s *integrationService) SyncAppointments(ctx context.Context, clinicID uuid.UUID) (*domain.SyncResult, error) {
	cfg, err := s.syncRepo.GetIntegrationConfig(ctx, clinicID, "idoctor")
	if err != nil {
		return nil, fmt.Errorf("integration.SyncAppointments: get config: %w", err)
	}
	if !cfg.IsActive {
		return nil, domain.ErrIntegrationNotConfigured
	}

	syncLog := &domain.SyncLog{
		ClinicID:  clinicID,
		Provider:  "idoctor",
		Operation: "sync_appointments",
		Status:    "running",
		StartedAt: time.Now(),
	}
	_ = s.syncRepo.CreateSyncLog(ctx, syncLog)

	result := &domain.SyncResult{
		Provider:  "idoctor",
		ClinicID:  clinicID,
		StartedAt: syncLog.StartedAt,
	}

	// Маппинг врачей (кэшируем)
	mappingKey := fmt.Sprintf("integration:idoctor:mapping:%s", clinicID)
	doctorMapping := make(map[string]uuid.UUID)
	if s.redis != nil {
		if cached, err := s.redis.Get(ctx, mappingKey).Bytes(); err == nil {
			_ = json.Unmarshal(cached, &doctorMapping)
		}
	}
	if len(doctorMapping) == 0 {
		doctorMapping, err = s.iDoctor.GetDoctorMapping(ctx, clinicID.String())
		if err != nil {
			s.log.Warn().Err(err).Msg("failed to get doctor mapping")
		} else if s.redis != nil {
			if b, err := json.Marshal(doctorMapping); err == nil {
				_ = s.redis.Set(ctx, mappingKey, b, cacheMappingTTL).Err()
			}
		}
	}

	since := time.Now().Add(-30 * time.Second)
	appointments, err := s.iDoctor.GetNewAppointments(ctx, clinicID.String(), since)
	if err != nil {
		syncLog.Status = "failed"
		syncLog.ErrorMessage = err.Error()
		now := time.Now()
		syncLog.CompletedAt = &now
		_ = s.syncRepo.UpdateSyncLog(ctx, syncLog)

		// DLQ
		s.publishDLQ(ctx, "idoctor", "sync_appointments", nil, err, 3)
		return nil, fmt.Errorf("integration.SyncAppointments: %w", err)
	}

	for _, appt := range appointments {
		if internalID, ok := doctorMapping[appt.DoctorID]; ok {
			appt.InternalDoctorID = &internalID
		}

		event := map[string]any{
			"clinic_id":   clinicID,
			"appointment": appt,
		}
		if s.kafka != nil {
			if err := s.kafka.Publish(ctx, "integration.appointment.created", clinicID.String(), event); err != nil {
				s.log.Error().Err(err).Msg("failed to publish appointment event")
				result.Failed++
				continue
			}
		}

		if err := s.iDoctor.UpdateAppointmentStatus(ctx, appt.ExternalID, "synced"); err != nil {
			s.log.Warn().Err(err).Str("appointment_id", appt.ExternalID).Msg("failed to update appointment status")
		}
		result.Created++
	}

	now := time.Now()
	result.FinishedAt = now
	syncLog.Status = "success"
	if result.Failed > 0 {
		syncLog.Status = "partial"
	}
	syncLog.RecordsProcessed = result.Created + result.Updated + result.Failed
	syncLog.CompletedAt = &now
	_ = s.syncRepo.UpdateSyncLog(ctx, syncLog)

	return result, nil
}

func (s *integrationService) HandleIncomingAppointment(ctx context.Context, payload domain.WebhookPayload) error {
	// Немедленно публикуем в Kafka для асинхронной обработки
	if s.kafka != nil {
		event := map[string]any{
			"provider":   payload.Provider,
			"event_type": payload.EventType,
			"data":       payload.Data,
		}
		if err := s.kafka.Publish(ctx, "integration.appointment.created", payload.Provider, event); err != nil {
			return fmt.Errorf("integration.HandleIncomingAppointment: publish: %w", err)
		}
	}
	return nil
}

// ── Лаборатории ───────────────────────────────────────────────────────────────

func (s *integrationService) FetchLabResults(ctx context.Context, clinicID uuid.UUID) ([]*domain.LabResult, error) {
	var allResults []*domain.LabResult

	// Олимп
	olympResults, err := s.olymp.GetPendingResults(ctx, clinicID.String())
	if err != nil {
		s.log.Error().Err(err).Str("provider", "olymp").Msg("fetch lab results failed")
	} else {
		for _, r := range olympResults {
			exists, _ := s.labRepo.ExistsByExternalID(ctx, r.ExternalID, r.LabProvider)
			if exists {
				continue
			}
			r.ClinicID = clinicID
			if err := s.labRepo.Create(ctx, r); err != nil {
				s.log.Error().Err(err).Msg("save olymp result")
				continue
			}
			_ = s.olymp.AcknowledgeResult(ctx, r.ExternalID)
			if s.kafka != nil {
				_ = s.kafka.Publish(ctx, "integration.lab_result.received", clinicID.String(), r)
			}
			allResults = append(allResults, r)
		}
	}

	// Инвиво
	invivoResults, err := s.invivo.GetPendingResults(ctx, clinicID.String())
	if err != nil {
		s.log.Error().Err(err).Str("provider", "invivo").Msg("fetch lab results failed")
	} else {
		for _, r := range invivoResults {
			exists, _ := s.labRepo.ExistsByExternalID(ctx, r.ExternalID, r.LabProvider)
			if exists {
				continue
			}
			r.ClinicID = clinicID
			if err := s.labRepo.Create(ctx, r); err != nil {
				s.log.Error().Err(err).Msg("save invivo result")
				continue
			}
			_ = s.invivo.AcknowledgeResult(ctx, r.ExternalID)
			if s.kafka != nil {
				_ = s.kafka.Publish(ctx, "integration.lab_result.received", clinicID.String(), r)
			}
			allResults = append(allResults, r)
		}
	}

	return allResults, nil
}

func (s *integrationService) AttachResultToPatient(ctx context.Context, resultID uuid.UUID, patientID uuid.UUID) error {
	if err := s.labRepo.AttachToPatient(ctx, resultID, patientID); err != nil {
		return fmt.Errorf("integration.AttachResultToPatient: %w", err)
	}
	return nil
}

func (s *integrationService) HandleLabWebhook(ctx context.Context, provider string, payload []byte) error {
	// Сигнатура уже проверена в handler/webhook.go
	// Публикуем асинхронно — ответ на webhook 200 OK уже отправлен
	if s.kafka != nil {
		event := map[string]any{
			"provider": provider,
			"payload":  string(payload),
		}
		if err := s.kafka.Publish(ctx, "integration.lab_result.received", provider, event); err != nil {
			return fmt.Errorf("integration.HandleLabWebhook: publish: %w", err)
		}
	}
	return nil
}

// ── Конфиги ───────────────────────────────────────────────────────────────────

func (s *integrationService) GetIntegrationConfig(ctx context.Context, clinicID uuid.UUID, provider string) (*domain.IntegrationConfig, error) {
	cacheKey := fmt.Sprintf("integration:config:%s:%s", clinicID, provider)

	if s.redis != nil {
		if cached, err := s.redis.Get(ctx, cacheKey).Bytes(); err == nil {
			var cfg domain.IntegrationConfig
			if json.Unmarshal(cached, &cfg) == nil {
				return &cfg, nil
			}
		}
	}

	cfg, err := s.syncRepo.GetIntegrationConfig(ctx, clinicID, provider)
	if err != nil {
		return nil, fmt.Errorf("integration.GetIntegrationConfig: %w", err)
	}

	if s.redis != nil {
		if b, err := json.Marshal(cfg); err == nil {
			_ = s.redis.Set(ctx, cacheKey, b, cacheConfigTTL).Err()
		}
	}

	return cfg, nil
}

func (s *integrationService) UpsertIntegrationConfig(ctx context.Context, req domain.UpsertConfigRequest) error {
	cfg := &domain.IntegrationConfig{
		ClinicID: req.ClinicID,
		Provider: req.Provider,
		IsActive: req.IsActive,
		Config:   req.Config,
	}
	if err := s.syncRepo.UpsertIntegrationConfig(ctx, cfg); err != nil {
		return fmt.Errorf("integration.UpsertIntegrationConfig: %w", err)
	}

	// инвалидируем кэш
	if s.redis != nil {
		cacheKey := fmt.Sprintf("integration:config:%s:%s", req.ClinicID, req.Provider)
		_ = s.redis.Del(ctx, cacheKey).Err()
	}

	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

type dlqMessage struct {
	Provider  string `json:"provider"`
	Operation string `json:"operation"`
	Payload   any    `json:"payload"`
	Error     string `json:"error"`
	Attempts  int    `json:"attempts"`
	FailedAt  string `json:"failed_at"`
}

func (s *integrationService) publishDLQ(ctx context.Context, provider, operation string, payload any, err error, attempts int) {
	if s.kafka == nil {
		return
	}
	msg := dlqMessage{
		Provider:  provider,
		Operation: operation,
		Payload:   payload,
		Error:     err.Error(),
		Attempts:  attempts,
		FailedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	if pubErr := s.kafka.Publish(ctx, "integration.failed", provider, msg); pubErr != nil {
		s.log.Error().Err(pubErr).Msg("failed to publish DLQ message")
	}
}
