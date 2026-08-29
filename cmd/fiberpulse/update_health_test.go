package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteUpdateHealthPublishesRestrictedReceiptBesideExecutable(t *testing.T) {
	directory := t.TempDir()
	executable := filepath.Join(directory, "fiberpulse")
	healthPath := filepath.Join(directory, ".fiberpulse-health-test")
	if err := writeUpdateHealth(healthPath, "1.2.3", executable); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(healthPath)
	if err != nil {
		t.Fatal(err)
	}
	var receipt updateHealthReceipt
	if err := json.Unmarshal(raw, &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.Version != "1.2.3" || receipt.PID != os.Getpid() {
		t.Fatalf("receipt=%+v", receipt)
	}
	info, err := os.Stat(healthPath)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("mode=%v err=%v", info.Mode().Perm(), err)
	}
}

func TestWriteUpdateHealthRejectsUnreservedOrExternalPath(t *testing.T) {
	directory := t.TempDir()
	executable := filepath.Join(directory, "fiberpulse")
	for _, path := range []string{filepath.Join(directory, "health.json"), filepath.Join(filepath.Dir(directory), ".fiberpulse-health-outside")} {
		if err := writeUpdateHealth(path, "1.2.3", executable); err == nil || !strings.Contains(err.Error(), "reserved file") {
			t.Fatalf("path %s was not rejected: %v", path, err)
		}
	}
}
