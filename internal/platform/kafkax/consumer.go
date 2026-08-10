package kafkax

import (
	"context"

	"github.com/segmentio/kafka-go"
)

type Consumer struct {
	r *kafka.Reader
}

func NewConsumer(brokers []string, topic string, groupId string) *Consumer {
	return &Consumer{
		r: kafka.NewReader(kafka.ReaderConfig{
			Brokers: brokers,
			Topic:   topic,
			GroupID: groupId,
		}),
	}
}

func (c *Consumer) ReadMessage(ctx context.Context) (kafka.Message, error) {
	return c.r.ReadMessage(ctx)
}

func (c *Consumer) Close() error {
	return c.r.Close()
}
