// Package plan holds the built-in catalog of Philippine ISP offers and the
// conservative comparison between a subscribed offer and a measured NDT7
// result. Catalog entries come from the providers' public pages (checked
// 2026-08-30); upload is left at zero when the provider does not advertise it.
package plan

import (
	"errors"
	"fmt"
)

var ErrUnknownOffer = errors.New("unknown plan offer")

type Offer struct {
	ID           string `json:"id"`
	ISP          string `json:"isp"`
	Name         string `json:"name"`
	DownloadMbps int    `json:"download_mbps"`
	UploadMbps   int    `json:"upload_mbps"` // 0 when not advertised
	Note         string `json:"note,omitempty"`
}

func Catalog() []Offer {
	return []Offer{
		{ID: "converge-fiberx-200", ISP: "Converge ICT", Name: "FiberX 200", DownloadMbps: 200},
		{ID: "converge-fiberx-400", ISP: "Converge ICT", Name: "FiberX 400", DownloadMbps: 400},
		{ID: "converge-fiberx-600", ISP: "Converge ICT", Name: "FiberX 600", DownloadMbps: 600},
		{ID: "converge-time-machine-50", ISP: "Converge ICT", Name: "FiberX Time Machine 50", DownloadMbps: 50, Note: "Daily time-based pass"},
		{ID: "pldt-fibr-200", ISP: "PLDT Home", Name: "Fibr 1299 (up to 200 Mbps)", DownloadMbps: 200},
		{ID: "pldt-fibr-400", ISP: "PLDT Home", Name: "Fibr 1699 (up to 400 Mbps)", DownloadMbps: 400},
		{ID: "pldt-fibr-600", ISP: "PLDT Home", Name: "Fibr 2099 (up to 600 Mbps)", DownloadMbps: 600},
		{ID: "globe-gfiber-300", ISP: "Globe", Name: "GFiber Postpaid 300", DownloadMbps: 300},
		{ID: "globe-gfiber-500", ISP: "Globe", Name: "GFiber Postpaid 500", DownloadMbps: 500},
		{ID: "globe-gfiber-800", ISP: "Globe", Name: "GFiber Postpaid 800", DownloadMbps: 800},
		{ID: "dito-home-100", ISP: "DITO", Name: "Home 5G 590 (up to 100 Mbps)", DownloadMbps: 100, Note: "Fixed wireless"},
		{ID: "dito-home-350", ISP: "DITO", Name: "Home 5G 990 (up to 350 Mbps)", DownloadMbps: 350, Note: "Fixed wireless"},
		{ID: "dito-home-500", ISP: "DITO", Name: "Home 5G 1290 (up to 500 Mbps)", DownloadMbps: 500, Note: "Fixed wireless"},
	}
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
	Level           Level  `json:"level"`
	DownloadPct     int    `json:"download_pct"`
	UploadPct       int    `json:"upload_pct,omitempty"` // 0 when the plan advertises no upload speed
	Summary         string `json:"summary"`
	Advice          string `json:"advice"`
	ComplaintWorthy bool   `json:"complaint_worthy"`
}

// Assess compares measured application-level bits per second with the
// advertised plan. Wording stays factual: an NDT7 measurement to a neutral
// nearby server is evidence of delivered performance, not legal proof of
// line capacity.
func Assess(offer Offer, downloadBPS, uploadBPS int64) Verdict {
	downloadPct := percent(downloadBPS, offer.DownloadMbps)
	verdict := Verdict{DownloadPct: downloadPct}
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

func percent(measuredBPS int64, advertisedMbps int) int {
	if advertisedMbps <= 0 || measuredBPS <= 0 {
		return 0
	}
	return int(measuredBPS * 100 / (int64(advertisedMbps) * 1_000_000))
}

// Describe renders a compact human-readable comparison, for example
// "448 of 400 Mbps advertised (112%)".
func Describe(verdict Verdict, offer Offer) string {
	return fmt.Sprintf("%d%% of the advertised %d Mbps", verdict.DownloadPct, offer.DownloadMbps)
}
