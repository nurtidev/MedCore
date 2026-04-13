package kafka

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

type Producer struct {
	producer *kafka.Producer
}

// NewProducer создаёт Kafka producer.
func NewProducer(brokers string) (*Producer, error) {
	p, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers":            brokers,
		"acks":                         "all",  // ждём подтверждения от всех реплик
		"retries":                      3,
		"retry.backoff.ms":             100,
		"enable.idempotence":           true,
		"max.in.flight.requests.per.connection": 5,
	})
	if err != nil {
		return nil, fmt.Errorf("kafka.NewProducer: %w", err)
	}

	return &Producer{producer: p}, nil
}

// Publish сериализует событие в JSON и отправляет в топик.
func (p *Producer) Publish(ctx context.Context, topic string, key string, event any) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("kafka.Producer.Publish: marshal: %w", err)
	}

	deliveryChan := make(chan kafka.Event, 1)

	err = p.producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{
			Topic:     &topic,
			Partition: kafka.PartitionAny,
		},
		Key:   []byte(key),
		Value: payload,
	}, deliveryChan)
	if err != nil {
		return fmt.Errorf("kafka.Producer.Publish: produce: %w", err)
	}

	select {
	case e := <-deliveryChan:
		msg, ok := e.(*kafka.Message)
		if !ok {
			return fmt.Errorf("kafka.Producer.Publish: unexpected event type")
		}
		if msg.TopicPartition.Error != nil {
			return fmt.Errorf("kafka.Producer.Publish: delivery: %w", msg.TopicPartition.Error)
		}
	case <-ctx.Done():
		return fmt.Errorf("kafka.Producer.Publish: context cancelled: %w", ctx.Err())
	}

	return nil
}

// Close завершает работу producer, дожидаясь отправки всех сообщений.
func (p *Producer) Close() {
	p.producer.Flush(5000)
	p.producer.Close()
}
