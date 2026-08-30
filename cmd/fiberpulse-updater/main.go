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
	kind := flag.String("kind", "file", "replacement kind: file or bundle")
	executable := flag.String("executable", "", "agent executable launched after a bundle update")
	waitPID := flag.Int("wait-pid", 0, "previous agent process that must exit before replacement")
	skipPlatformVerify := flag.Bool("skip-platform-verify", false, "skip OS signature verification (unsigned Windows development releases only)")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout+15*time.Second)
	defer cancel()
	err := agentupdate.Apply(ctx, agentupdate.Options{
		Target:             *target,
		Staged:             *staged,
		ManifestPath:       *manifestPath,
		PublicKeyHex:       *publicKeyHex,
		StatePath:          *statePath,
		CurrentVersion:     *currentVersion,
		Channel:            *channel,
		Kind:               agentupdate.Kind(*kind),
		Executable:         *executable,
		WaitPID:            *waitPID,
		HealthTimeout:      *timeout,
		SkipPlatformVerify: *skipPlatformVerify,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
