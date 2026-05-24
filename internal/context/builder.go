package context

import (
	"encoding/json"
	"strings"
	"time"

	"go.uber.org/zap"
	"github.com/eidolon/eidolon/internal/kafka"
)

// KeystrokeEvent is what the VSCode extension publishes
type KeystrokeEvent struct {
	RequestID    string `json:"request_id"`
	FilePath     string `json:"file_path"`
	LanguageID   string `json:"language_id"`
	Content      string `json:"content"`
	CursorOffset int    `json:"cursor_offset"`
	Timestamp    int64  `json:"timestamp"`
}

// ContextPayload is the enriched payload sent to inference
type ContextPayload struct {
	RequestID  string `json:"request_id"`
	FilePath   string `json:"file_path"`
	LanguageID string `json:"language_id"`
	Prefix     string `json:"prefix"`
	Suffix     string `json:"suffix"`
	MaxTokens  int    `json:"max_tokens"`
	Timestamp  int64  `json:"timestamp"`
}

const (
	maxPrefixChars = 3000
	maxSuffixChars = 500
	maxTokens      = 64
)

type Builder struct {
	producer *kafka.Producer
	logger   *zap.Logger
	debounce map[string]*time.Timer
}

func NewBuilder(producer *kafka.Producer, logger *zap.Logger) *Builder {
	return &Builder{
		producer: producer,
		logger:   logger,
		debounce: make(map[string]*time.Timer),
	}
}

// Handle processes a raw keystroke event from Kafka
func (b *Builder) Handle(msg kafka.Message) error {
	var event KeystrokeEvent
	if err := json.Unmarshal([]byte(msg.Value), &event); err != nil {
		return err
	}

	// debounce per file — cancel previous timer if typing fast
	if t, ok := b.debounce[event.FilePath]; ok {
		t.Stop()
	}

	b.debounce[event.FilePath] = time.AfterFunc(300*time.Millisecond, func() {
		if err := b.process(event); err != nil {
			b.logger.Error("context build failed",
				zap.String("file", event.FilePath),
				zap.Error(err),
			)
		}
	})

	return nil
}

func (b *Builder) process(event KeystrokeEvent) error {
	prefix, suffix := splitAtCursor(event.Content, event.CursorOffset)

	// trim to token budget
	if len(prefix) > maxPrefixChars {
		prefix = prefix[len(prefix)-maxPrefixChars:]
	}
	if len(suffix) > maxSuffixChars {
		suffix = suffix[:maxSuffixChars]
	}

	// skip if prefix is too short to be useful
	if strings.TrimSpace(prefix) == "" {
		return nil
	}

	payload := ContextPayload{
		RequestID:  event.RequestID,
		FilePath:   event.FilePath,
		LanguageID: event.LanguageID,
		Prefix:     prefix,
		Suffix:     suffix,
		MaxTokens:  maxTokens,
		Timestamp:  time.Now().UnixMilli(),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	b.logger.Info("context built",
		zap.String("request_id", payload.RequestID),
		zap.String("language", payload.LanguageID),
		zap.Int("prefix_len", len(prefix)),
		zap.Int("suffix_len", len(suffix)),
	)

	return b.producer.Publish("eidolon.context.requests", event.FilePath, data)
}

// splitAtCursor splits file content into prefix and suffix at cursor position
func splitAtCursor(content string, offset int) (string, string) {
	runes := []rune(content)
	if offset < 0 {
		offset = 0
	}
	if offset > len(runes) {
		offset = len(runes)
	}
	return string(runes[:offset]), string(runes[offset:])
}
