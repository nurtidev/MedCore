package worker

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nurtidev/medcore/internal/integration/repository"
	"github.com/nurtidev/medcore/internal/integration/service"
	"github.com/rs/zerolog"
)

// SyncWorker запускает фоновую синхронизацию расписания каждые 30 секунд.
type SyncWorker struct {
	svc      service.IntegrationService
	syncRepo repository.SyncRepository
	interval time.Duration
	log      zerolog.Logger
}

// NewSyncWorker создаёт нового SyncWorker.
func NewSyncWorker(svc service.IntegrationService, syncRepo repository.SyncRepository, interval time.Duration, log zerolog.Logger) *SyncWorker {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &SyncWorker{
		svc:      svc,
		syncRepo: syncRepo,
		interval: interval,
		log:      log,
	}
}

// Run запускает рабочий цикл и блокируется до отмены контекста.
func (w *SyncWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.log.Info().Dur("interval", w.interval).Msg("sync worker started")

	for {
		select {
		case <-ctx.Done():
			w.log.Info().Msg("sync worker stopped")
			return
		case <-ticker.C:
			w.runCycle(ctx)
		}
	}
}

func (w *SyncWorker) runCycle(ctx context.Context) {
	start := time.Now()

	// Получить все активные клиники с настроенным iDoctor
	clinics, err := w.getActiveClinics(ctx)
	if err != nil {
		w.log.Error().Err(err).Msg("sync worker: get active clinics")
		return
	}

	for _, clinicID := range clinics {
		cycleStart := time.Now()

		result, err := w.svc.SyncAppointments(ctx, clinicID)
		elapsed := time.Since(cycleStart)

		if err != nil {
			w.log.Error().
				Err(err).
				Str("clinic_id", clinicID.String()).
				Msg("sync worker: sync appointments failed")
			continue
		}

		w.log.Info().
			Str("clinic_id", clinicID.String()).
			Int("created", result.Created).
			Int("updated", result.Updated).
			Int("failed", result.Failed).
			Dur("elapsed", elapsed).
			Msg("sync cycle completed")

		// Требование ТЗ: время цикла ≤ 1 секунды
		if elapsed > time.Second {
			w.log.Warn().
				Str("clinic_id", clinicID.String()).
				Dur("elapsed", elapsed).
				Msg("sync cycle exceeded 1s SLA")
		}
	}

	w.log.Debug().
		Dur("total_elapsed", time.Since(start)).
		Int("clinics", len(clinics)).
		Msg("sync worker cycle done")
}

// getActiveClinics возвращает список clinic_id с активной интеграцией iDoctor.
func (w *SyncWorker) getActiveClinics(ctx context.Context) ([]uuid.UUID, error) {
	// Реальная реализация должна делать запрос к БД:
	// SELECT DISTINCT clinic_id FROM integration_configs WHERE provider='idoctor' AND is_active=true
	// Здесь используем SyncRepository для минимальной зависимости.
	// В production добавить метод ListActiveClinicsByProvider в SyncRepository.
	//
	// Placeholder: для демонстрации возвращаем пустой список.
	// Конкретный запрос добавляется при подключении к реальной БД.
	return []uuid.UUID{}, nil
}
