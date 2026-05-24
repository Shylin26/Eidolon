package rest

import (
	"context"
	"encoding/json"
	"os"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/eidolon/eidolon/internal/storage/sqlite"
	"go.uber.org/zap"

	"github.com/eidolon/eidolon/internal/kafka"
)

type Server struct {
	producer    *kafka.Producer
	sqliteStore *sqlite.Store
	logger      *zap.Logger
	router      *mux.Router
	srv         *http.Server
}

func NewServer(producer *kafka.Producer, logger *zap.Logger, port int) *Server {
	homeDir, _ := os.UserHomeDir()
	dbPath := homeDir + "/eidolon/data/eidolon.db"
	sqlStore, _ := sqlite.NewStore(dbPath)

	s := &Server{
		producer:    producer,
		sqliteStore: sqlStore,
		logger:      logger,
		router:      mux.NewRouter(),
	}
	s.srv = &http.Server{
		Addr:        fmt.Sprintf(":%d", port),
		Handler:     s.router,
		ReadTimeout: 10 * time.Second,
		IdleTimeout: 60 * time.Second,
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.router.Use(s.corsMiddleware)
	s.router.Use(s.loggingMiddleware)
	s.router.Handle("/metrics", promhttp.Handler())
	s.router.HandleFunc("/api/health", s.handleHealth).Methods("GET")
	s.router.HandleFunc("/api/status", s.handleStatus).Methods("GET")
	s.router.HandleFunc("/api/completions/stream", s.handleCompletionStream).Methods("GET")
	s.router.HandleFunc("/api/keystroke", s.handleKeystroke).Methods("POST", "OPTIONS")
	s.router.HandleFunc("/api/feedback", s.handleFeedback).Methods("POST", "OPTIONS")
}

func (s *Server) Start() error {
	s.logger.Info("REST API starting", zap.String("addr", s.srv.Addr))
	return s.srv.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "eidolon",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"model":   "CodeLlama-7B-4bit",
		"adapter": "base",
		"kafka":   "connected",
		"uptime":  time.Now().Unix(),
	})
}

func (s *Server) handleCompletionStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	consumer, err := kafka.NewConsumer(
		"localhost:9092",
		fmt.Sprintf("eidolon-sse-%d", time.Now().UnixNano()),
		[]string{"eidolon.completions"},
		s.logger,
	)
	if err != nil {
		http.Error(w, "failed to create consumer", http.StatusInternalServerError)
		return
	}
	defer consumer.Close()

	ctx := r.Context()
	consumer.Poll(ctx, func(msg kafka.Message) error {
		fmt.Fprintf(w, "data: %s\n\n", msg.Value)
		flusher.Flush()
		return nil
	})
}

func (s *Server) handleKeystroke(w http.ResponseWriter, r *http.Request) {
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	data, _ := json.Marshal(body)
	if err := s.producer.Publish("eidolon.keystrokes", "api", data); err != nil {
		http.Error(w, "failed to publish", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "queued"})
}

func (s *Server) handleFeedback(w http.ResponseWriter, r *http.Request) {
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var body struct {
		RequestID string `json:"request_id"`
		Action    string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if body.RequestID == "" || (body.Action != "accept" && body.Action != "reject") {
		http.Error(w, "invalid request_id or action", http.StatusBadRequest)
		return
	}
	if s.sqliteStore != nil {
		if err := s.sqliteStore.RecordFeedback(r.Context(), body.RequestID, body.Action); err != nil {
			s.logger.Warn("feedback record failed", zap.Error(err))
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "recorded"})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		s.logger.Info("http",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Duration("latency", time.Since(start)),
		)
	})
}
