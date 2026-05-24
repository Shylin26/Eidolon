package kafka

import (
	"context"
	"fmt"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"go.uber.org/zap"
)

type Message struct {
	Topic string
	Key   []byte
	Value string
}

type Consumer struct {
	c      *kafka.Consumer
	logger *zap.Logger
}

func NewConsumer(bootstrapServers, groupID string, topics []string, logger *zap.Logger) (*Consumer, error) {
	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":       bootstrapServers,
		"group.id":                groupID,
		"auto.offset.reset":       "latest",
		"enable.auto.commit":      true,
		"broker.address.family":   "v4",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer: %w", err)
	}

	if err := c.SubscribeTopics(topics, nil); err != nil {
		return nil, fmt.Errorf("failed to subscribe to topics %v: %w", topics, err)
	}

	logger.Info("kafka consumer ready",
		zap.Strings("topics", topics),
		zap.String("group", groupID),
	)
	return &Consumer{c: c, logger: logger}, nil
}

func (c *Consumer) Poll(ctx context.Context, handler func(Message) error) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			ev := c.c.Poll(100)
			if ev == nil {
				continue
			}
			switch e := ev.(type) {
			case *kafka.Message:
				msg := Message{
					Topic: *e.TopicPartition.Topic,
					Key:   e.Key,
					Value: string(e.Value),
				}
				if err := handler(msg); err != nil {
					c.logger.Error("handler error",
						zap.String("topic", msg.Topic),
						zap.Error(err),
					)
				}
			case kafka.Error:
				c.logger.Error("kafka error", zap.Error(e))
			}
		}
	}
}

func (c *Consumer) Close() {
	c.c.Close()
	c.logger.Info("kafka consumer closed")
}
