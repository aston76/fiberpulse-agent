package update

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"
)

func TestSemanticVersionOrdering(t *testing.T) {
	tests := []struct {
		left  string
		right string
		want  int
	}{
		{"1.0.1", "1.0.0", 1},
		{"2.0.0", "10.0.0", -1},
		{"1.0.0", "1.0.0-rc.1", 1},
		{"1.0.0-rc.2", "1.0.0-rc.10", -1},
		{"1.0.0-alpha", "1.0.0-beta", -1},
		{"1.2.3+build.7", "1.2.3+build.8", 0},
	}
	for _, test := range tests {
		got, err := compareSemanticVersions(test.left, test.right)
		if err != nil || got != test.want {
			t.Fatalf("compare %s to %s: got=%d want=%d err=%v", test.left, test.right, got, test.want, err)
		}
	}
}

func TestSemanticVersionRejectsAmbiguousValues(t *testing.T) {
	for _, value := range []string{"v1.2.3", "1.02.3", "1.2", "1.2.3-01", "latest"} {
		if _, err := parseSemanticVersion(value); err == nil {
			t.Fatalf("accepted invalid version %q", value)
		}
	}
}

func TestSignedManifestRejectsTamperingAndUnknownFields(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	manifest := Manifest{Version: "1.1.0", Channel: "stable", Sequence: 2, SHA256: hex.EncodeToString(make([]byte, 32)), Size: 10, URL: "https://updates.example/agent", MinimumVersion: "1.0.0", ExpiresAt: time.Now().Add(time.Hour).UTC()}
	unsigned, _ := json.Marshal(manifest)
	manifest.Signature = ed25519.Sign(privateKey, unsigned)
	raw, _ := json.Marshal(manifest)
	if _, err := loadManifest(raw, hex.EncodeToString(publicKey)); err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}
	manifest.Size++
	tampered, _ := json.Marshal(manifest)
	if _, err := loadManifest(tampered, hex.EncodeToString(publicKey)); err == nil {
		t.Fatal("tampered manifest accepted")
	}
	withUnknown := append(raw[:len(raw)-1], []byte(`,"unexpected":true}`)...)
	if _, err := loadManifest(withUnknown, hex.EncodeToString(publicKey)); err == nil {
		t.Fatal("unknown manifest field accepted")
	}
}
