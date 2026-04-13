package worker_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nurtidev/medcore/internal/analytics/domain"
	"github.com/nurtidev/medcore/internal/analytics/worker"
	sharedkafka "github.com/nurtidev/medcore/internal/shared/kafka"
)

// ─── Mock recorder ────────────────────────────────────────────────────────────

type mockRecorder struct{ mock.Mock }

func (m *mockRecorder) RecordEventBatch(ctx context.Context, events []*domain.ClinicEvent) error {
	return m.Called(ctx, events).Error(0)
}

// ─── fakeConsumer — drives the consumer without a real Kafka broker ───────────

type fakeConsumer struct {
	msgs    []*sharedkafka.Message
	pos     int
	commits int
}

func (f *fakeConsumer) Poll(_ int) (*sharedkafka.Message, error) {
	if f.pos >= len(f.msgs) {
		return nil, nil
	}
	m := f.msgs[f.pos]
	f.pos++
	return m, nil
}

func (f *fakeConsumer) Commit() error {
	f.commits++
	return nil
}

// testableConsumer wraps KafkaConsumer to expose Run with a fakeConsumer.
// Since KafkaConsumer.Run calls c.consumer.Poll/Commit, we mirror that logic.
func runConsumerWithMessages(
	t *testing.T,
	msgs []*sharedkafka.Message,
	recorder worker.EventRecorder,
	batchSize int,
	flushInterval time.Duration,
) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Build a real KafkaConsumer but swap the internal Poll/Commit using a helper
	// that directly calls the parseMessage + recorder logic (white-box via exported method).
	_ = worker.RunWithFakeConsumer(ctx, msgs, recorder, zerolog.Nop(), batchSize, flushInterval)
}

// ─── Tests ────────────────────────────────────────────────────────────────────

func TestKafkaConsumer_PaymentCompleted_RecordsEvent(t *testing.T) {
	clinicID := uuid.New()
	payload, _ := json.Marshal(map[string]any{
		"event_id":   uuid.New().String(),
		"clinic_id":  clinicID.String(),
		"doctor_id":  uuid.New().String(),
		"patient_id": uuid.New().String(),
		"amount":     15000.0,
		"currency":   "KZT",
		"created_at": time.Now().UTC().Format(time.RFC3339),
	})

	msgs := []*sharedkafka.Message{
		{Topic: "payment.completed", Value: payload},
	}

	rec := &mockRecorder{}
	rec.On("RecordEventBatch", mock.Anything, mock.MatchedBy(func(events []*domain.ClinicEvent) bool {
		require.Len(t, events, 1)
		e := events[0]
		assert.Equal(t, domain.EventPaymentCompleted, e.EventType)
		assert.Equal(t, clinicID.String(), e.ClinicID)
		assert.InDelta(t, 15000.0, e.Amount, 0.001)
		assert.Equal(t, "KZT", e.Currency)
		return true
	})).Return(nil)

	runConsumerWithMessages(t, msgs, rec, 100, 50*time.Millisecond)
	rec.AssertExpectations(t)
}

func TestKafkaConsumer_BatchFlush_On100Events(t *testing.T) {
	msgs := make([]*sharedkafka.Message, 100)
	for i := range msgs {
		payload, _ := json.Marshal(map[string]any{
			"event_id":   uuid.New().String(),
			"clinic_id":  uuid.New().String(),
			"doctor_id":  uuid.New().String(),
			"patient_id": uuid.New().String(),
			"amount":     1000.0,
			"currency":   "KZT",
			"created_at": time.Now().UTC().Format(time.RFC3339),
		})
		msgs[i] = &sharedkafka.Message{Topic: "payment.completed", Value: payload}
	}

	rec := &mockRecorder{}
	rec.On("RecordEventBatch", mock.Anything, mock.MatchedBy(func(events []*domain.ClinicEvent) bool {
		return len(events) == 100
	})).Return(nil).Once()

	runConsumerWithMessages(t, msgs, rec, 100, time.Minute) // long flush interval → only size trigger
	rec.AssertExpectations(t)
}

func TestKafkaConsumer_BatchFlush_OnTimeout(t *testing.T) {
	// 3 events, batch size = 100 → flush triggered only by timeout.
	msgs := make([]*sharedkafka.Message, 3)
	for i := range msgs {
		payload, _ := json.Marshal(map[string]any{
			"event_id":   uuid.New().String(),
			"clinic_id":  uuid.New().String(),
			"amount":     500.0,
			"currency":   "KZT",
			"created_at": time.Now().UTC().Format(time.RFC3339),
		})
		msgs[i] = &sharedkafka.Message{Topic: "payment.completed", Value: payload}
	}

	rec := &mockRecorder{}
	rec.On("RecordEventBatch", mock.Anything, mock.MatchedBy(func(events []*domain.ClinicEvent) bool {
		return len(events) == 3
	})).Return(nil).Once()

	runConsumerWithMessages(t, msgs, rec, 100, 50*time.Millisecond) // short interval → timeout flush
	rec.AssertExpectations(t)
}
