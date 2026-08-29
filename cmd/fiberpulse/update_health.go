package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type updateHealthReceipt struct {
	Version string `json:"version"`
	PID     int    `json:"pid"`
}

func writeUpdateHealth(path, version, executable string) error {
	if path == "" || version == "" || executable == "" {
		return errors.New("update health path, version, and executable are required")
	}
	if !filepath.IsAbs(path) || !filepath.IsAbs(executable) {
		return errors.New("update health and executable paths must be absolute")
	}
	cleanPath := filepath.Clean(path)
	cleanExecutable := filepath.Clean(executable)
	if filepath.Dir(cleanPath) != filepath.Dir(cleanExecutable) || !strings.HasPrefix(filepath.Base(cleanPath), ".fiberpulse-health-") {
		return errors.New("update health receipt must use a reserved file beside the executable")
	}
	raw, err := json.Marshal(updateHealthReceipt{Version: version, PID: os.Getpid()})
	if err != nil {
		return err
	}
	temporary := cleanPath + ".new"
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create update health receipt: %w", err)
	}
	_, writeErr := file.Write(raw)
	syncErr := file.Sync()
	closeErr := file.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil {
		_ = os.Remove(temporary)
		return errors.Join(writeErr, syncErr, closeErr)
	}
	if err := os.Rename(temporary, cleanPath); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("publish update health receipt: %w", err)
	}
	return nil
}
