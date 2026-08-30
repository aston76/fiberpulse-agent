package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fiberpulse.dev/agent/internal/update"
)

func TestGenerateKeyProducesAWorkingEd25519Pair(t *testing.T) {
	var output bytes.Buffer
	if err := run([]string{"generate-key"}, &output); err != nil {
		t.Fatal(err)
	}
	var privateHex, publicHex string
	for _, line := range strings.Split(output.String(), "\n") {
		if strings.HasPrefix(line, "private key") {
			privateHex = line[strings.LastIndex(line, " ")+1:]
		}
		if strings.HasPrefix(line, "public key") {
			publicHex = line[strings.LastIndex(line, " ")+1:]
		}
	}
	privateKey, err := hex.DecodeString(privateHex)
	if err != nil || len(privateKey) != ed25519.PrivateKeySize {
		t.Fatalf("private key: %v", err)
	}
	publicKey, err := hex.DecodeString(publicHex)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		t.Fatalf("public key: %v", err)
	}
	message := []byte("fiberpulse-release")
	if !ed25519.Verify(ed25519.PublicKey(publicKey), message, ed25519.Sign(ed25519.PrivateKey(privateKey), message)) {
		t.Fatal("generated key pair does not verify")
	}
}

func TestSignProducesVerifiableFeedDocument(t *testing.T) {
	var keyOutput bytes.Buffer
	if err := run([]string{"generate-key"}, &keyOutput); err != nil {
		t.Fatal(err)
	}
	var privateHex, publicHex string
	for _, line := range strings.Split(keyOutput.String(), "\n") {
		if strings.HasPrefix(line, "private key") {
			privateHex = line[strings.LastIndex(line, " ")+1:]
		}
		if strings.HasPrefix(line, "public key") {
			publicHex = line[strings.LastIndex(line, " ")+1:]
		}
	}
	directory := t.TempDir()
	artifact := filepath.Join(directory, "FiberPulse-1.4.0-macos.zip")
	if err := os.WriteFile(artifact, []byte("release-zip-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(directory, "latest-macos-universal.json")
	var output bytes.Buffer
	err := run([]string{"sign", "-version", "1.4.0", "-channel", "stable", "-sequence", "42", "-artifact", artifact, "-url", "https://github.com/aston76/fiberpulse-agent/releases/download/v1.4.0/FiberPulse-1.4.0-macos.zip", "-minimum-version", "1.0.0", "-key-hex", privateHex, "-expected-public-key", publicHex, "-out", out}, &output)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var manifest update.Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Version != "1.4.0" || manifest.Sequence != 42 || manifest.Channel != "stable" {
		t.Fatalf("manifest=%+v", manifest)
	}
	if manifest.Size != int64(len("release-zip-bytes")) {
		t.Fatalf("size=%d", manifest.Size)
	}
	// Re-verify the signature exactly like the updater does: the struct
	// without its signature is the signed payload.
	publicKey, err := hex.DecodeString(publicHex)
	if err != nil {
		t.Fatal(err)
	}
	signature := manifest.Signature
	manifest.Signature = nil
	unsigned, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if !ed25519.Verify(ed25519.PublicKey(publicKey), unsigned, signature) {
		t.Fatal("signed feed document does not verify against the public key")
	}
	info, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("feed document permissions: %v", info.Mode().Perm())
	}
}

func TestSignRejectsInvalidOperatorInput(t *testing.T) {
	directory := t.TempDir()
	artifact := filepath.Join(directory, "agent")
	if err := os.WriteFile(artifact, []byte("bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := run([]string{"sign", "-version", "v1.4.0", "-sequence", "1", "-artifact", artifact, "-url", "https://example.com/a", "-key-hex", strings.Repeat("00", 64), "-out", filepath.Join(directory, "o.json")}, &output); err == nil || !strings.Contains(err.Error(), "leading v") {
		t.Fatalf("leading v accepted: %v", err)
	}
	if err := run([]string{"sign", "-version", "1.4.0", "-sequence", "0", "-artifact", artifact, "-url", "https://example.com/a", "-key-hex", strings.Repeat("00", 64), "-out", filepath.Join(directory, "o.json")}, &output); err == nil || !strings.Contains(err.Error(), "sequence") {
		t.Fatalf("zero sequence accepted: %v", err)
	}
	if err := run([]string{"sign", "-version", "1.4.0", "-sequence", "1", "-artifact", artifact, "-url", "http://example.com/a", "-key-hex", strings.Repeat("00", 64), "-out", filepath.Join(directory, "o.json")}, &output); err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Fatalf("plaintext artifact URL accepted: %v", err)
	}
	if err := run([]string{"sign", "-version", "1.4.0", "-sequence", "1", "-artifact", artifact, "-url", "https://example.com/a", "-key-hex", "abcd", "-out", filepath.Join(directory, "o.json")}, &output); err == nil {
		t.Fatal("short private key accepted")
	}
	if err := run([]string{"sign", "-version", "1.4.0", "-sequence", "1", "-artifact", artifact, "-url", "https://example.com/a", "-key-hex", strings.Repeat("00", 64), "-expected-public-key", strings.Repeat("11", 32), "-out", filepath.Join(directory, "o.json")}, &output); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("mismatched release key accepted: %v", err)
	}
	if err := run([]string{"unknown"}, &output); err == nil {
		t.Fatal("unknown command accepted")
	}
}
