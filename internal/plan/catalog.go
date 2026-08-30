// Package plan holds the country-aware catalog of ISP offers and the
// conservative comparison between a subscribed offer and a measured NDT7
// result. The catalog is embedded from data/*.json snapshots of the canonical
// catalog maintained in fiberpulse-platform/data/catalog (ADR 0001); every
// offer carries its own country and verification metadata so new markets are
// added by data, without changing the API or persisted selections.
package plan

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

var ErrUnknownOffer = errors.New("unknown plan offer")

type Offer struct {
	ID           string  `json:"id"`
	CountryCode  string  `json:"country_code"`
	CountryName  string  `json:"country_name"`
	ISP          string  `json:"isp"`
	Name         string  `json:"name"`
	DownloadMbps int     `json:"download_mbps"`
	UploadMbps   int     `json:"upload_mbps"` // 0 when not advertised
	PriceAmount  float64 `json:"price_amount,omitempty"`
	CurrencyCode string  `json:"currency_code,omitempty"`
	PricePHP     int     `json:"price_php,omitempty"` // Legacy API field; use PriceAmount for new countries.
	PricePeriod  string  `json:"price_period,omitempty"`
	Category     string  `json:"category,omitempty"`
	Note         string  `json:"note,omitempty"`
	SourceURL    string  `json:"source_url,omitempty"`
	VerifiedAt   string  `json:"verified_at,omitempty"`
	Custom       bool    `json:"custom,omitempty"`
	DayMbps      int     `json:"day_mbps,omitempty"`
	NightMbps    int     `json:"night_mbps,omitempty"`
	// AdvertisedSpeedBasis records how the plan speed is legally marketed:
	// "up_to" (US, PH and most markets), "average" (UK Ofcom rule) or
	// "typical" (fixed-wireless ranges). The UI words comparisons accordingly.
	AdvertisedSpeedBasis string `json:"advertised_speed_basis,omitempty"`
}

const (
	PhilippinesCode     = "PH"
	PhilippinesName     = "Philippines"
	PhilippinesCurrency = "PHP"
)

// Catalog snapshots are embedded copies of the canonical per-country plan
// catalog maintained in fiberpulse-platform/data/catalog (plan-catalog-v1
// contract). Run `make catalog` to refresh them before a release.

//go:embed data/*.json
var catalogFS embed.FS

type catalogFile struct {
	SchemaVersion string  `json:"schema_version"`
	CountryCode   string  `json:"country_code"`
	CountryName   string  `json:"country_name"`
	CurrencyCode  string  `json:"currency_code"`
	SpeedBasis    string  `json:"advertised_speed_basis"`
	Offers        []Offer `json:"offers"`
}

const catalogSchemaVersion = "plan-catalog-v1"

var catalogData = loadCatalog()

func loadCatalog() []Offer {
	entries, err := catalogFS.ReadDir("data")
	if err != nil {
		panic(fmt.Sprintf("plan: read embedded catalog dir: %v", err))
	}
	var offers []Offer
	seen := map[string]string{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		raw, err := catalogFS.ReadFile("data/" + entry.Name())
		if err != nil {
			panic(fmt.Sprintf("plan: read embedded catalog %s: %v", entry.Name(), err))
		}
		var file catalogFile
		if err := json.Unmarshal(raw, &file); err != nil {
			panic(fmt.Sprintf("plan: parse embedded catalog %s: %v", entry.Name(), err))
		}
		if file.SchemaVersion != catalogSchemaVersion {
			panic(fmt.Sprintf("plan: embedded catalog %s has schema %q, want %q", entry.Name(), file.SchemaVersion, catalogSchemaVersion))
		}
		for _, offer := range file.Offers {
			offer = normalize(offer, file)
			if prev, dup := seen[offer.ID]; dup {
				panic(fmt.Sprintf("plan: duplicate offer id %q in %s (already in %s)", offer.ID, entry.Name(), prev))
			}
			seen[offer.ID] = entry.Name()
			offers = append(offers, offer)
		}
	}
	sort.SliceStable(offers, func(i, j int) bool {
		if (offers[i].CountryCode == PhilippinesCode) != (offers[j].CountryCode == PhilippinesCode) {
			return offers[i].CountryCode == PhilippinesCode
		}
		if offers[i].CountryName != offers[j].CountryName {
			return offers[i].CountryName < offers[j].CountryName
		}
		if offers[i].ISP != offers[j].ISP {
			return offers[i].ISP < offers[j].ISP
		}
		if offers[i].DownloadMbps != offers[j].DownloadMbps {
			return offers[i].DownloadMbps < offers[j].DownloadMbps
		}
		return offers[i].Name < offers[j].Name
	})
	return offers
}

// normalize applies the file-level defaults and keeps the legacy Philippine
// price field in sync for persisted selections and older report templates.
func normalize(offer Offer, file catalogFile) Offer {
	if offer.CountryCode == "" {
		offer.CountryCode = file.CountryCode
	}
	if offer.CountryName == "" {
		offer.CountryName = file.CountryName
	}
	if offer.CurrencyCode == "" && offer.PriceAmount > 0 {
		offer.CurrencyCode = file.CurrencyCode
	}
	if offer.AdvertisedSpeedBasis == "" {
		offer.AdvertisedSpeedBasis = file.SpeedBasis
	}
	// The Philippine snapshot carries whole-peso prices; keep the legacy
	// integer field in sync for persisted selections and report templates.
	if offer.CountryCode == PhilippinesCode && offer.PricePHP == 0 && offer.PriceAmount > 0 {
		offer.PricePHP = int(offer.PriceAmount)
	}
	return offer
}

// Catalog returns the embedded catalog snapshot. The Philippines stay first
// so upgrades from the Philippine-only build keep their default selection.
func Catalog() []Offer {
	return append([]Offer(nil), catalogData...)
}

// ValidateCustom checks a subscriber-entered offer. Custom plans cover
// regional ISPs, grandfathered offers and promotions without pretending that
// FiberPulse's built-in catalog can be permanently exhaustive.
func ValidateCustom(offer Offer) (Offer, error) {
	offer.CountryCode = strings.ToUpper(strings.TrimSpace(offer.CountryCode))
	offer.CountryName = strings.TrimSpace(offer.CountryName)
	if offer.CountryCode == "" && offer.CountryName == "" {
		// Preserve custom selections created by the Philippine-only build.
		offer.CountryCode = PhilippinesCode
		offer.CountryName = PhilippinesName
	}
	if len(offer.CountryCode) != 2 || offer.CountryCode[0] < 'A' || offer.CountryCode[0] > 'Z' || offer.CountryCode[1] < 'A' || offer.CountryCode[1] > 'Z' {
		return Offer{}, errors.New("custom country code must contain two ISO letters")
	}
	if offer.CountryName == "" || len(offer.CountryName) > 80 {
		return Offer{}, errors.New("custom country name must contain 1 to 80 characters")
	}
	offer.ISP = strings.TrimSpace(offer.ISP)
	offer.Name = strings.TrimSpace(offer.Name)
	if offer.ISP == "" || len(offer.ISP) > 80 {
		return Offer{}, errors.New("custom provider must contain 1 to 80 characters")
	}
	if offer.Name == "" || len(offer.Name) > 120 {
		return Offer{}, errors.New("custom offer must contain 1 to 120 characters")
	}
	if offer.DownloadMbps < 1 || offer.DownloadMbps > 10000 {
		return Offer{}, errors.New("custom download speed must be between 1 and 10000 Mbps")
	}
	if offer.UploadMbps < 0 || offer.UploadMbps > 10000 {
		return Offer{}, errors.New("custom upload speed must be between 0 and 10000 Mbps")
	}
	offer.ID = "custom"
	offer.PriceAmount = 0
	offer.CurrencyCode = ""
	offer.PricePHP = 0
	offer.PricePeriod = ""
	offer.Category = "Custom"
	offer.Note = "Entered by the subscriber; verify the advertised speed on the latest bill."
	offer.SourceURL = ""
	offer.VerifiedAt = ""
	offer.Custom = true
	return offer, nil
}

func Find(id string) (Offer, bool) {
	for _, offer := range Catalog() {
		if offer.ID == id {
			return offer, true
		}
	}
	return Offer{}, false
}

// Level ranks how the latest measurement compares with the advertised plan.
type Level string

const (
	LevelOnPar     Level = "on_par"          // within the expected range of the plan
	LevelBelow     Level = "below_plan"      // clearly slower than advertised
	LevelWellBelow Level = "well_below_plan" // far below the plan, grounds to complain
)

// Conservative thresholds for "up to X Mbps" consumer plans.
const (
	onParPercent     = 70
	wellBelowPercent = 40
)

type Verdict struct {
	Level                  Level  `json:"level"`
	DownloadPct            int    `json:"download_pct"`
	UploadPct              int    `json:"upload_pct,omitempty"` // 0 when the plan advertises no upload speed
	AdvertisedDownloadMbps int    `json:"advertised_download_mbps"`
	Summary                string `json:"summary"`
	Advice                 string `json:"advice"`
	ComplaintWorthy        bool   `json:"complaint_worthy"`
}

// Assess compares measured application-level bits per second with the
// advertised plan. Wording stays factual: an NDT7 measurement to a neutral
// nearby server is evidence of delivered performance, not legal proof of
// line capacity.
func Assess(offer Offer, downloadBPS, uploadBPS int64) Verdict {
	return AssessAt(offer, downloadBPS, uploadBPS, time.Now())
}

// AssessAt applies time-dependent advertised speeds using Philippine Standard
// Time (UTC+8), which has no daylight-saving transitions.
func AssessAt(offer Offer, downloadBPS, uploadBPS int64, observedAt time.Time) Verdict {
	advertised := EffectiveDownloadMbps(offer, observedAt)
	downloadPct := percent(downloadBPS, advertised)
	verdict := Verdict{DownloadPct: downloadPct, AdvertisedDownloadMbps: advertised}
	if offer.UploadMbps > 0 {
		verdict.UploadPct = percent(uploadBPS, offer.UploadMbps)
	}
	switch {
	case downloadPct >= onParPercent:
		verdict.Level = LevelOnPar
		verdict.Summary = "In line with your plan"
		verdict.Advice = "The measured download speed is within the normal range of the advertised speed."
	case downloadPct >= wellBelowPercent:
		verdict.Level = LevelBelow
		verdict.Summary = "Slower than your plan"
		verdict.Advice = "Measured download is clearly below the advertised speed. Keep monitoring and retain a PDF report as evidence."
	default:
		verdict.Level = LevelWellBelow
		verdict.Summary = "Well below your plan"
		verdict.Advice = "Measured download is far below the advertised speed. You have reasonable grounds to contact your ISP; attach the FiberPulse PDF report."
		verdict.ComplaintWorthy = true
	}
	return verdict
}

func EffectiveDownloadMbps(offer Offer, observedAt time.Time) int {
	if offer.DayMbps <= 0 || offer.NightMbps <= 0 {
		return offer.DownloadMbps
	}
	hour := observedAt.UTC().Add(8 * time.Hour).Hour()
	if hour >= 7 && hour < 19 {
		return offer.DayMbps
	}
	return offer.NightMbps
}

func percent(measuredBPS int64, advertisedMbps int) int {
	if advertisedMbps <= 0 || measuredBPS <= 0 {
		return 0
	}
	return int(measuredBPS * 100 / (int64(advertisedMbps) * 1_000_000))
}

// Describe renders a compact human-readable comparison, for example
// "448 of 400 Mbps advertised (112%)".
func Describe(verdict Verdict, offer Offer) string {
	advertised := verdict.AdvertisedDownloadMbps
	if advertised <= 0 {
		advertised = offer.DownloadMbps
	}
	return fmt.Sprintf("%d%% of the advertised %d Mbps", verdict.DownloadPct, advertised)
}
