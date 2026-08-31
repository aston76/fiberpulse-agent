package main

import (
	"context"
	"flag"
	"fmt"
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
	"fiberpulse.dev/agent/internal/sharing"
	"fiberpulse.dev/agent/internal/sponsor"
)

var version = "0.1.1-dev"
var sharingEndpoint = ""
var updateFeedURL = ""
var updatePublicKey = ""
var updateSkipPlatformVerify = ""

func main() {
	if err := run(); err != nil {
		slog.Error("FiberPulse stopped with an error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	quitExisting := flag.Bool("quit", false, "request graceful shutdown of the running agent")
	background := flag.Bool("background", false, "start without opening the dashboard")
	postUpdate := flag.String("post-update", "", "version started after a verified update")
	updateHealthPath := flag.String("update-health-file", "", "internal post-update health receipt path")
	flag.Parse()
	configDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	dataDir := os.Getenv("FIBERPULSE_DATA_DIR")
	if dataDir == "" {
		dataDir = filepath.Join(configDir, "FiberPulse")
	}
	shutdownPath := platform.ShutdownPath(dataDir)
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
	provider := measurement.Provider(&measurement.MLabProvider{ClientName: "fiberpulse-ph", ClientVersion: version, Enabled: true, Timeout: 60 * time.Second})
	if os.Getenv("FIBERPULSE_DEV_FAKE") == "1" {
		provider = &measurement.FakeProvider{}
	}
	sponsorOffer := sponsor.Offer{
		CampaignID: os.Getenv("FIBERPULSE_SPONSOR_CAMPAIGN_ID"), Label: os.Getenv("FIBERPULSE_SPONSOR_LABEL"),
		Headline: os.Getenv("FIBERPULSE_SPONSOR_HEADLINE"), Body: os.Getenv("FIBERPULSE_SPONSOR_BODY"),
		CTA: os.Getenv("FIBERPULSE_SPONSOR_CTA"), URL: os.Getenv("FIBERPULSE_SPONSOR_URL"),
	}
	shareURL := os.Getenv("FIBERPULSE_SHARE_URL")
	if shareURL == "" {
		shareURL = sharingEndpoint
	}
	var shareTransport sharing.Sender
	if shareURL != "" {
		transport, transportErr := sharing.NewHTTPTransport(shareURL, nil)
		if transportErr != nil {
			return fmt.Errorf("configure anonymous sharing: %w", transportErr)
		}
		shareTransport = transport
	}
	updateConfig, err := buildUpdateConfig(dataDir, logger)
	if err != nil {
		return err
	}
	agent, err := app.New(app.Config{Version: version, DatabasePath: filepath.Join(dataDir, "fiberpulse.db"), Provider: provider, ProbeURL: os.Getenv("FIBERPULSE_PROBE_URL"), DNSName: os.Getenv("FIBERPULSE_DNS_NAME"), SharingTransport: shareTransport, Sponsor: sponsorOffer, Logger: logger, Update: updateConfig})
	if err != nil {
		return err
	}
	defer agent.Close()
	url, err := agent.Start()
	if err != nil {
		return err
	}
	if *updateHealthPath != "" {
		executable, err := os.Executable()
		if err != nil {
			return fmt.Errorf("resolve executable for update health receipt: %w", err)
		}
		if err := writeUpdateHealth(*updateHealthPath, version, executable); err != nil {
			return err
		}
	}
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(quit)
	shutdownRequests, stopShutdownListener, shutdownErr := platform.ShutdownRequests(shutdownPath)
	if shutdownErr != nil {
		logger.Warn("installer shutdown listener unavailable", "error", shutdownErr)
	} else {
		defer stopShutdownListener()
	}
	actions := platform.TrayActions{
		Open: func() { _ = platform.OpenURL(agent.BootstrapURL()) },
		Test: func() { _ = agent.StartTest(context.Background(), "manual") },
		Pause: func() {
			if err := agent.TogglePause(context.Background()); err != nil {
				logger.Warn("tray pause action failed", "error", err)
			}
		},
		Report: func() { _ = platform.OpenURL(agent.BootstrapURL()) },
		CheckUpdate: func() {
			go func() {
				if err := agent.CheckForUpdate(context.Background()); err != nil {
					logger.Info("manual update check", "error", err)
				}
				status := agent.UpdateStatus()
				state := platform.TrayState{Version: status.CurrentVersion, Paused: agent.Paused(), UpdateStatus: status.Status, AvailableVersion: status.AvailableVersion, UpdateError: status.Error}
				if platform.PresentUpdateResult(state) {
					if err := agent.ApplyUpdate(context.Background()); err != nil {
						logger.Warn("approved update install failed", "error", err)
					}
				}
			}()
		},
		InstallUpdate: func() {
			go func() {
				if err := agent.ApplyUpdate(context.Background()); err != nil {
					logger.Warn("update install failed", "error", err)
				}
			}()
		},
		State: func() platform.TrayState {
			status := agent.UpdateStatus()
			return platform.TrayState{Version: status.CurrentVersion, Paused: agent.Paused(), UpdateStatus: status.Status, AvailableVersion: status.AvailableVersion, UpdateError: status.Error}
		},
		Quit: func() { _ = agent.Action(context.Background(), "quit", []byte(`{}`)) },
	}
	if !*background {
		if err := platform.OpenURL(url); err != nil {
			logger.Info("open the dashboard manually", "url", url, "error", err)
		}
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
	if trayErr := platform.RunTray(actions, agent.Done()); trayErr != nil {
		logger.Warn("native tray unavailable", "error", trayErr)
	}
	agent.Wait()
	if err := agent.Close(); err != nil {
		return err
	}
	logger.Info("FiberPulse shutdown complete", "os", runtime.GOOS)
	return nil
}
