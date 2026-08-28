package sharing

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

type Identity struct {
	Public  ed25519.PublicKey
	Private ed25519.PrivateKey
}

func NewIdentity() (Identity, error) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return Identity{}, err
	}
	return Identity{Public: public, Private: private}, nil
}

func InstallationID(public ed25519.PublicKey) string {
	sum := sha256.Sum256(public)
	return hex.EncodeToString(sum[:16])
}

func SigningMessage(method, path, timestamp, nonce string, sequence uint64, body []byte) []byte {
	hash := sha256.Sum256(body)
	return []byte(fmt.Sprintf("%s\n%s\n%s\n%s\n%d\n%x", strings.ToUpper(method), path, timestamp, nonce, sequence, hash))
}

func (i Identity) Sign(method, path, timestamp, nonce string, sequence uint64, body []byte) (string, error) {
	if len(i.Private) != ed25519.PrivateKeySize {
		return "", errors.New("invalid private key")
	}
	return base64.StdEncoding.EncodeToString(ed25519.Sign(i.Private, SigningMessage(method, path, timestamp, nonce, sequence, body))), nil
}

func Verify(public ed25519.PublicKey, signature, method, path, timestamp, nonce string, sequence uint64, body []byte) bool {
	raw, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return false
	}
	return ed25519.Verify(public, SigningMessage(method, path, timestamp, nonce, sequence, body), raw)
}
