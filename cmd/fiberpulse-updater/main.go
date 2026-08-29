package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	agentupdate "fiberpulse.dev/agent/internal/update"
)

func main() {
	target := flag.String("target", "", "installed agent path")
	staged := flag.String("staged", "", "staged agent path")
	manifestPath := flag.String("manifest", "", "signed manifest path")
	publicKeyHex := flag.String("public-key", "", "Ed25519 public key hex")
	statePath := flag.String("state", "", "anti-rollback state path")
	currentVersion := flag.String("current-version", "", "currently installed semantic version")
	channel := flag.String("channel", "stable", "expected update channel")
	timeout := flag.Duration("timeout", 30*time.Second, "post-update health timeout")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout+15*time.Second)
	defer cancel()
	err := agentupdate.Apply(ctx, agentupdate.Options{
		Target:         *target,
		Staged:         *staged,
		ManifestPath:   *manifestPath,
		PublicKeyHex:   *publicKeyHex,
		StatePath:      *statePath,
		CurrentVersion: *currentVersion,
		Channel:        *channel,
		HealthTimeout:  *timeout,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
