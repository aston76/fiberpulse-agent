package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"fiberpulse.dev/agent/internal/app"
	"fiberpulse.dev/agent/internal/measurement"
	"fiberpulse.dev/agent/internal/platform"
)

var version = "0.1.0-dev"

func main() {
	if err := run(); err != nil {
		slog.Error("FiberPulse stopped with an error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	quitExisting := flag.Bool("quit", false, "request graceful shutdown of the running agent")
	postUpdate := flag.String("post-update", "", "version started after a verified update")
	flag.Parse()
	configDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	dataDir := os.Getenv("FIBERPULSE_DATA_DIR")
	if dataDir == "" {
		dataDir = filepath.Join(configDir, "FiberPulse")
	}
	shutdownPath := filepath.Join(dataDir, "shutdown.sock")
	if *quitExisting {
		return platform.RequestShutdown(shutdownPath)
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return err
	}
	lock, err := platform.AcquireSingleInstance(filepath.Join(dataDir, "agent.lock"))
	if err != nil {
		return err
	}
	defer lock.Close()
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
	if *postUpdate != "" {
		logger.Info("post-update startup", "version", *postUpdate)
	}
	provider := measurement.Provider(&measurement.MLabProvider{ClientName: "fiberpulse-ph", ClientVersion: version, Enabled: os.Getenv("FIBERPULSE_ENABLE_MLAB_DEV") == "1", Timeout: 60 * time.Second})
	if os.Getenv("FIBERPULSE_DEV_FAKE") == "1" {
		provider = &measurement.FakeProvider{}
	}
	agent, err := app.New(app.Config{Version: version, DatabasePath: filepath.Join(dataDir, "fiberpulse.db"), Provider: provider, ProbeURL: os.Getenv("FIBERPULSE_PROBE_URL"), DNSName: os.Getenv("FIBERPULSE_DNS_NAME"), Logger: logger})
	if err != nil {
		return err
	}
	url, err := agent.Start()
	if err != nil {
		return err
	}
	defer agent.Close()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(quit)
	shutdownRequests, stopShutdownListener, shutdownErr := platform.ShutdownRequests(shutdownPath)
	if shutdownErr != nil {
		logger.Warn("installer shutdown listener unavailable", "error", shutdownErr)
	} else {
		defer stopShutdownListener()
	}
	stopTray, trayErr := platform.StartTray(platform.TrayActions{Open: func() { _ = platform.OpenURL(agent.BootstrapURL()) }, Test: func() { _ = agent.StartTest(context.Background(), "manual") }, Pause: func() {}, Report: func() { _ = platform.OpenURL(agent.BootstrapURL()) }, Update: func() {}, Quit: func() { _ = agent.Action(context.Background(), "quit", []byte(`{}`)) }})
	if trayErr != nil {
		logger.Warn("native tray unavailable", "error", trayErr)
	} else {
		defer stopTray()
	}
	if err := platform.OpenURL(url); err != nil {
		logger.Info("open the dashboard manually", "url", url, "error", err)
	}
	go func() {
		select {
		case <-quit:
			_ = agent.Action(context.Background(), "quit", []byte(`{}`))
		case <-shutdownRequests:
			_ = agent.Action(context.Background(), "quit", []byte(`{}`))
		case <-time.After(100 * 365 * 24 * time.Hour):
		}
	}()
	agent.Wait()
	logger.Info("FiberPulse shutdown complete", "os", runtime.GOOS)
	return nil
}
