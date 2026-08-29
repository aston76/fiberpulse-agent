package complaint

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"net/textproto"
	"sort"
	"strings"
	"time"

	"fiberpulse.dev/agent/internal/measurement"
	"fiberpulse.dev/agent/internal/plan"
)

const (
	TargetTests       = 21
	TargetDays        = 7
	minimumConfidence = 70
	windowDuration    = 7 * 24 * time.Hour
)

type Profile struct {
	FullName              string `json:"full_name"`
	AccountNumber         string `json:"account_number"`
	ServiceAddress        string `json:"service_address"`
	ContactEmail          string `json:"contact_email"`
	ContactPhone          string `json:"contact_phone"`
	ProviderModem         string `json:"provider_modem"`
	ProviderRouter        string `json:"provider_router"`
	AdditionalRouter      bool   `json:"additional_router"`
	AdditionalRouterModel string `json:"additional_router_model"`
	MeshSystem            bool   `json:"mesh_system"`
	MeshModel             string `json:"mesh_model"`
	TestConnection        string `json:"test_connection"`
	NetworkLayout         string `json:"network_layout"`
	TypicalDeviceCount    string `json:"typical_device_count"`
	Notes                 string `json:"notes"`
	SupportEmailOverride  string `json:"support_email_override"`
	SupportPhoneOverride  string `json:"support_phone_override"`
}

type SupportContact struct {
	ISP        string `json:"isp"`
	Email      string `json:"email,omitempty"`
	Phone      string `json:"phone,omitempty"`
	SupportURL string `json:"support_url"`
	SourceURL  string `json:"source_url"`
	VerifiedAt string `json:"verified_at"`
	Note       string `json:"note,omitempty"`
}

type Assessment struct {
	WindowStart            time.Time `json:"window_start"`
	WindowEnd              time.Time `json:"window_end"`
	FirstQualifiedAt       time.Time `json:"first_qualified_at,omitempty"`
	LastQualifiedAt        time.Time `json:"last_qualified_at,omitempty"`
	TargetTests            int       `json:"target_tests"`
	QualifiedTests         int       `json:"qualified_tests"`
	TestsRemaining         int       `json:"tests_remaining"`
	TargetDays             int       `json:"target_days"`
	ObservedDays           int       `json:"observed_days"`
	DaysRemaining          int       `json:"days_remaining"`
	WiredTests             int       `json:"wired_tests"`
	WiFiTests              int       `json:"wifi_tests"`
	MedianDownloadMbps     float64   `json:"median_download_mbps"`
	MedianUploadMbps       float64   `json:"median_upload_mbps"`
	MedianLatencyMS        float64   `json:"median_latency_ms"`
	AdvertisedDownloadMbps int       `json:"advertised_download_mbps"`
	DownloadPercent        int       `json:"download_percent"`
	EvidenceReady          bool      `json:"evidence_ready"`
	ProfileComplete        bool      `json:"profile_complete"`
	Underperforming        bool      `json:"underperforming"`
	ComplaintReady         bool      `json:"complaint_ready"`
	Status                 string    `json:"status"`
	Reasons                []string  `json:"reasons,omitempty"`
}

type Draft struct {
	To         string `json:"to,omitempty"`
	Subject    string `json:"subject"`
	Body       string `json:"body"`
	CallScript string `json:"call_script"`
	SupportURL string `json:"support_url,omitempty"`
	Ready      bool   `json:"ready"`
	Warning    string `json:"warning,omitempty"`
}

func ValidateProfile(input Profile) (Profile, error) {
	profile := input
	fields := []struct {
		name  string
		value *string
		max   int
	}{
		{"full name", &profile.FullName, 120},
		{"account number", &profile.AccountNumber, 80},
		{"service address", &profile.ServiceAddress, 300},
		{"contact email", &profile.ContactEmail, 160},
		{"contact phone", &profile.ContactPhone, 60},
		{"provider modem", &profile.ProviderModem, 120},
		{"provider router", &profile.ProviderRouter, 120},
		{"additional router model", &profile.AdditionalRouterModel, 120},
		{"mesh model", &profile.MeshModel, 120},
		{"typical device count", &profile.TypicalDeviceCount, 40},
		{"notes", &profile.Notes, 800},
		{"support email override", &profile.SupportEmailOverride, 160},
		{"support phone override", &profile.SupportPhoneOverride, 60},
	}
	for _, field := range fields {
		*field.value = strings.TrimSpace(*field.value)
		if len(*field.value) > field.max {
			return Profile{}, fmt.Errorf("%s must not exceed %d characters", field.name, field.max)
		}
		if hasUnsafeControl(*field.value) {
			return Profile{}, fmt.Errorf("%s contains unsupported control characters", field.name)
		}
	}
	for name, value := range map[string]string{"contact email": profile.ContactEmail, "support email override": profile.SupportEmailOverride} {
		if value == "" {
			continue
		}
		address, err := mail.ParseAddress(value)
		if err != nil || !strings.EqualFold(address.Address, value) {
			return Profile{}, fmt.Errorf("%s is invalid", name)
		}
	}
	if profile.TestConnection != "" && profile.TestConnection != "ethernet" && profile.TestConnection != "wifi" && profile.TestConnection != "mixed" {
		return Profile{}, errors.New("test connection must be ethernet, wifi, or mixed")
	}
	validLayouts := map[string]bool{"": true, "provider_router_direct": true, "provider_router_plus_own": true, "bridge_own_router": true, "mesh": true, "other": true}
	if !validLayouts[profile.NetworkLayout] {
		return Profile{}, errors.New("network layout is invalid")
	}
	if !profile.AdditionalRouter {
		profile.AdditionalRouterModel = ""
	}
	if !profile.MeshSystem {
		profile.MeshModel = ""
	}
	return profile, nil
}

func ProfileComplete(profile Profile) bool {
	return profile.FullName != "" && profile.AccountNumber != "" && profile.ServiceAddress != ""
}

func Assess(results []measurement.Result, offer *plan.Offer, profile Profile, now time.Time) Assessment {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()
	assessment := Assessment{
		WindowStart: now.Add(-windowDuration), WindowEnd: now,
		TargetTests: TargetTests, TargetDays: TargetDays,
		ProfileComplete: ProfileComplete(profile), Status: "collecting",
	}
	qualified := make([]measurement.Result, 0, len(results))
	for _, result := range results {
		if result.StartedAt.Before(assessment.WindowStart) || result.StartedAt.After(now) || !qualifies(result) {
			continue
		}
		qualified = append(qualified, result)
	}
	sort.Slice(qualified, func(i, j int) bool { return qualified[i].StartedAt.Before(qualified[j].StartedAt) })
	if len(qualified) > TargetTests {
		qualified = qualified[len(qualified)-TargetTests:]
	}
	days := make(map[string]struct{})
	phTime := time.FixedZone("PHT", 8*60*60)
	for _, result := range qualified {
		days[result.StartedAt.In(phTime).Format("2006-01-02")] = struct{}{}
		switch result.NetworkBefore.ConnectionType {
		case measurement.ConnectionEthernet:
			assessment.WiredTests++
		case measurement.ConnectionWiFi:
			assessment.WiFiTests++
		}
	}
	assessment.QualifiedTests = len(qualified)
	assessment.ObservedDays = len(days)
	assessment.TestsRemaining = max(0, TargetTests-assessment.QualifiedTests)
	assessment.DaysRemaining = max(0, TargetDays-assessment.ObservedDays)
	if len(qualified) > 0 {
		assessment.FirstQualifiedAt = qualified[0].StartedAt
		assessment.LastQualifiedAt = qualified[len(qualified)-1].StartedAt
		download, upload, latency := medianMetrics(qualified)
		assessment.MedianDownloadMbps = download / 1e6
		assessment.MedianUploadMbps = upload / 1e6
		assessment.MedianLatencyMS = latency / 1e3
	}
	assessment.EvidenceReady = assessment.QualifiedTests >= TargetTests && assessment.ObservedDays >= TargetDays
	if offer != nil {
		assessment.AdvertisedDownloadMbps = plan.EffectiveDownloadMbps(*offer, now)
		if assessment.AdvertisedDownloadMbps > 0 && assessment.MedianDownloadMbps > 0 {
			assessment.DownloadPercent = int(assessment.MedianDownloadMbps * 100 / float64(assessment.AdvertisedDownloadMbps))
		}
		assessment.Underperforming = assessment.DownloadPercent > 0 && assessment.DownloadPercent < 70
	}
	assessment.ComplaintReady = assessment.EvidenceReady && assessment.ProfileComplete && offer != nil && assessment.Underperforming
	switch {
	case offer == nil:
		assessment.Status = "plan_required"
		assessment.Reasons = append(assessment.Reasons, "Select the Internet offer shown on the latest bill.")
	case !assessment.EvidenceReady:
		assessment.Status = "collecting"
		if assessment.TestsRemaining > 0 {
			assessment.Reasons = append(assessment.Reasons, fmt.Sprintf("Collect %d more qualified test(s).", assessment.TestsRemaining))
		}
		if assessment.DaysRemaining > 0 {
			assessment.Reasons = append(assessment.Reasons, fmt.Sprintf("Continue monitoring on %d more day(s).", assessment.DaysRemaining))
		}
	case !assessment.ProfileComplete:
		assessment.Status = "profile_required"
		assessment.Reasons = append(assessment.Reasons, "Add the account holder, account number, and service address.")
	case !assessment.Underperforming:
		assessment.Status = "performance_normal"
		assessment.Reasons = append(assessment.Reasons, "The seven-day median is not currently below the conservative 70% threshold.")
	default:
		assessment.Status = "ready"
	}
	return assessment
}

func BuildDraft(profile Profile, offer *plan.Offer, contact SupportContact, assessment Assessment) Draft {
	isp := contact.ISP
	planName := "selected Internet service"
	if offer != nil {
		isp = offer.ISP
		planName = offer.Name
	}
	if isp == "" {
		isp = "Internet provider"
	}
	to := contact.Email
	if profile.SupportEmailOverride != "" {
		to = profile.SupportEmailOverride
	}
	accountRef := profile.AccountNumber
	if accountRef == "" {
		accountRef = "account number to be added"
	}
	subject := fmt.Sprintf("Request for technical investigation - %s - account %s", planName, accountRef)
	period := "the current monitoring period"
	if !assessment.FirstQualifiedAt.IsZero() && !assessment.LastQualifiedAt.IsZero() {
		period = assessment.FirstQualifiedAt.Format("2006-01-02") + " to " + assessment.LastQualifiedAt.Format("2006-01-02")
	}
	body := fmt.Sprintf(`Dear %s Support,

I am requesting a technical investigation into the performance of my Internet connection.

Subscriber details
- Account holder: %s
- Account number: %s
- Service address: %s
- Contact email: %s
- Contact phone: %s

Subscribed service
- Provider: %s
- Offer: %s
- Advertised download: up to %d Mbps

FiberPulse monitoring summary
- Monitoring period: %s
- Qualified measurements: %d across %d day(s)
- Median download: %.1f Mbps (%d%% of the advertised speed)
- Median upload: %.1f Mbps
- Median minimum latency: %.1f ms
- Ethernet tests: %d
- Wi-Fi tests: %d

Installation and test conditions
- Provider modem / ONT: %s
- Provider router: %s
- Additional router: %s
- Mesh system: %s
- Main test connection: %s
- Network layout: %s
- Typical connected devices: %s
- Additional notes: %s

The attached FiberPulse PDF contains the individual measurements and methodology notes. The tests are application-level M-Lab NDT7 measurements and are provided as diagnostic evidence, not as proof of physical line capacity.

Please review the line provisioning, ONT or modem status, optical signal where applicable, router configuration, and possible congestion. Please create a technical support ticket and reply with the ticket reference and your findings.

Kind regards,
%s`,
		isp, valueOr(profile.FullName, "Not provided"), accountRef, valueOr(profile.ServiceAddress, "Not provided"), valueOr(profile.ContactEmail, "Not provided"), valueOr(profile.ContactPhone, "Not provided"),
		isp, planName, assessment.AdvertisedDownloadMbps, period, assessment.QualifiedTests, assessment.ObservedDays, assessment.MedianDownloadMbps, assessment.DownloadPercent, assessment.MedianUploadMbps, assessment.MedianLatencyMS, assessment.WiredTests, assessment.WiFiTests,
		valueOr(profile.ProviderModem, "Not provided"), valueOr(profile.ProviderRouter, "Not provided"), routerDescription(profile.AdditionalRouter, profile.AdditionalRouterModel), routerDescription(profile.MeshSystem, profile.MeshModel), valueOr(profile.TestConnection, "Not provided"), valueOr(strings.ReplaceAll(profile.NetworkLayout, "_", " "), "Not provided"), valueOr(profile.TypicalDeviceCount, "Not provided"), valueOr(profile.Notes, "None"), valueOr(profile.FullName, "Subscriber"))
	callScript := fmt.Sprintf("Hello, I am %s. My account number is %s and my plan is %s, advertised at up to %d Mbps. FiberPulse collected %d qualified tests across %d days. The median download was %.1f Mbps, or %d%% of the plan speed. Please create a technical investigation ticket and give me the reference number. I can provide the PDF report with every measurement.", valueOr(profile.FullName, "the account holder"), accountRef, planName, assessment.AdvertisedDownloadMbps, assessment.QualifiedTests, assessment.ObservedDays, assessment.MedianDownloadMbps, assessment.DownloadPercent)
	warning := "Evidence collection or subscriber details are not complete yet. Review the draft, but wait until the dossier is ready before sending it as a formal complaint."
	if assessment.ComplaintReady {
		warning = ""
	}
	return Draft{To: to, Subject: subject, Body: body, CallScript: callScript, SupportURL: contact.SupportURL, Ready: assessment.ComplaintReady, Warning: warning}
}

func EML(draft Draft, pdf []byte) ([]byte, error) {
	if len(pdf) < 5 || !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		return nil, errors.New("a valid PDF attachment is required")
	}
	var out bytes.Buffer
	boundary := "fiberpulse-evidence-boundary"
	out.WriteString("MIME-Version: 1.0\r\n")
	if draft.To != "" {
		out.WriteString("To: " + headerValue(draft.To) + "\r\n")
	}
	out.WriteString("Subject: " + mime.QEncoding.Encode("UTF-8", headerValue(draft.Subject)) + "\r\n")
	out.WriteString("Content-Type: multipart/mixed; boundary=\"" + boundary + "\"\r\n\r\n")
	w := multipart.NewWriter(&out)
	if err := w.SetBoundary(boundary); err != nil {
		return nil, err
	}
	textHeader := make(textproto.MIMEHeader)
	textHeader.Set("Content-Type", "text/plain; charset=UTF-8")
	textHeader.Set("Content-Transfer-Encoding", "quoted-printable")
	textPart, err := w.CreatePart(textHeader)
	if err != nil {
		return nil, err
	}
	quoted := quotedprintable.NewWriter(textPart)
	if _, err := quoted.Write([]byte(draft.Body)); err != nil {
		return nil, err
	}
	if err := quoted.Close(); err != nil {
		return nil, err
	}
	attachmentHeader := make(textproto.MIMEHeader)
	attachmentHeader.Set("Content-Type", "application/pdf; name=\"fiberpulse-complaint-report.pdf\"")
	attachmentHeader.Set("Content-Disposition", "attachment; filename=\"fiberpulse-complaint-report.pdf\"")
	attachmentHeader.Set("Content-Transfer-Encoding", "base64")
	attachment, err := w.CreatePart(attachmentHeader)
	if err != nil {
		return nil, err
	}
	encoded := base64.StdEncoding.EncodeToString(pdf)
	for len(encoded) > 76 {
		if _, err := fmt.Fprintf(attachment, "%s\r\n", encoded[:76]); err != nil {
			return nil, err
		}
		encoded = encoded[76:]
	}
	if _, err := fmt.Fprintf(attachment, "%s\r\n", encoded); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func ContactForISP(isp string) SupportContact {
	normalized := strings.ToLower(strings.TrimSpace(isp))
	contacts := map[string]SupportContact{
		"converge ict":    {ISP: "Converge ICT", Email: "customercare@convergeict.com", Phone: "(02) 8667 0850 / 0919 057 2428", SupportURL: "https://legacy.convergeict.com/support/i-need-help/", SourceURL: "https://corporate.convergeict.com/privacy-notice", VerifiedAt: "2026-08-30"},
		"pldt home":       {ISP: "PLDT Home", Phone: "171", SupportURL: "https://www.pldthome.com/contact", SourceURL: "https://www.pldthome.com/contact", VerifiedAt: "2026-08-30", Note: "PLDT directs residential concerns to hotline 171 and PLDT Cares."},
		"globe at home":   {ISP: "Globe AT HOME", Phone: "211 / (02) 7730-1000", SupportURL: "https://www.globe.com.ph/contact-us", SourceURL: "https://www.globe.com.ph/contact-us", VerifiedAt: "2026-08-30", Note: "Consumer broadband concerns are handled through GlobeOne, chat, or the published hotlines."},
		"dito":            {ISP: "DITO", Email: "customerservice@dito.ph", Phone: "185", SupportURL: "https://dito.ph/help-center?tab=askdito", SourceURL: "https://dito.ph/help-center?tab=askdito", VerifiedAt: "2026-08-30"},
		"cablelink":       {ISP: "Cablelink", Phone: "(02) 8988-5465 / 0919 065 3200", SupportURL: "https://www.cablelink.com.ph/", SourceURL: "https://www.cablelink.com.ph/", VerifiedAt: "2026-08-30"},
		"surf2sawa":       {ISP: "Surf2Sawa", Phone: "(02) 8667 0850", SupportURL: "https://surf2sawa.com/", SourceURL: "https://corporate.convergeict.com/newsroom/surf2-sawa-by-converge-furthers-drive-for-affordable-internet-services-surpasses-500-000-active-prepaid-broadband-subscribers-", VerifiedAt: "2026-08-30", Note: "Use the official Surf2Sawa channel or the local servicing dealer; the listed hotline is Converge customer experience."},
		"padeco":          {ISP: "PADECO", Email: "sales@padeco.com.ph", Phone: "0919 059 8936 / (074) 300 6506", SupportURL: "https://padeco.com.ph/support.html", SourceURL: "https://padeco.com.ph/support.html", VerifiedAt: "2026-08-30"},
		"ludeco":          {ISP: "LUDECO", Email: "support@ludeco.com.ph", Phone: "(072) 603-1045 / 0919 059 8938", SupportURL: "https://www.ludeco.com.ph/", SourceURL: "https://www.ludeco.com.ph/", VerifiedAt: "2026-08-30"},
		"jquest network":  {ISP: "JQuest Network", Phone: "0956 400 9568", SupportURL: "https://jquestnetwork.com/", SourceURL: "https://jquestnetwork.com/", VerifiedAt: "2026-08-30"},
		"fiberbro makati": {ISP: "Fiberbro Makati", SupportURL: "https://fiberbro-makati.com/", SourceURL: "https://fiberbro-makati.com/", VerifiedAt: "2026-08-30"},
		"ikonnect":        {ISP: "iKonnect", SupportURL: "https://ikonnect.ph/", SourceURL: "https://ikonnect.ph/", VerifiedAt: "2026-08-30"},
		"tech net 88":     {ISP: "Tech Net 88", SupportURL: "https://www.technet88.com/", SourceURL: "https://www.technet88.com/", VerifiedAt: "2026-08-30"},
	}
	if contact, ok := contacts[normalized]; ok {
		return contact
	}
	return SupportContact{ISP: strings.TrimSpace(isp), VerifiedAt: "2026-08-30", Note: "No verified built-in support contact is available. Add the email or phone shown on the latest bill."}
}

func qualifies(result measurement.Result) bool {
	return result.Provider == measurement.ProviderMLabNDT7 && result.Status == measurement.StatusComplete && result.DownloadBPS > 0 && result.ConfidenceScore >= minimumConfidence && result.PublicEligible && !result.NetworkBefore.VPNDetected && !result.NetworkBefore.ProxyDetected && !result.NetworkBefore.Metered && !result.NetworkBefore.Roaming
}

func medianMetrics(results []measurement.Result) (download, upload, latency float64) {
	if len(results) == 0 {
		return 0, 0, 0
	}
	down := make([]float64, 0, len(results))
	up := make([]float64, 0, len(results))
	rtt := make([]float64, 0, len(results))
	for _, result := range results {
		down = append(down, float64(result.DownloadBPS))
		up = append(up, float64(result.UploadBPS))
		rtt = append(rtt, float64(result.MinRTTUS))
	}
	return median(down), median(up), median(rtt)
}

func median(values []float64) float64 {
	sort.Float64s(values)
	middle := len(values) / 2
	if len(values)%2 == 1 {
		return values[middle]
	}
	return (values[middle-1] + values[middle]) / 2
}

func routerDescription(enabled bool, model string) string {
	if !enabled {
		return "No"
	}
	if model == "" {
		return "Yes - model not provided"
	}
	return "Yes - " + model
}

func valueOr(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func hasUnsafeControl(value string) bool {
	for _, char := range value {
		if char < 32 && char != '\n' && char != '\r' && char != '\t' {
			return true
		}
	}
	return false
}

func headerValue(value string) string {
	return strings.NewReplacer("\r", " ", "\n", " ").Replace(value)
}
