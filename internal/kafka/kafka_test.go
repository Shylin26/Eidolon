package kafka

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestProducerConsumer(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	producer, err := NewProducer("localhost:9092", logger)
	if err != nil {
		t.Fatalf("failed to create producer: %v", err)
	}
	defer producer.Close()

	consumer, err := NewConsumer("localhost:9092", "eidolon-test-group", []string{"eidolon.metrics"}, logger)
	if err != nil {
		t.Fatalf("failed to create consumer: %v", err)
	}
	defer consumer.Close()

	// publish a test message
	err = producer.Publish("eidolon.metrics", "test-key", []byte(`{"test":"eidolon smoke test"}`))
	if err != nil {
		t.Fatalf("failed to publish: %v", err)
	}

	// consume it back with a 10s timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	received := make(chan string, 1)
	go func() {
		consumer.Poll(ctx, func(msg Message) error {
			received <- string(msg.Value)
			cancel()
			return nil
		})
	}()

	select {
	case msg := <-received:
		t.Logf("✓ received message: %s", msg)
	case <-ctx.Done():
		t.Fatal("timed out waiting for message")
	}
}
