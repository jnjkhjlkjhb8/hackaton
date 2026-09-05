// Command host receives farm telemetry and serves Host management endpoints.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"example.com/farmhost/internal/config"
	"example.com/farmhost/internal/httpapi"
	"example.com/farmhost/internal/store"
	"example.com/farmhost/internal/telemetry"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer pool.Close()
	consumer := telemetry.NewConsumer(telemetry.WriterFunc(func(ctx context.Context, masterID string, batch telemetry.Batch) error {
		return store.StoreBatch(ctx, pool, masterID, batch)
	}), logger)
	mqttClient, err := telemetry.Start(cfg.MQTTBrokerURL, cfg.MQTTUsername, cfg.MQTTPassword, consumer)
	if err != nil {
		return fmt.Errorf("start mqtt: %w", err)
	}
	defer mqttClient.Disconnect(250)

	srv := httpapi.NewServer(pool, logger, cfg.HTTPAddr)
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Run() }()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("shutdown server: %w", err)
	}
	return nil
}
