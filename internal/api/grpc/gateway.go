package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/eidolon/eidolon/internal/inference"
	"github.com/eidolon/eidolon/internal/kafka"
)

// ContextPayload mirrors what the context builder publishes
type ContextPayload struct {
	RequestID  string `json:"request_id"`
	FilePath   string `json:"file_path"`
	LanguageID string `json:"language_id"`
	Prefix     string `json:"prefix"`
	Suffix     string `json:"suffix"`
	MaxTokens  int    `json:"max_tokens"`
	Timestamp  int64  `json:"timestamp"`
}

// CompletionEvent is what we publish to eidolon.completions
type CompletionEvent struct {
	RequestID string `json:"request_id"`
	FilePath  string `json:"file_path"`
	Token     string `json:"token"`
	Done      bool   `json:"done"`
	Error     string `json:"error,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

type Gateway struct {
	inferClient *inference.Client
	producer    *kafka.Producer
	logger      *zap.Logger
}

func NewGateway(inferClient *inference.Client, producer *kafka.Producer, logger *zap.Logger) *Gateway {
	return &Gateway{
		inferClient: inferClient,
		producer:    producer,
		logger:      logger,
	}
}

// Handle consumes from eidolon.context.requests
func (g *Gateway) Handle(msg kafka.Message) error {
	var payload ContextPayload
	if err := json.Unmarshal([]byte(msg.Value), &payload); err != nil {
		return fmt.Errorf("unmarshal context payload: %w", err)
	}

	g.logger.Info("inference request received",
		zap.String("request_id", payload.RequestID),
		zap.String("language", payload.LanguageID),
		zap.Int("prefix_len", len(payload.Prefix)),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	tokens, err := g.inferClient.Complete(ctx, inference.Request{
		RequestID:   payload.RequestID,
		Prefix:      payload.Prefix,
		Suffix:      payload.Suffix,
		MaxTokens:   payload.MaxTokens,
		Temperature: 0.2,
		TopP:        0.95,
	})
	if err != nil {
		g.publishError(payload, err)
		return fmt.Errorf("inference failed: %w", err)
	}

	tokenCount := 0
	for tok := range tokens {
		event := CompletionEvent{
			RequestID: payload.RequestID,
			FilePath:  payload.FilePath,
			Token:     tok.Token,
			Done:      tok.Done,
			Timestamp: time.Now().UnixMilli(),
		}
		if tok.Error != "" {
			event.Error = tok.Error
			event.Done = true
		}

		data, _ := json.Marshal(event)
		if err := g.producer.Publish("eidolon.completions", payload.RequestID, data); err != nil {
			g.logger.Error("failed to publish token", zap.Error(err))
		}
		tokenCount++
		if tok.Done {
			break
		}
	}

	g.logger.Info("inference complete",
		zap.String("request_id", payload.RequestID),
		zap.Int("tokens", tokenCount),
	)
	return nil
}

func (g *Gateway) publishError(payload ContextPayload, err error) {
	event := CompletionEvent{
		RequestID: payload.RequestID,
		FilePath:  payload.FilePath,
		Done:      true,
		Error:     err.Error(),
		Timestamp: time.Now().UnixMilli(),
	}
	data, _ := json.Marshal(event)
	g.producer.Publish("eidolon.completions", payload.RequestID, data)
}
