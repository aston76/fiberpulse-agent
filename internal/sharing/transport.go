package sharing

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	registrationPath = "/api/v1/installations"
	measurementPath  = "/api/v1/measurements"
)

type Sender interface {
	Send(context.Context, Identity, uint64, []byte) error
}

type HTTPTransport struct {
	baseURL    string
	client     *http.Client
	mu         sync.Mutex
	registered map[string]bool
}

func NewHTTPTransport(endpoint string, client *http.Client) (*HTTPTransport, error) {
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(endpoint), "/"))
	if err != nil || parsed.Host == "" {
		return nil, errors.New("invalid sharing endpoint")
	}
	host := parsed.Hostname()
	if parsed.Scheme != "https" && !(parsed.Scheme == "http" && (host == "127.0.0.1" || host == "localhost" || host == "::1")) {
		return nil, errors.New("sharing endpoint must use HTTPS outside loopback")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil {
		return nil, errors.New("sharing endpoint must not contain credentials, query parameters or fragments")
	}
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}}
	}
	return &HTTPTransport{baseURL: parsed.String(), client: client, registered: make(map[string]bool)}, nil
}

func (t *HTTPTransport) Send(ctx context.Context, identity Identity, sequence uint64, body []byte) error {
	if len(identity.Public) != ed25519.PublicKeySize || len(identity.Private) != ed25519.PrivateKeySize {
		return errors.New("invalid sharing identity")
	}
	if len(body) == 0 || len(body) > 32<<10 {
		return errors.New("invalid sharing payload size")
	}
	installationID := InstallationID(identity.Public)
	if err := t.ensureRegistered(ctx, installationID, identity.Public); err != nil {
		return err
	}
	err := t.sendSigned(ctx, installationID, identity, sequence, body)
	if errors.Is(err, errUnknownInstallation) {
		t.mu.Lock()
		delete(t.registered, installationID)
		t.mu.Unlock()
		if registerErr := t.ensureRegistered(ctx, installationID, identity.Public); registerErr != nil {
			return registerErr
		}
		return t.sendSigned(ctx, installationID, identity, sequence, body)
	}
	return err
}

var errUnknownInstallation = errors.New("sharing installation is not registered")

func (t *HTTPTransport) ensureRegistered(ctx context.Context, installationID string, public ed25519.PublicKey) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.registered[installationID] {
		return nil
	}
	body, err := json.Marshal(map[string]string{
		"installation_id": installationID,
		"public_key":      base64.StdEncoding.EncodeToString(public),
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.baseURL+registrationPath, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "FiberPulse-Agent")
	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("register sharing installation: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("register sharing installation: HTTP %d", resp.StatusCode)
	}
	t.registered[installationID] = true
	return nil
}

func (t *HTTPTransport) sendSigned(ctx context.Context, installationID string, identity Identity, sequence uint64, body []byte) error {
	timestamp := time.Now().UTC().Format(time.RFC3339)
	nonce := uuid.NewString()
	signature, err := identity.Sign(http.MethodPost, measurementPath, timestamp, nonce, sequence, body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.baseURL+measurementPath, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "FiberPulse-Agent")
	req.Header.Set("X-FiberPulse-Installation", installationID)
	req.Header.Set("X-FiberPulse-Timestamp", timestamp)
	req.Header.Set("X-FiberPulse-Nonce", nonce)
	req.Header.Set("X-FiberPulse-Sequence", strconv.FormatUint(sequence, 10))
	req.Header.Set("X-FiberPulse-Signature", signature)
	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("send anonymous measurement: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
	if resp.StatusCode == http.StatusUnauthorized {
		return errUnknownInstallation
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("send anonymous measurement: HTTP %d", resp.StatusCode)
	}
	return nil
}
