package sharing

import "time"

// MeasurementEvent is the complete public-sharing contract. It intentionally
// has no fields for IP addresses, subscriber details, device names, SSIDs,
// exact coordinates or stable public installation identifiers.
type MeasurementEvent struct {
	EventID                string    `json:"event_id"`
	TimestampBucket        time.Time `json:"timestamp_bucket"`
	MeasurementProvider    string    `json:"measurement_provider"`
	ProtocolVersion        string    `json:"protocol_version"`
	AgentVersion           string    `json:"agent_version"`
	SchemaVersion          string    `json:"schema_version"`
	MethodologyVersion     string    `json:"methodology_version"`
	ConfidenceVersion      string    `json:"confidence_version"`
	ServerFQDN             string    `json:"server_fqdn,omitempty"`
	DownloadBPS            int64     `json:"download_bps"`
	UploadBPS              int64     `json:"upload_bps"`
	MinRTTUS               int64     `json:"min_rtt_us"`
	ConnectionType         string    `json:"connection_type"`
	WiFiQualityBucket      string    `json:"wifi_quality_bucket,omitempty"`
	ConfidenceScore        int       `json:"confidence_score"`
	ConfidenceLevel        string    `json:"confidence_level"`
	PublicEligible         bool      `json:"public_eligible"`
	PlanCountryCode        string    `json:"plan_country_code,omitempty"`
	PlanCountryName        string    `json:"plan_country_name,omitempty"`
	ISP                    string    `json:"isp,omitempty"`
	OfferName              string    `json:"offer_name,omitempty"`
	SubscriptionType       string    `json:"subscription_type,omitempty"`
	AdvertisedDownloadMbps int       `json:"advertised_download_mbps,omitempty"`
	AdvertisedUploadMbps   int       `json:"advertised_upload_mbps,omitempty"`
	CatalogOffer           bool      `json:"catalog_offer"`
}
