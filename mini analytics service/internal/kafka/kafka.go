package kafka

import (
	"time"

	"github.com/segmentio/kafka-go"
)

// NewWriter returns a kafka-go writer with sensible defaults for this project.
func NewWriter(brokers []string, topic string) *kafka.Writer {
	return &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        topic,
		Balancer:     &kafka.Hash{},
		RequiredAcks: kafka.RequireOne,
		Async:        false,
		BatchTimeout: 250 * time.Millisecond,
		BatchSize:    1,
	}
}

// NewReader constructs a reader bound to a consumer group.
func NewReader(brokers []string, topic, group string) *kafka.Reader {
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:         brokers,
		Topic:           topic,
		GroupID:         group,
		MinBytes:        1e4,
		MaxBytes:        10e6,
		StartOffset:     kafka.LastOffset,
		CommitInterval:  time.Second,
		ReadLagInterval: 5 * time.Second,
		MaxWait:         time.Second,
	})
}
