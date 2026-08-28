//go:build !windows

package platform

import (
	"strings"
	"testing"
	"time"
)

func TestShutdownRequestIsIdempotentWhenAgentIsNotRunning(t *testing.T) {
	// Use the same bounded endpoint derivation as production. macOS limits Unix
	// socket paths to roughly 104 bytes and its per-test temporary directory can
	// already exceed that limit on GitHub-hosted runners.
	endpoint := ShutdownPath(t.TempDir())
	if err := RequestShutdown(endpoint); err != nil {
		t.Fatalf("idempotent shutdown failed: %v", err)
	}
}

func TestShutdownEndpointIsShortAndAcknowledged(t *testing.T) {
	endpoint := ShutdownPath("/Volumes/" + strings.Repeat("deep-project-directory/", 20))
	if len(endpoint) > 100 {
		t.Fatalf("Unix socket endpoint is too long: %d", len(endpoint))
	}
	requests, cleanup, err := ShutdownRequests(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	result := make(chan error, 1)
	go func() { result <- RequestShutdown(endpoint) }()
	select {
	case <-requests:
		cleanup()
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown request was not delivered")
	}
	if err := <-result; err != nil {
		t.Fatal(err)
	}
}
