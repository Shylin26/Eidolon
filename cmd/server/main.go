package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	eidoloncontext "github.com/eidolon/eidolon/internal/context"
	"github.com/eidolon/eidolon/internal/config"
	eidolonkafka "github.com/eidolon/eidolon/internal/kafka"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	logger.Info("Eidolon starting",
		zap.String("kafka", cfg.Kafka.BootstrapServers),
		zap.Int("grpc_port", cfg.Server.GRPCPort),
		zap.Int("http_port", cfg.Server.HTTPPort),
	)

	producer, err := eidolonkafka.NewProducer(cfg.Kafka.BootstrapServers, logger)
	if err != nil {
		logger.Fatal("failed to create kafka producer", zap.Error(err))
	}
	defer producer.Close()

	builder := eidoloncontext.NewBuilder(producer, logger)

	consumer, err := eidolonkafka.NewConsumer(
		cfg.Kafka.BootstrapServers,
		"eidolon-context-builder",
		[]string{"eidolon.keystrokes"},
		logger,
	)
	if err != nil {
		logger.Fatal("failed to create kafka consumer", zap.Error(err))
	}
	defer consumer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		logger.Info("context builder started, consuming eidolon.keystrokes")
		consumer.Poll(ctx, builder.Handle)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	logger.Info("Eidolon shutting down", zap.String("signal", sig.String()))
	cancel()
}
