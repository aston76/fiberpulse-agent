package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"fiberpulse.dev/agent/internal/observatory"
)

func main() {
	if err := run(); err != nil {
		slog.Error("FiberPulse Observatory stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
	dataDir := strings.TrimSpace(os.Getenv("FIBERPULSE_OBSERVATORY_DATA_DIR"))
	if dataDir == "" {
		dataDir = filepath.Join("dist", "observatory-runtime")
	}
	address := strings.TrimSpace(os.Getenv("FIBERPULSE_OBSERVATORY_ADDRESS"))
	if address == "" {
		address = "127.0.0.1:8090"
	}
	mode := strings.TrimSpace(os.Getenv("FIBERPULSE_OBSERVATORY_LOCATION_MODE"))
	if mode != "" && mode != "cloudflare" && mode != "plan-country" {
		return errors.New("FIBERPULSE_OBSERVATORY_LOCATION_MODE must be cloudflare or plan-country")
	}
	store, err := observatory.OpenStore(filepath.Join(dataDir, "observatory.db"))
	if err != nil {
		return err
	}
	defer store.Close()
	server, err := observatory.NewServer(observatory.Config{Store: store, Logger: logger, TrustCloudflareLocation: mode == "cloudflare"})
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if _, err := store.PurgeInactiveInstallations(ctx, time.Now().UTC().Add(-400*24*time.Hour)); err != nil {
		return err
	}
	go purgeInactiveInstallations(ctx, store, logger)
	logger.Info("FiberPulse Observatory starting", "address", address, "location_mode", mode)
	return observatory.Run(ctx, address, server.Handler(), logger)
}

func purgeInactiveInstallations(ctx context.Context, store *observatory.Store, logger *slog.Logger) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if _, err := store.PurgeInactiveInstallations(ctx, now.UTC().Add(-400*24*time.Hour)); err != nil {
				logger.Warn("inactive installation cleanup failed", "error", err)
			}
		}
	}
}
