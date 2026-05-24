package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/eidolon/eidolon/internal/embedding"
	"github.com/eidolon/eidolon/internal/storage/sqlite"
	"github.com/eidolon/eidolon/internal/inference"
	"github.com/eidolon/eidolon/internal/kafka"
	"github.com/eidolon/eidolon/internal/storage/vector"
)

type ContextPayload struct {
	RequestID  string `json:"request_id"`
	FilePath   string `json:"file_path"`
	LanguageID string `json:"language_id"`
	Prefix     string `json:"prefix"`
	Suffix     string `json:"suffix"`
	MaxTokens  int    `json:"max_tokens"`
	Timestamp  int64  `json:"timestamp"`
}

type CompletionEvent struct {
	RequestID string `json:"request_id"`
	FilePath  string `json:"file_path"`
	Token     string `json:"token"`
	Done      bool   `json:"done"`
	Error     string `json:"error,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

type Gateway struct {
	inferClient  *inference.Client
	embedClient  *embedding.Client
	vectorStore  *vector.Store
	sqliteStore  *sqlite.Store
	producer     *kafka.Producer
	logger       *zap.Logger
}

func NewGateway(inferClient *inference.Client, producer *kafka.Producer, logger *zap.Logger) *Gateway {
	embedClient := embedding.NewClient()

	ctx := context.Background()
	store, err := vector.NewStore(ctx)
	if err != nil {
		logger.Warn("pgvector unavailable — RAG disabled", zap.Error(err))
		store = nil
	}

	homeDir, _ := os.UserHomeDir()
	dbPath := homeDir + "/eidolon/data/eidolon.db"
	logger.Info("opening sqlite", zap.String("path", dbPath))
	sqlStore, err2 := sqlite.NewStore(dbPath)
	if err2 != nil {
		logger.Error("sqlite unavailable", zap.Error(err2))
		sqlStore = nil
	} else {
		logger.Info("sqlite ready")
	}

	return &Gateway{
		inferClient: inferClient,
		embedClient: embedClient,
		vectorStore: store,
		sqliteStore: sqlStore,
		producer:    producer,
		logger:      logger,
	}
}

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

	// G: inject similar code chunks into the prefix
	prefix := g.enrichWithRAG(payload.Prefix, payload.LanguageID)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	tokens, err := g.inferClient.Complete(ctx, inference.Request{
		RequestID:   payload.RequestID,
		Prefix:      prefix,
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
	fullText := ""
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
		fullText += tok.Token
		tokenCount++
		if tokenCount == 1 {
			g.logger.Info("first token received", zap.String("request_id", payload.RequestID), zap.String("token", tok.Token))
		}
		if tok.Done {
			g.logger.Info("all tokens received", zap.String("request_id", payload.RequestID), zap.Int("count", tokenCount), zap.Int("text_len", len(fullText)))
			break
		}
	}

	g.logger.Info("inference complete",
		zap.String("request_id", payload.RequestID),
		zap.Int("tokens", tokenCount),
	)

	// persist to sqlite
	if g.sqliteStore != nil {
		err := g.sqliteStore.SaveCompletion(context.Background(), sqlite.Completion{
			RequestID:       payload.RequestID,
			FilePath:        payload.FilePath,
			LanguageID:      payload.LanguageID,
			CompletionText:  fullText,
			TokensGenerated: tokenCount,
		})
		if err != nil {
			g.logger.Error("sqlite save failed", zap.Error(err))
		} else {
			g.logger.Info("completion saved to sqlite", zap.String("request_id", payload.RequestID))
		}
	} else {
		g.logger.Warn("sqliteStore is nil, skipping save")
	}

	return nil
}

func (g *Gateway) enrichWithRAG(prefix, lang string) string {
	if g.vectorStore == nil || g.embedClient == nil {
		return prefix
	}

	vec, err := g.embedClient.EmbedOne(prefix)
	if err != nil {
		g.logger.Warn("embed failed for RAG", zap.Error(err))
		return prefix
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	chunks, err := g.vectorStore.Search(ctx, vec, 3)
	if err != nil || len(chunks) == 0 {
		return prefix
	}

	var sb strings.Builder
	sb.WriteString("// Similar code from your codebase:\n")
	for _, c := range chunks {
		sb.WriteString(fmt.Sprintf("// --- %s ---\n", c.FilePath))
		lines := strings.Split(c.Content, "\n")
		for _, l := range lines {
			if strings.TrimSpace(l) != "" {
				sb.WriteString(fmt.Sprintf("// %s\n", l))
			}
		}
	}
	sb.WriteString("\n")
	sb.WriteString(prefix)

	g.logger.Info("RAG context injected", zap.Int("chunks", len(chunks)))
	return sb.String()
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
