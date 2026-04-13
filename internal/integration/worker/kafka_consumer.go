package worker

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/nurtidev/medcore/internal/integration/service"
	sharedkafka "github.com/nurtidev/medcore/internal/shared/kafka"
	"github.com/rs/zerolog"
)

// KafkaConsumer слушает события из других сервисов.
type KafkaConsumer struct {
	consumer *sharedkafka.Consumer
	svc      service.IntegrationService
	log      zerolog.Logger
}

// NewKafkaConsumer создаёт KafkaConsumer.
func NewKafkaConsumer(consumer *sharedkafka.Consumer, svc service.IntegrationService, log zerolog.Logger) *KafkaConsumer {
	return &KafkaConsumer{
		consumer: consumer,
		svc:      svc,
		log:      log,
	}
}

// Run запускает цикл чтения из Kafka.
func (c *KafkaConsumer) Run(ctx context.Context) {
	if err := c.consumer.Subscribe([]string{"payment.completed"}); err != nil {
		c.log.Error().Err(err).Msg("kafka consumer: subscribe failed")
		return
	}

	c.log.Info().Msg("kafka consumer started")

	for {
		select {
		case <-ctx.Done():
			c.log.Info().Msg("kafka consumer stopped")
			return
		default:
		}

		msg, err := c.consumer.Poll(100)
		if err != nil {
			c.log.Error().Err(err).Msg("kafka consumer: poll error")
			continue
		}
		if msg == nil {
			continue
		}

		if err := c.handleMessage(ctx, msg); err != nil {
			c.log.Error().
				Err(err).
				Str("topic", msg.Topic).
				Msg("kafka consumer: handle message failed")
			continue
		}

		if err := c.consumer.Commit(); err != nil {
			c.log.Error().Err(err).Msg("kafka consumer: commit failed")
		}
	}
}

func (c *KafkaConsumer) handleMessage(ctx context.Context, msg *sharedkafka.Message) error {
	switch msg.Topic {
	case "payment.completed":
		return c.handlePaymentCompleted(ctx, msg.Value)
	default:
		c.log.Warn().Str("topic", msg.Topic).Msg("unknown topic")
		return nil
	}
}

// paymentCompletedEvent — событие о завершении оплаты.
type paymentCompletedEvent struct {
	AppointmentID  string    `json:"appointment_id"`
	ExternalID     string    `json:"external_id"`
	ExternalSource string    `json:"external_source"`
	ClinicID       string    `json:"clinic_id"`
	Amount         float64   `json:"amount"`
	Currency       string    `json:"currency"`
	CompletedAt    time.Time `json:"completed_at"`
}

// handlePaymentCompleted обновляет статус записи в агрегаторе после оплаты.
func (c *KafkaConsumer) handlePaymentCompleted(ctx context.Context, payload []byte) error {
	var event paymentCompletedEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		c.log.Error().Err(err).Msg("payment.completed: unmarshal")
		return nil // не ретраим — неправильный формат
	}

	if event.ExternalSource != "idoctor" || event.ExternalID == "" {
		return nil
	}

	clinicID, err := uuid.Parse(event.ClinicID)
	if err != nil {
		c.log.Warn().Str("clinic_id", event.ClinicID).Msg("payment.completed: invalid clinic_id")
		return nil
	}
	_ = clinicID

	// Синхронизируем статус — в реальном сценарии вызываем UpdateAppointmentStatus
	// через IntegrationService (нужно добавить метод) или напрямую через iDoctor адаптер.
	c.log.Info().
		Str("external_id", event.ExternalID).
		Str("source", event.ExternalSource).
		Msg("payment completed: would update appointment status in aggregator")

	return nil
}
