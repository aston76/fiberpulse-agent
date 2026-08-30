package localapi

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"fiberpulse.dev/agent/internal/localization"
)

//go:embed web/*
var webFS embed.FS

type Controller interface {
	Snapshot(context.Context) (any, error)
	Action(context.Context, string, json.RawMessage) error
	Export(context.Context, string) ([]byte, string, error)
}

type Server struct {
	controller Controller
	listener   net.Listener
	httpServer *http.Server
	host       string
	baseURL    string
	mu         sync.Mutex
	bootstrap  string
	session    string
	csrf       string
}

func New(controller Controller) *Server { return &Server{controller: controller} }

func (s *Server) Start() error {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return err
	}
	s.listener = listener
	port := listener.Addr().(*net.TCPAddr).Port
	s.host = net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	s.baseURL = "http://" + s.host
	s.session = randomToken(32)
	s.csrf = randomToken(32)
	s.rotateBootstrap()
	mux := http.NewServeMux()
	mux.HandleFunc("/bootstrap/", s.handleBootstrap)
	mux.HandleFunc("/api/v1/status", s.auth(s.handleStatus))
	mux.HandleFunc("/api/v1/actions/", s.auth(s.handleAction))
	mux.HandleFunc("/api/v1/export/", s.auth(s.handleExport))
	assets, _ := fs.Sub(webFS, "web")
	mux.Handle("/", s.auth(s.secureStatic(http.FileServer(http.FS(assets)))))
	s.httpServer = &http.Server{Handler: s.hostGuard(mux), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	go func() { _ = s.httpServer.Serve(listener) }()
	return nil
}

func (s *Server) BootstrapURL() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rotateBootstrap()
	return s.baseURL + "/bootstrap/" + s.bootstrap
}
func (s *Server) BaseURL() string { return s.baseURL }
func (s *Server) Close(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) rotateBootstrap() { s.bootstrap = randomToken(32) }

func (s *Server) hostGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != s.host {
			http.Error(w, "invalid host", http.StatusMisdirectedRequest)
			return
		}
		setSecurityHeaders(w)
		next.ServeHTTP(w, r)
	})
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("fp_session")
		if err != nil || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(s.session)) != 1 {
			if !strings.HasPrefix(r.URL.Path, "/api/") && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusUnauthorized)
				if r.Method == http.MethodGet {
					_, _ = io.WriteString(w, "<!doctype html><html lang=\"en\"><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width\"><title>FiberPulse authorization required</title><main><h1>FiberPulse authorization required</h1><p>Open FiberPulse from its menu-bar or system-tray icon to create a fresh private dashboard session.</p><p>This page cannot authorize itself and no data has been exposed.</p></main></html>")
				}
				return
			}
			writeError(w, http.StatusUnauthorized, "authentication.required")
			return
		}
		next(w, r)
	}
}

func (s *Server) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	token := strings.TrimPrefix(r.URL.Path, "/bootstrap/")
	s.mu.Lock()
	valid := subtle.ConstantTimeCompare([]byte(token), []byte(s.bootstrap)) == 1 && s.bootstrap != ""
	if valid {
		s.bootstrap = ""
	}
	s.mu.Unlock()
	if !valid {
		http.Error(w, "expired bootstrap token", http.StatusUnauthorized)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "fp_session", Value: s.session, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: false, MaxAge: 8 * 60 * 60})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	data, err := s.controller.Snapshot(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "status.failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"csrf_token": s.csrf, "data": data})
}

func (s *Server) handleAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.validOrigin(r) || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(s.csrf)) != 1 {
		writeError(w, http.StatusForbidden, "csrf.invalid")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	var body json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, context.Canceled) {
		writeError(w, http.StatusBadRequest, "body.invalid")
		return
	}
	action := strings.TrimPrefix(r.URL.Path, "/api/v1/actions/")
	if err := s.controller.Action(r.Context(), action, body); err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": map[string]string{"code": "action.rejected", "detail": err.Error()}})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"accepted": true})
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.validOrigin(r) || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(s.csrf)) != 1 {
		writeError(w, http.StatusForbidden, "csrf.invalid")
		return
	}
	format := strings.TrimPrefix(r.URL.Path, "/api/v1/export/")
	body, contentType, err := s.controller.Export(localization.WithLanguage(r.Context(), r.URL.Query().Get("language")), format)
	if err != nil {
		writeError(w, http.StatusBadRequest, "export.failed")
		return
	}
	w.Header().Set("Content-Type", contentType)
	filename := map[string]string{
		"pdf": "fiberpulse-report.pdf", "csv": "fiberpulse-report.csv",
		"complaint-pdf": "fiberpulse-complaint-report.pdf",
		"complaint-eml": "fiberpulse-complaint-email.eml",
	}[format]
	if filename == "" {
		filename = fmt.Sprintf("fiberpulse-report.%s", format)
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (s *Server) validOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	return origin == s.baseURL
}
func (s *Server) secureStatic(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		next.ServeHTTP(w, r)
	}
}
func setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
	w.Header().Set("Cache-Control", "no-store")
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code}})
}
func randomToken(size int) string {
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		panic("secure random source unavailable: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
