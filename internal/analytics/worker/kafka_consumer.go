package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog"

	"github.com/nurtidev/medcore/internal/analytics/domain"
	sharedkafka "github.com/nurtidev/medcore/internal/shared/kafka"
)

// ─── Prometheus metrics ───────────────────────────────────────────────────────

var consumerLag = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "analytics_kafka_consumer_lag",
	Help: "Approximate consumer lag (messages behind).",
})

// ─── Kafka payload shapes ─────────────────────────────────────────────────────

// paymentEvent is the JSON shape published by billing-service.
type paymentEvent struct {
	EventID   string    `json:"event_id"`
	ClinicID  string    `json:"clinic_id"`
	DoctorID  string    `json:"doctor_id"`
	PatientID string    `json:"patient_id"`
	Amount    float64   `json:"amount"`
	Currency  string    `json:"currency"`
	CreatedAt time.Time `json:"created_at"`
	Metadata  string    `json:"metadata"`
}

// appointmentEvent is the JSON shape published by integration-service.
type appointmentEvent struct {
	EventID       string    `json:"event_id"`
	ClinicID      string    `json:"clinic_id"`
	DoctorID      string    `json:"doctor_id"`
	PatientID     string    `json:"patient_id"`
	CreatedAt     time.Time `json:"created_at"`
	Metadata      string    `json:"metadata"`
}

// labResultEvent is the JSON shape for lab results.
type labResultEvent struct {
	EventID   string    `json:"event_id"`
	ClinicID  string    `json:"clinic_id"`
	PatientID string    `json:"patient_id"`
	CreatedAt time.Time `json:"created_at"`
	Metadata  string    `json:"metadata"`
}

// loginEvent is published by auth-service audit log.
type loginEvent struct {
	EventID   string    `json:"event_id"`
	ClinicID  string    `json:"clinic_id"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

// ─── KafkaConsumer ────────────────────────────────────────────────────────────

// EventRecorder is the subset of domain.AnalyticsService used by the consumer.
type EventRecorder interface {
	RecordEventBatch(ctx context.Context, events []*domain.ClinicEvent) error
}

type KafkaConsumer struct {
	consumer      *sharedkafka.Consumer
	recorder      EventRecorder
	log           zerolog.Logger
	batchSize     int
	flushInterval time.Duration
}

// NewKafkaConsumer creates a consumer that reads from the configured topics and
// batch-inserts events into ClickHouse.
func NewKafkaConsumer(
	consumer *sharedkafka.Consumer,
	recorder EventRecorder,
	log zerolog.Logger,
	batchSize int,
	flushInterval time.Duration,
) *KafkaConsumer {
	return &KafkaConsumer{
		consumer:      consumer,
		recorder:      recorder,
		log:           log,
		batchSize:     batchSize,
		flushInterval: flushInterval,
	}
}

// Run starts the consume loop. It blocks until ctx is cancelled.
// Batch is flushed when it reaches batchSize OR flushInterval elapses — whichever comes first.
// Kafka offset is committed only after a successful ClickHouse insert.
func (c *KafkaConsumer) Run(ctx context.Context) error {
	batch := make([]*domain.ClinicEvent, 0, c.batchSize)
	ticker := time.NewTicker(c.flushInterval)
	defer ticker.Stop()

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		if err := c.recorder.RecordEventBatch(ctx, batch); err != nil {
			return fmt.Errorf("KafkaConsumer.flush: record batch: %w", err)
		}
		if err := c.consumer.Commit(); err != nil {
			return fmt.Errorf("KafkaConsumer.flush: commit: %w", err)
		}
		c.log.Info().Int("count", len(batch)).Msg("kafka batch flushed")
		batch = batch[:0]
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			// Drain remaining events on shutdown.
			if err := flush(); err != nil {
				c.log.Error().Err(err).Msg("shutdown flush failed")
			}
			return ctx.Err()

		case <-ticker.C:
			if err := flush(); err != nil {
				// Don't commit offset — message will be re-delivered.
				c.log.Error().Err(err).Msg("timed flush failed, retrying next interval")
			}
			consumerLag.Set(0) // reset lag gauge after successful flush

		default:
			msg, err := c.consumer.Poll(100) // 100 ms poll timeout
			if err != nil {
				c.log.Error().Err(err).Msg("kafka poll error")
				continue
			}
			if msg == nil {
				continue
			}

			event, err := c.parseMessage(msg)
			if err != nil {
				c.log.Warn().Err(err).Str("topic", msg.Topic).Msg("skip unparseable message")
				continue
			}

			batch = append(batch, event)
			consumerLag.Inc()

			if len(batch) >= c.batchSize {
				if err := flush(); err != nil {
					c.log.Error().Err(err).Msg("size-based flush failed, retrying next poll")
				}
			}
		}
	}
}

// parseMessage maps a raw Kafka message to a ClinicEvent based on topic name.
func (c *KafkaConsumer) parseMessage(msg *sharedkafka.Message) (*domain.ClinicEvent, error) {
	switch msg.Topic {
	case "payment.completed":
		return parsePayment(msg.Value, domain.EventPaymentCompleted)
	case "payment.failed":
		return parsePayment(msg.Value, domain.EventPaymentFailed)
	case "integration.appointment.created":
		return parseAppointment(msg.Value, domain.EventAppointmentCreated)
	case "integration.lab_result.received":
		return parseLabResult(msg.Value)
	case "user.login":
		return parseLogin(msg.Value)
	default:
		return nil, fmt.Errorf("unknown topic: %s", msg.Topic)
	}
}

// ─── Test helper ─────────────────────────────────────────────────────────────

// RunWithFakeConsumer drives the consumer batch logic using a pre-built slice of
// messages instead of a real Kafka broker. Intended for unit tests only.
func RunWithFakeConsumer(
	ctx context.Context,
	msgs []*sharedkafka.Message,
	recorder EventRecorder,
	log zerolog.Logger,
	batchSize int,
	flushInterval time.Duration,
) error {
	batch := make([]*domain.ClinicEvent, 0, batchSize)
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	pos := 0

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		if err := recorder.RecordEventBatch(ctx, batch); err != nil {
			return err
		}
		batch = batch[:0]
		return nil
	}

	// helper consumer to reuse parseMessage
	helper := &KafkaConsumer{log: log}

	for {
		select {
		case <-ctx.Done():
			return flush()
		case <-ticker.C:
			if err := flush(); err != nil {
				log.Error().Err(err).Msg("fake consumer: flush error")
			}
			return nil
		default:
			if pos >= len(msgs) {
				// No more messages — wait for timeout to flush.
				time.Sleep(5 * time.Millisecond)
				continue
			}
			msg := msgs[pos]
			pos++

			event, err := helper.parseMessage(msg)
			if err != nil {
				log.Warn().Err(err).Msg("fake consumer: skip message")
				continue
			}
			batch = append(batch, event)
			if len(batch) >= batchSize {
				if err := flush(); err != nil {
					log.Error().Err(err).Msg("fake consumer: size flush error")
				}
				return nil
			}
		}
	}
}

// ─── parsers ──────────────────────────────────────────────────────────────────

func parsePayment(raw []byte, eventType domain.EventType) (*domain.ClinicEvent, error) {
	var p paymentEvent
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("parsePayment: %w", err)
	}
	if p.EventID == "" {
		p.EventID = uuid.New().String()
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now().UTC()
	}
	return &domain.ClinicEvent{
		EventID:   p.EventID,
		ClinicID:  p.ClinicID,
		DoctorID:  p.DoctorID,
		PatientID: p.PatientID,
		EventType: eventType,
		Amount:    p.Amount,
		Currency:  p.Currency,
		CreatedAt: p.CreatedAt,
		Metadata:  p.Metadata,
	}, nil
}

func parseAppointment(raw []byte, eventType domain.EventType) (*domain.ClinicEvent, error) {
	var a appointmentEvent
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, fmt.Errorf("parseAppointment: %w", err)
	}
	if a.EventID == "" {
		a.EventID = uuid.New().String()
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now().UTC()
	}
	return &domain.ClinicEvent{
		EventID:   a.EventID,
		ClinicID:  a.ClinicID,
		DoctorID:  a.DoctorID,
		PatientID: a.PatientID,
		EventType: eventType,
		CreatedAt: a.CreatedAt,
		Metadata:  a.Metadata,
	}, nil
}

func parseLabResult(raw []byte) (*domain.ClinicEvent, error) {
	var l labResultEvent
	if err := json.Unmarshal(raw, &l); err != nil {
		return nil, fmt.Errorf("parseLabResult: %w", err)
	}
	if l.EventID == "" {
		l.EventID = uuid.New().String()
	}
	if l.CreatedAt.IsZero() {
		l.CreatedAt = time.Now().UTC()
	}
	return &domain.ClinicEvent{
		EventID:   l.EventID,
		ClinicID:  l.ClinicID,
		PatientID: l.PatientID,
		EventType: domain.EventLabResultReceived,
		CreatedAt: l.CreatedAt,
		Metadata:  l.Metadata,
	}, nil
}

func parseLogin(raw []byte) (*domain.ClinicEvent, error) {
	var l loginEvent
	if err := json.Unmarshal(raw, &l); err != nil {
		return nil, fmt.Errorf("parseLogin: %w", err)
	}
	if l.EventID == "" {
		l.EventID = uuid.New().String()
	}
	if l.CreatedAt.IsZero() {
		l.CreatedAt = time.Now().UTC()
	}
	return &domain.ClinicEvent{
		EventID:   l.EventID,
		ClinicID:  l.ClinicID,
		DoctorID:  l.UserID,
		EventType: domain.EventUserLogin,
		CreatedAt: l.CreatedAt,
	}, nil
}
