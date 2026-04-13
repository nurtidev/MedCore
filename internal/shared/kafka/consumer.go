package kafka

import (
	"context"
	"fmt"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

type Message struct {
	Topic   string
	Key     []byte
	Value   []byte
	Offset  int64
	Partition int32
}

type HandlerFunc func(ctx context.Context, msg Message) error

type Consumer struct {
	consumer *kafka.Consumer
}

// NewConsumer создаёт Kafka consumer с заданной consumer group.
func NewConsumer(brokers, groupID string) (*Consumer, error) {
	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":        brokers,
		"group.id":                 groupID,
		"auto.offset.reset":        "earliest",
		"enable.auto.commit":       false, // коммитим вручную после обработки
		"session.timeout.ms":       30000,
		"heartbeat.interval.ms":    3000,
		"max.poll.interval.ms":     300000,
	})
	if err != nil {
		return nil, fmt.Errorf("kafka.NewConsumer: %w", err)
	}

	return &Consumer{consumer: c}, nil
}

// Subscribe подписывается на список топиков.
func (c *Consumer) Subscribe(topics []string) error {
	if err := c.consumer.SubscribeTopics(topics, nil); err != nil {
		return fmt.Errorf("kafka.Consumer.Subscribe: %w", err)
	}
	return nil
}

// Poll читает одно сообщение с таймаутом (ms).
// Возвращает nil, nil если сообщений нет.
func (c *Consumer) Poll(timeoutMs int) (*Message, error) {
	ev := c.consumer.Poll(timeoutMs)
	if ev == nil {
		return nil, nil
	}

	switch e := ev.(type) {
	case *kafka.Message:
		return &Message{
			Topic:     *e.TopicPartition.Topic,
			Key:       e.Key,
			Value:     e.Value,
			Offset:    int64(e.TopicPartition.Offset),
			Partition: e.TopicPartition.Partition,
		}, nil
	case kafka.Error:
		if e.Code() == kafka.ErrAllBrokersDown {
			return nil, fmt.Errorf("kafka.Consumer.Poll: all brokers down: %w", e)
		}
		// остальные ошибки не фатальны
		return nil, nil
	default:
		return nil, nil
	}
}

// Commit подтверждает текущий offset.
func (c *Consumer) Commit() error {
	if _, err := c.consumer.Commit(); err != nil {
		return fmt.Errorf("kafka.Consumer.Commit: %w", err)
	}
	return nil
}

// Close закрывает consumer.
func (c *Consumer) Close() error {
	if err := c.consumer.Close(); err != nil {
		return fmt.Errorf("kafka.Consumer.Close: %w", err)
	}
	return nil
}
