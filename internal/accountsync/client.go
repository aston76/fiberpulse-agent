package accountsync

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"fiberpulse.dev/agent/internal/measurement"
)

var ErrAuthorizationPending = errors.New("account authorization pending")

type Client struct {
	baseURL string
	http    *http.Client
}

type DeviceRequest struct {
	DeviceCode      string `json:"deviceCode"`
	UserCode        string `json:"userCode"`
	VerificationURL string `json:"verificationUrl"`
	ExpiresIn       int    `json:"expiresIn"`
	Interval        int    `json:"interval"`
}

type Account struct {
	Email              string `json:"email"`
	Plan               string `json:"plan"`
	SubscriptionStatus string `json:"subscriptionStatus"`
	GoogleLinked       bool   `json:"googleLinked"`
}

type Exchange struct {
	AccessToken string  `json:"accessToken"`
	ExpiresIn   int     `json:"expiresIn"`
	Account     Account `json:"account"`
}

type measurementPayload struct {
	ID           string  `json:"id"`
	MeasuredAt   string  `json:"measuredAt"`
	DownloadMbps float64 `json:"downloadMbps"`
	UploadMbps   float64 `json:"uploadMbps"`
	LatencyMs    float64 `json:"latencyMs"`
}

func New(baseURL string, transport http.RoundTripper) (*Client, error) {
	parsed, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("account API URL must be an HTTPS origin")
	}
	if transport == nil {
		transport = http.DefaultTransport
	}
	return &Client{baseURL: parsed.String(), http: &http.Client{Transport: transport, Timeout: 20 * time.Second}}, nil
}

func (c *Client) Start(ctx context.Context, deviceName string) (DeviceRequest, error) {
	var result DeviceRequest
	err := c.request(ctx, http.MethodPost, "/v1/device/start", "", map[string]string{"deviceName": deviceName}, &result)
	if err != nil {
		return DeviceRequest{}, err
	}
	if result.DeviceCode == "" || result.UserCode == "" || result.VerificationURL == "" {
		return DeviceRequest{}, errors.New("account service returned an incomplete device request")
	}
	return result, nil
}

func (c *Client) Exchange(ctx context.Context, deviceCode string) (Exchange, error) {
	var result Exchange
	err := c.request(ctx, http.MethodPost, "/v1/device/exchange", "", map[string]string{"deviceCode": deviceCode}, &result)
	if err != nil {
		var statusError *statusError
		if errors.As(err, &statusError) && statusError.Status == http.StatusPreconditionRequired {
			return Exchange{}, ErrAuthorizationPending
		}
		return Exchange{}, err
	}
	if result.AccessToken == "" || result.Account.Email == "" {
		return Exchange{}, errors.New("account service returned an incomplete authorization")
	}
	return result, nil
}

func (c *Client) Session(ctx context.Context, token string) (Account, error) {
	var result Account
	if err := c.request(ctx, http.MethodGet, "/v1/device/session", token, nil, &result); err != nil {
		return Account{}, err
	}
	return result, nil
}

func (c *Client) Logout(ctx context.Context, token string) error {
	return c.request(ctx, http.MethodPost, "/v1/device/logout", token, map[string]any{}, nil)
}

func (c *Client) Upload(ctx context.Context, token string, results []measurement.Result) error {
	items := make([]measurementPayload, 0, len(results))
	for _, result := range results {
		if result.Status != measurement.StatusComplete || result.ID == "" || result.StartedAt.IsZero() {
			continue
		}
		items = append(items, measurementPayload{
			ID: result.ID, MeasuredAt: result.StartedAt.UTC().Format(time.RFC3339Nano),
			DownloadMbps: float64(result.DownloadBPS) / 1_000_000,
			UploadMbps:   float64(result.UploadBPS) / 1_000_000,
			LatencyMs:    float64(result.MinRTTUS) / 1_000,
		})
	}
	for len(items) > 0 {
		count := min(100, len(items))
		if err := c.request(ctx, http.MethodPost, "/v1/device/measurements", token, map[string]any{"items": items[:count]}, nil); err != nil {
			return err
		}
		items = items[count:]
	}
	return nil
}

type statusError struct {
	Status int
	Code   string
}

func (e *statusError) Error() string {
	return fmt.Sprintf("account service rejected the request (%d, %s)", e.Status, e.Code)
}

func (c *Client) request(ctx context.Context, method, path, token string, payload, target any) error {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := c.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	limited := io.LimitReader(response.Body, 1<<20)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var problem struct {
			Code string `json:"code"`
		}
		_ = json.NewDecoder(limited).Decode(&problem)
		return &statusError{Status: response.StatusCode, Code: problem.Code}
	}
	if target == nil || response.StatusCode == http.StatusNoContent {
		_, _ = io.Copy(io.Discard, limited)
		return nil
	}
	return json.NewDecoder(limited).Decode(target)
}
