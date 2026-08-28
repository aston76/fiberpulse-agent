package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type Manifest struct {
	Version   string `json:"version"`
	Sequence  uint64 `json:"sequence"`
	SHA256    string `json:"sha256"`
	Size      int64  `json:"size"`
	Signature []byte `json:"signature"`
}

func main() {
	target := flag.String("target", "", "installed agent path")
	staged := flag.String("staged", "", "verified staged agent path")
	manifestPath := flag.String("manifest", "", "signed manifest path")
	publicKeyHex := flag.String("public-key", "", "Ed25519 public key hex")
	flag.Parse()
	if err := update(*target, *staged, *manifestPath, *publicKeyHex); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func update(target, staged, manifestPath, keyHex string) error {
	if target == "" || staged == "" || manifestPath == "" {
		return errors.New("target, staged, and manifest are required")
	}
	key, err := hex.DecodeString(keyHex)
	if err != nil || len(key) != ed25519.PublicKeySize {
		return errors.New("invalid public key")
	}
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return err
	}
	signature := m.Signature
	m.Signature = nil
	unsigned, _ := json.Marshal(m)
	if !ed25519.Verify(ed25519.PublicKey(key), unsigned, signature) {
		return errors.New("manifest signature verification failed")
	}
	artifact, err := os.ReadFile(staged)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(artifact)
	if int64(len(artifact)) != m.Size || hex.EncodeToString(sum[:]) != m.SHA256 {
		return errors.New("artifact hash or size mismatch")
	}
	backup := target + ".previous"
	_ = os.Remove(backup)
	if _, err := os.Stat(target); err == nil {
		if err := os.Rename(target, backup); err != nil {
			return err
		}
	}
	if err := copyFile(staged, target); err != nil {
		_ = os.Rename(backup, target)
		return err
	}
	cmd := exec.Command(target, "--post-update", m.Version)
	if err := cmd.Start(); err != nil {
		_ = os.Remove(target)
		_ = os.Rename(backup, target)
		return err
	}
	time.Sleep(3 * time.Second)
	return nil
}
func copyFile(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := target + ".new"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	_, copyErr := out.ReadFrom(in)
	syncErr := out.Sync()
	closeErr := out.Close()
	if copyErr != nil || syncErr != nil || closeErr != nil {
		_ = os.Remove(tmp)
		return errors.Join(copyErr, syncErr, closeErr)
	}
	if err := os.Rename(tmp, filepath.Clean(target)); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
