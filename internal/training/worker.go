package training

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"os/exec"
	"time"

	"go.uber.org/zap"
	_ "github.com/mattn/go-sqlite3"
)

const (
	feedbackThreshold = 50
	checkInterval     = 30 * time.Second
)

type Worker struct {
	dbPath    string
	logger    *zap.Logger
	lastCount int
}

type TrainingStatus struct {
	Status      string  `json:"status"`
	Message     string  `json:"message"`
	Timestamp   float64 `json:"timestamp"`
	AdapterPath string  `json:"adapter_path"`
}

func NewWorker(dbPath string, logger *zap.Logger) *Worker {
	return &Worker{dbPath: dbPath, logger: logger}
}

func (w *Worker) Run(ctx context.Context) {
	w.logger.Info("retraining worker started",
		zap.Int("threshold", feedbackThreshold),
		zap.Duration("check_interval", checkInterval),
	)
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.check(ctx)
		}
	}
}

func (w *Worker) check(ctx context.Context) {
	db, err := sql.Open("sqlite3", w.dbPath)
	if err != nil {
		w.logger.Error("sqlite open failed", zap.Error(err))
		return
	}
	defer db.Close()

	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM feedback_events").Scan(&count); err != nil {
		w.logger.Error("count failed", zap.Error(err))
		return
	}

	newEvents := count - w.lastCount
	w.logger.Info("feedback check",
		zap.Int("total", count),
		zap.Int("new", newEvents),
		zap.Int("threshold", feedbackThreshold),
	)

	if newEvents >= feedbackThreshold {
		w.logger.Info("threshold reached — triggering LoRA retraining")
		w.lastCount = count
		w.triggerRetraining()
	}
}

func (w *Worker) triggerRetraining() {
	home, _ := os.UserHomeDir()
	script := home + "/eidolon/ml/retrain.py"
	logf   := home + "/eidolon/data/retrain.log"

	lf, err := os.OpenFile(logf, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		w.logger.Error("log open failed", zap.Error(err))
		return
	}
	defer lf.Close()

	cmd := exec.Command("python3", script)
	cmd.Stdout = lf
	cmd.Stderr = lf

	if err := cmd.Start(); err != nil {
		w.logger.Error("retrain start failed", zap.Error(err))
		return
	}

	go func() {
		if err := cmd.Wait(); err != nil {
			w.logger.Error("retrain failed", zap.Error(err))
			return
		}
		w.logger.Info("retraining complete")
		w.swapAdapter()
	}()
}

func (w *Worker) swapAdapter() {
	home, _ := os.UserHomeDir()
	data, err := os.ReadFile(home + "/eidolon/data/training_status.json")
	if err != nil {
		return
	}
	var status TrainingStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return
	}
	if status.Status == "complete" && status.AdapterPath != "" {
		_ = os.WriteFile(home+"/eidolon/data/active_adapter.txt", []byte(status.AdapterPath), 0644)
		w.logger.Info("adapter hot-swapped", zap.String("adapter", status.AdapterPath))
	}
}
