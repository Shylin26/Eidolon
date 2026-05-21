package kafka

import (
	"fmt"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"go.uber.org/zap"
)

type Producer struct {
	p      *kafka.Producer
	logger *zap.Logger
}

func NewProducer(bootstrapServers string, logger *zap.Logger) (*Producer, error) {
	p, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers":       bootstrapServers,
		"acks":                    "1",
		"linger.ms":               5,
		"batch.size":              16384,
		"broker.address.family":   "v4",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create producer: %w", err)
	}

	go func() {
		for e := range p.Events() {
			switch ev := e.(type) {
			case *kafka.Message:
				if ev.TopicPartition.Error != nil {
					logger.Error("delivery failed",
						zap.String("topic", *ev.TopicPartition.Topic),
						zap.Error(ev.TopicPartition.Error),
					)
				}
			}
		}
	}()

	logger.Info("kafka producer ready", zap.String("servers", bootstrapServers))
	return &Producer{p: p, logger: logger}, nil
}

func (p *Producer) Publish(topic, key string, value []byte) error {
	return p.p.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{
			Topic:     &topic,
			Partition: kafka.PartitionAny,
		},
		Key:   []byte(key),
		Value: value,
	}, nil)
}

func (p *Producer) Close() {
	p.p.Flush(5000)
	p.p.Close()
	p.logger.Info("kafka producer closed")
}
