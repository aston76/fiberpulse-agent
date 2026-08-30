package complaint

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"
	"testing"
	"time"

	"fiberpulse.dev/agent/internal/measurement"
	"fiberpulse.dev/agent/internal/plan"
)

func TestAssessmentRequiresSevenDaysAndTwentyOneQualifiedTests(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	results := make([]measurement.Result, 0, TargetTests)
	for day := 0; day < TargetDays; day++ {
		for sample := 0; sample < 3; sample++ {
			results = append(results, measurement.Result{
				Provider: measurement.ProviderMLabNDT7, Status: measurement.StatusComplete,
				StartedAt:   now.AddDate(0, 0, -(TargetDays - 1 - day)).Add(time.Duration(sample*2-6) * time.Hour),
				DownloadBPS: 400_000_000, UploadBPS: 350_000_000, MinRTTUS: 11_000,
				ConfidenceScore: 95, PublicEligible: true,
				NetworkBefore: measurement.NetworkContext{ConnectionType: measurement.ConnectionEthernet, Online: true},
			})
		}
	}
	offer, ok := plan.Find("converge-super-prime-2099")
	if !ok {
		t.Fatal("test offer missing")
	}
	profile := Profile{FullName: "Test Subscriber", AccountNumber: "ACC-123", ServiceAddress: "Test address", TestConnection: "ethernet"}
	assessment := Assess(results, &offer, profile, now.Add(13*time.Hour))
	if !assessment.EvidenceReady || !assessment.ComplaintReady || assessment.QualifiedTests != 21 || assessment.ObservedDays != 7 {
		t.Fatalf("unexpected assessment: %+v", assessment)
	}
	if assessment.DownloadPercent != 50 || assessment.MedianDownloadMbps != 400 || assessment.WiredTests != 21 {
		t.Fatalf("unexpected metrics: %+v", assessment)
	}

	assessment = Assess(results[:18], &offer, profile, now.Add(13*time.Hour))
	if assessment.EvidenceReady || assessment.ComplaintReady || assessment.TestsRemaining != 3 {
		t.Fatalf("short collection incorrectly ready: %+v", assessment)
	}
}

func TestProfileValidationAndContactOverride(t *testing.T) {
	profile, err := ValidateProfile(Profile{
		FullName: "  Test Subscriber ", AccountNumber: " ACC-123 ", ServiceAddress: " Cebu ",
		ContactEmail: "subscriber@example.com", SupportEmailOverride: "support@example.net",
		NetworkLayout: "provider_router_direct", TestConnection: "ethernet",
	})
	if err != nil {
		t.Fatal(err)
	}
	if profile.FullName != "Test Subscriber" || !ProfileComplete(profile) {
		t.Fatalf("profile was not normalized: %+v", profile)
	}
	if _, err := ValidateProfile(Profile{ContactEmail: "not-an-email"}); err == nil {
		t.Fatal("invalid email accepted")
	}
	contact := ContactForISP("Converge ICT")
	if contact.Email != "customercare@convergeict.com" || contact.Phone == "" || contact.SourceURL == "" {
		t.Fatalf("unexpected contact: %+v", contact)
	}
}

func TestEMLContainsDraftAndPDFAttachment(t *testing.T) {
	draft := Draft{To: "support@example.com", Subject: "Technical investigation", Body: "Please investigate the connection."}
	pdf := []byte("%PDF-1.4\nfixture")
	body, err := EML(draft, pdf)
	if err != nil {
		t.Fatal(err)
	}
	message, err := mail.ReadMessage(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	mediaType, params, err := mime.ParseMediaType(message.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/mixed" {
		t.Fatalf("invalid message content type: %q %v", mediaType, err)
	}
	reader := multipart.NewReader(message.Body, params["boundary"])
	var foundText, foundPDF bool
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(part)
		if err != nil {
			t.Fatal(err)
		}
		switch part.Header.Get("Content-Type") {
		case "text/plain; charset=UTF-8":
			foundText = strings.Contains(string(content), "Please investigate")
		case "application/pdf; name=\"fiberpulse-complaint-report.pdf\"":
			decoded, decodeErr := io.ReadAll(base64.NewDecoder(base64.StdEncoding, bytes.NewReader(content)))
			if decodeErr != nil {
				t.Fatal(decodeErr)
			}
			foundPDF = bytes.Equal(decoded, pdf)
		}
	}
	if !foundText || !foundPDF {
		t.Fatalf("missing EML parts text=%v pdf=%v", foundText, foundPDF)
	}
}

func TestLocalizedDraftsUseEverySupportedLanguage(t *testing.T) {
	profile := Profile{FullName: "Test Subscriber", AccountNumber: "ACC-123", ServiceAddress: "Test address"}
	assessment := Assessment{AdvertisedDownloadMbps: 500, QualifiedTests: 21, ObservedDays: 7, MedianDownloadMbps: 200, DownloadPercent: 40}
	expectations := map[string]string{
		"en": "Subscriber details", "fr": "Coordonnées de l’abonné", "de": "Kundendaten", "es": "Datos del abonado",
		"pt-BR": "Dados do assinante", "it": "Dati dell’abbonato", "hi": "ग्राहक विवरण",
	}
	for language, expected := range expectations {
		draft := BuildDraftLocalized(profile, nil, SupportContact{ISP: "Example ISP"}, assessment, language)
		if !strings.Contains(draft.Body, expected) {
			t.Errorf("localized draft %s does not contain %q", language, expected)
		}
	}
}
