// release-sign is the offline release-signing tool. It generates Ed25519
// release keys and produces the signed per-platform feed documents consumed
// by the FiberPulse auto-update client. The private key only ever lives in
// the operator GitHub secret store; this tool never writes it to disk unless
// explicitly asked.
package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"fiberpulse.dev/agent/internal/update"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "release-sign:", err)
		os.Exit(1)
	}
}

func run(arguments []string, output io.Writer) error {
	if len(arguments) == 0 {
		return errors.New("usage: release-sign <generate-key|sign> [flags]")
	}
	switch arguments[0] {
	case "generate-key":
		return generateKey(output)
	case "sign":
		return sign(arguments[1:], output)
	default:
		return fmt.Errorf("unknown command %q", arguments[0])
	}
}

func generateKey(output io.Writer) error {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("generate release key: %w", err)
	}
	if _, err := fmt.Fprintf(output, "private key (store as the FIBERPULSE_UPDATE_SIGNING_KEY GitHub secret, never commit): %s\n", hex.EncodeToString(privateKey)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "public key (embed through the FIBERPULSE_UPDATE_PUBLIC_KEY repository variable): %s\n", hex.EncodeToString(publicKey)); err != nil {
		return err
	}
	return nil
}

func sign(arguments []string, output io.Writer) error {
	flags := flag.NewFlagSet("sign", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	version := flags.String("version", "", "release semantic version without a leading v")
	channel := flags.String("channel", "stable", "update channel: stable or canary")
	sequence := flags.Uint64("sequence", 0, "strictly increasing release sequence")
	artifact := flags.String("artifact", "", "path of the release artifact being published")
	artifactURL := flags.String("url", "", "absolute HTTPS download URL of the artifact")
	minimumVersion := flags.String("minimum-version", "0.1.0", "minimum installed version allowed to update")
	ttl := flags.Duration("ttl", 90*24*time.Hour, "manifest validity window from now")
	keyHex := flags.String("key-hex", "", "Ed25519 private key in hexadecimal")
	keyFile := flags.String("key-file", "", "file containing the Ed25519 private key in hexadecimal")
	expectedPublicKey := flags.String("expected-public-key", "", "expected Ed25519 public key in hexadecimal")
	out := flags.String("out", "", "output path of the signed feed document")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *version == "" || *artifact == "" || *artifactURL == "" || *out == "" {
		return errors.New("version, artifact, url and out are required")
	}
	if strings.HasPrefix(*version, "v") {
		return errors.New("version must not carry a leading v")
	}
	if *channel != "stable" && *channel != "canary" {
		return errors.New("channel must be stable or canary")
	}
	if *sequence == 0 {
		return errors.New("sequence must be a strictly increasing positive integer")
	}
	parsed, err := url.Parse(*artifactURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return errors.New("artifact URL must be an absolute HTTPS URL without credentials")
	}
	privateKey, err := readPrivateKey(*keyHex, *keyFile)
	if err != nil {
		return err
	}
	if *expectedPublicKey != "" {
		expected, decodeErr := hex.DecodeString(*expectedPublicKey)
		if decodeErr != nil || len(expected) != ed25519.PublicKeySize {
			return errors.New("expected public key must be a 32-byte Ed25519 key in hexadecimal")
		}
		actual := privateKey.Public().(ed25519.PublicKey)
		if !bytes.Equal(actual, expected) {
			return errors.New("release private key does not match the embedded public key")
		}
	}
	digest, size, err := hashArtifact(*artifact)
	if err != nil {
		return err
	}
	manifest := update.Manifest{
		Version:        *version,
		Channel:        *channel,
		Sequence:       *sequence,
		SHA256:         digest,
		Size:           size,
		URL:            *artifactURL,
		MinimumVersion: *minimumVersion,
		ExpiresAt:      time.Now().UTC().Add(*ttl).Truncate(time.Second),
	}
	raw, err := update.Sign(manifest, privateKey)
	if err != nil {
		return err
	}
	temporary := *out + ".new"
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create signed feed document: %w", err)
	}
	if _, err := file.Write(raw); err != nil {
		_ = file.Close()
		_ = os.Remove(temporary)
		return fmt.Errorf("write signed feed document: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("close signed feed document: %w", err)
	}
	if err := os.Rename(temporary, *out); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("publish signed feed document: %w", err)
	}
	if _, err := fmt.Fprintf(output, "signed %s feed for %s (%d bytes, sequence %d) written to %s\n", *channel, *version, size, *sequence, *out); err != nil {
		return err
	}
	return nil
}

func readPrivateKey(keyHex, keyFile string) (ed25519.PrivateKey, error) {
	if keyHex == "" && keyFile == "" {
		return nil, errors.New("a private key is required through key-hex or key-file")
	}
	if keyHex != "" && keyFile != "" {
		return nil, errors.New("provide the private key through key-hex or key-file, not both")
	}
	raw := keyHex
	if keyFile != "" {
		content, err := os.ReadFile(keyFile)
		if err != nil {
			return nil, fmt.Errorf("read private key file: %w", err)
		}
		raw = strings.TrimSpace(string(content))
	}
	key, err := hex.DecodeString(raw)
	if err != nil || len(key) != ed25519.PrivateKeySize {
		return nil, errors.New("private key must be a 64-byte Ed25519 key in hexadecimal")
	}
	return ed25519.PrivateKey(key), nil
}

func hashArtifact(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, fmt.Errorf("open release artifact: %w", err)
	}
	defer file.Close()
	digest := sha256.New()
	size, err := io.Copy(digest, file)
	if err != nil {
		return "", 0, fmt.Errorf("hash release artifact: %w", err)
	}
	if size == 0 {
		return "", 0, errors.New("release artifact is empty")
	}
	return hex.EncodeToString(digest.Sum(nil)), size, nil
}
