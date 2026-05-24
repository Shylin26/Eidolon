package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	CompletionsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "eidolon_completions_total",
		Help: "Total number of completions generated",
	}, []string{"language", "status"})

	CompletionTokens = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "eidolon_completion_tokens",
		Help:    "Number of tokens per completion",
		Buckets: []float64{10, 20, 32, 48, 64, 96, 128, 256},
	})

	InferenceLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "eidolon_inference_duration_seconds",
		Help:    "Time from request to first token",
		Buckets: prometheus.DefBuckets,
	})

	FeedbackTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "eidolon_feedback_total",
		Help: "Total accept/reject feedback signals",
	}, []string{"action"})

	RAGChunksInjected = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "eidolon_rag_chunks_injected",
		Help:    "Number of RAG chunks injected per request",
		Buckets: []float64{0, 1, 2, 3, 4, 5},
	})

	KafkaMessagesProduced = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "eidolon_kafka_messages_produced_total",
		Help: "Total Kafka messages produced per topic",
	}, []string{"topic"})

	ActiveInferenceRequests = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "eidolon_active_inference_requests",
		Help: "Number of inference requests currently in flight",
	})
)
