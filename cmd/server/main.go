package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"hacktrapagent-api-endpoint/internal/config"
	"hacktrapagent-api-endpoint/internal/httpapi"
	"hacktrapagent-api-endpoint/internal/ratelimit"
	"hacktrapagent-api-endpoint/internal/service"
	"hacktrapagent-api-endpoint/internal/storage/clickhouse"
)

func main() {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	store, err := clickhouse.NewStore(ctx, cfg.ClickHouse)
	if err != nil {
		slog.Error("failed to connect clickhouse", "error", err)
		os.Exit(1)
	}
	defer func() {
		if cerr := store.Close(); cerr != nil {
			slog.Warn("failed to close clickhouse", "error", cerr)
		}
	}()

	if err := store.EnsureSchema(ctx); err != nil {
		slog.Error("failed to ensure schema", "error", err)
		os.Exit(1)
	}

	limiter, err := ratelimit.New(cfg.Limits)
	if err != nil {
		slog.Error("failed to configure limiter", "error", err)
		os.Exit(1)
	}

	maxWindow := limiter.MaxWindow()
	if maxWindow > 0 {
		if err := preloadCounters(ctx, store, limiter, maxWindow); err != nil {
			slog.Error("failed to preload limiter", "error", err)
			os.Exit(1)
		}
	}

	svc := service.New(
		service.Dependencies{
			Store:     store,
			Limiter:   limiter,
			Blacklist: cfg.Blacklist,
			Whitelist: cfg.Whitelist,
			Clock:     time.Now,
		},
	)

	handler := httpapi.NewHandler(svc, cfg.RequestBodyLimitBytes)

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("starting HTTP server", "addr", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server exited with error", "error", err)
			os.Exit(1)
		}
	}()

	shutdownCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-shutdownCtx.Done()

	ctxTimeout, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctxTimeout); err != nil {
		slog.Error("failed to shutdown server", "error", err)
		os.Exit(1)
	}
}

func preloadCounters(ctx context.Context, store service.EventStore, limiter *ratelimit.Limiter, maxWindow time.Duration) error {
	now := time.Now()
	rows, err := store.LoadRecent(ctx, now.Add(-maxWindow))
	if err != nil {
		return err
	}

	for _, row := range rows {
		limiter.Seed(row.ToValuesMap(), row.RegisteredAt, now)
	}

	slog.Info("preloaded limiter counters", "events", len(rows), "window", maxWindow.String())
	return nil
}
