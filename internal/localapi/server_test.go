package localapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

type testController struct{ actions int }

func (c *testController) Snapshot(context.Context) (any, error) {
	return map[string]bool{"ok": true}, nil
}
func (c *testController) Action(context.Context, string, json.RawMessage) error {
	c.actions++
	return nil
}
func (c *testController) Export(context.Context, string) ([]byte, string, error) {
	return []byte("ok"), "text/plain", nil
}

func TestBootstrapSessionHostAndCSRF(t *testing.T) {
	controller := &testController{}
	server := New(controller)
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	defer server.Close(context.Background())
	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	bootstrap := server.BootstrapURL()
	response, err := client.Get(bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("bootstrap status %d", response.StatusCode)
	}
	cookies := response.Cookies()
	response.Body.Close()
	if len(cookies) != 1 || !cookies[0].HttpOnly {
		t.Fatalf("invalid session cookie: %+v", cookies)
	}
	response, err = client.Get(bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("reused bootstrap status %d", response.StatusCode)
	}
	response.Body.Close()
	statusURL := server.BaseURL() + "/api/v1/status"
	request, _ := http.NewRequest(http.MethodGet, statusURL, nil)
	request.AddCookie(cookies[0])
	response, err = client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(response.Body)
	response.Body.Close()
	var envelope struct {
		CSRF string `json:"csrf_token"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || envelope.CSRF == "" {
		t.Fatalf("status body %s err=%v", raw, err)
	}
	actionURL := server.BaseURL() + "/api/v1/actions/test"
	request, _ = http.NewRequest(http.MethodPost, actionURL, bytes.NewBufferString(`{}`))
	request.AddCookie(cookies[0])
	request.Header.Set("Content-Type", "application/json")
	response, err = client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("missing csrf status %d", response.StatusCode)
	}
	response.Body.Close()
	request, _ = http.NewRequest(http.MethodPost, actionURL, bytes.NewBufferString(`{}`))
	request.AddCookie(cookies[0])
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", server.BaseURL())
	request.Header.Set("X-CSRF-Token", envelope.CSRF)
	response, err = client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusAccepted || controller.actions != 1 {
		t.Fatalf("valid action status=%d actions=%d", response.StatusCode, controller.actions)
	}
	response.Body.Close()
	exportURL := server.BaseURL() + "/api/v1/export/csv"
	request, _ = http.NewRequest(http.MethodGet, exportURL, nil)
	request.AddCookie(cookies[0])
	response, err = client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	exported, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != http.StatusOK || string(exported) != "ok" {
		t.Fatalf("export status=%d body=%q", response.StatusCode, exported)
	}
	if disposition := response.Header.Get("Content-Disposition"); disposition != "attachment; filename=fiberpulse-report.csv" {
		t.Fatalf("unexpected content disposition %q", disposition)
	}
	parsed, _ := url.Parse(statusURL)
	request, _ = http.NewRequest(http.MethodGet, statusURL, nil)
	request.Host = "attacker.invalid"
	request.URL.Host = parsed.Host
	request.AddCookie(cookies[0])
	response, err = client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusMisdirectedRequest {
		t.Fatalf("spoofed host status %d", response.StatusCode)
	}
	response.Body.Close()
}

func TestStaticSecurityHeaders(t *testing.T) {
	server := New(&testController{})
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	defer server.Close(context.Background())
	client := &http.Client{}
	response, err := client.Get(server.BootstrapURL())
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	response, err = client.Get(server.BaseURL() + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if !strings.Contains(response.Header.Get("Content-Security-Policy"), "frame-ancestors 'none'") {
		t.Fatal("missing restrictive CSP")
	}
	if response.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("missing nosniff")
	}
}

func TestUnauthenticatedBrowserGetsRecoveryInstructions(t *testing.T) {
	server := New(&testController{})
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	defer server.Close(context.Background())
	response, err := http.Get(server.BaseURL() + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized || !strings.Contains(string(body), "menu-bar or system-tray icon") {
		t.Fatalf("status=%d body=%q", response.StatusCode, body)
	}
	response, err = http.Get(server.BaseURL() + "/api/v1/status")
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized || !strings.Contains(string(body), "authentication.required") {
		t.Fatalf("API status=%d body=%q", response.StatusCode, body)
	}
}
