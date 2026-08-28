package measurement

import (
	"context"
	"errors"
	"time"
)

const (
	ProviderMLabNDT7        = "mlab_ndt7"
	ProviderDevelopmentFake = "development_fake"
	SchemaVersion           = "measurement-v1"
	MethodologyVersion      = "methodology-v1"
	ConfidenceVersion       = "confidence-v1"
)

var (
	ErrConsentRequired   = errors.New("M-Lab consent is required")
	ErrProviderDisabled  = errors.New("measurement provider is disabled")
	ErrNetworkIneligible = errors.New("network is not eligible for a performance test")
)

type Status string

const (
	StatusComplete  Status = "complete"
	StatusPartial   Status = "partial"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

type ConnectionType string

const (
	ConnectionEthernet ConnectionType = "ethernet"
	ConnectionWiFi     ConnectionType = "wifi"
	ConnectionCellular ConnectionType = "cellular"
	ConnectionOther    ConnectionType = "other"
	ConnectionUnknown  ConnectionType = "unknown"
)

type NetworkContext struct {
	InterfaceID    string         `json:"interface_id,omitempty"`
	RouteID        string         `json:"route_id,omitempty"`
	ConnectionType ConnectionType `json:"connection_type"`
	WiFiQuality    int            `json:"wifi_quality,omitempty"`
	Metered        bool           `json:"metered"`
	Roaming        bool           `json:"roaming"`
	VPNDetected    bool           `json:"vpn_suspected"`
	ProxyDetected  bool           `json:"proxy_suspected"`
	Online         bool           `json:"online"`
	CapturedAt     time.Time      `json:"captured_at"`
}

type PreflightResult struct {
	Eligible bool           `json:"eligible"`
	Reasons  []string       `json:"reasons,omitempty"`
	Network  NetworkContext `json:"network"`
}

type Progress struct {
	Phase        string `json:"phase"`
	Bytes        int64  `json:"bytes"`
	ElapsedUS    int64  `json:"elapsed_us"`
	EstimatedBPS int64  `json:"estimated_bps"`
}

type Result struct {
	ID                 string         `json:"id"`
	Provider           string         `json:"provider"`
	ProtocolVersion    string         `json:"protocol_version"`
	ClientVersion      string         `json:"client_version"`
	SchemaVersion      string         `json:"schema_version"`
	MethodologyVersion string         `json:"methodology_version"`
	ConfidenceVersion  string         `json:"confidence_version"`
	StartedAt          time.Time      `json:"started_at"`
	CompletedAt        time.Time      `json:"completed_at"`
	ServerFQDN         string         `json:"server_fqdn,omitempty"`
	DownloadBPS        int64          `json:"download_bps"`
	UploadBPS          int64          `json:"upload_bps"`
	MinRTTUS           int64          `json:"min_rtt_us"`
	BytesDown          int64          `json:"bytes_down"`
	BytesUp            int64          `json:"bytes_up"`
	DownloadDurationUS int64          `json:"download_duration_us"`
	UploadDurationUS   int64          `json:"upload_duration_us"`
	Status             Status         `json:"status"`
	ErrorCode          string         `json:"error_code,omitempty"`
	ErrorDetail        string         `json:"error_detail,omitempty"`
	NetworkBefore      NetworkContext `json:"network_before"`
	NetworkAfter       NetworkContext `json:"network_after"`
	ConfidenceScore    int            `json:"confidence_score"`
	ConfidenceLevel    string         `json:"confidence_level"`
	ConfidenceReasons  []string       `json:"confidence_reasons,omitempty"`
	PublicEligible     bool           `json:"public_eligible"`
}

type Metadata struct {
	Name            string `json:"name"`
	ProtocolVersion string `json:"protocol_version"`
	ClientVersion   string `json:"client_version"`
	Enabled         bool   `json:"enabled"`
}

type Provider interface {
	Capabilities() []string
	Preflight(context.Context, NetworkContext, bool) (PreflightResult, error)
	Run(context.Context, NetworkContext, func(Progress)) (Result, error)
	Metadata() Metadata
}
