package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/eidolon/eidolon/internal/training"

	eidolongrpc "github.com/eidolon/eidolon/internal/api/grpc"
	"github.com/eidolon/eidolon/internal/api/rest"
	"github.com/eidolon/eidolon/internal/config"
	eidoloncontext "github.com/eidolon/eidolon/internal/context"
	"github.com/eidolon/eidolon/internal/inference"
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	builder := eidoloncontext.NewBuilder(producer, logger)
	keystrokeConsumer, err := eidolonkafka.NewConsumer(cfg.Kafka.BootstrapServers, "eidolon-context-builder", []string{"eidolon.keystrokes"}, logger)
	if err != nil {
		logger.Fatal("failed to create keystroke consumer", zap.Error(err))
	}
	defer keystrokeConsumer.Close()

	go func() {
		logger.Info("context builder started")
		keystrokeConsumer.Poll(ctx, builder.Handle)
	}()

	inferClient := inference.NewClient()
	gateway := eidolongrpc.NewGateway(inferClient, producer, logger)
	contextConsumer, err := eidolonkafka.NewConsumer(cfg.Kafka.BootstrapServers, "eidolon-infer-gateway", []string{"eidolon.context.requests"}, logger)
	if err != nil {
		logger.Fatal("failed to create context consumer", zap.Error(err))
	}
	defer contextConsumer.Close()

	go func() {
		logger.Info("inference gateway started")
		contextConsumer.Poll(ctx, gateway.Handle)
	}()

	restServer := rest.NewServer(producer, logger, cfg.Server.HTTPPort)
	go func() {
		if err := restServer.Start(); err != nil {
			logger.Info("REST server stopped", zap.Error(err))
		}
	}()

	homedir, _ := os.UserHomeDir()
	retrainWorker := training.NewWorker(homedir+"/eidolon/data/eidolon.db", logger)
	go retrainWorker.Run(ctx)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	logger.Info("Eidolon shutting down", zap.String("signal", sig.String()))
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	restServer.Shutdown(shutdownCtx)
}
