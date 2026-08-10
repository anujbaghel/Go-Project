package kafkax

import (
	"context"

	"github.com/segmentio/kafka-go"
)

type Producer struct {
	w *kafka.Writer
}

func NewProducer(brokers []string, topic string) *Producer {
	return &Producer{
		w: &kafka.Writer{
			Addr:     kafka.TCP(brokers...),
			Topic:    topic,
			Balancer: &kafka.Hash{},
		},
	}
}

func (p *Producer) Publish(ctx context.Context, key string, value []byte) error {
	return p.w.WriteMessages(ctx, kafka.Message{Key: []byte(key), Value: value})
}

func (p *Producer) Close() error {
	return p.w.Close()
}
