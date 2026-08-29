// Package plan holds the country-aware catalog of ISP offers and the
// conservative comparison between a subscribed offer and a measured NDT7
// result. The initial catalog covers the Philippines; every offer carries its
// own country and verification metadata so other markets can be added without
// changing the API or persisted selections.
package plan

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrUnknownOffer = errors.New("unknown plan offer")

type Offer struct {
	ID           string `json:"id"`
	CountryCode  string `json:"country_code"`
	CountryName  string `json:"country_name"`
	ISP          string `json:"isp"`
	Name         string `json:"name"`
	DownloadMbps int    `json:"download_mbps"`
	UploadMbps   int    `json:"upload_mbps"` // 0 when not advertised
	PriceAmount  int    `json:"price_amount,omitempty"`
	CurrencyCode string `json:"currency_code,omitempty"`
	PricePHP     int    `json:"price_php,omitempty"` // Legacy API field; use PriceAmount for new countries.
	PricePeriod  string `json:"price_period,omitempty"`
	Category     string `json:"category,omitempty"`
	Note         string `json:"note,omitempty"`
	SourceURL    string `json:"source_url,omitempty"`
	VerifiedAt   string `json:"verified_at,omitempty"`
	Custom       bool   `json:"custom,omitempty"`
	DayMbps      int    `json:"day_mbps,omitempty"`
	NightMbps    int    `json:"night_mbps,omitempty"`
}

const CatalogVerifiedAt = "2026-08-30"

const (
	PhilippinesCode     = "PH"
	PhilippinesName     = "Philippines"
	PhilippinesCurrency = "PHP"
)

func Catalog() []Offer {
	return []Offer{
		// Current Converge residential offers.
		verified(Offer{ID: "converge-super-play-1349", ISP: "Converge ICT", Name: "Super FiberX Play 1349", DownloadMbps: 200, PricePHP: 1349, PricePeriod: "month", Category: "Super FiberX", SourceURL: "https://www.convergeict.com/super-fiberx"}),
		verified(Offer{ID: "converge-super-max-1599", ISP: "Converge ICT", Name: "Super FiberX Max 1599", DownloadMbps: 400, PricePHP: 1599, PricePeriod: "month", Category: "Super FiberX", SourceURL: "https://www.convergeict.com/super-fiberx"}),
		verified(Offer{ID: "converge-super-prime-2099", ISP: "Converge ICT", Name: "Super FiberX Prime 2099", DownloadMbps: 800, PricePHP: 2099, PricePeriod: "month", Category: "Super FiberX", SourceURL: "https://www.convergeict.com/super-fiberx"}),
		verified(Offer{ID: "converge-super-ultra-2599", ISP: "Converge ICT", Name: "Super FiberX Ultra 2599", DownloadMbps: 1000, PricePHP: 2599, PricePeriod: "month", Category: "Super FiberX", SourceURL: "https://www.convergeict.com/super-fiberx"}),
		verified(Offer{ID: "converge-netflix-1558", ISP: "Converge ICT", Name: "FiberX Netflix 1558", DownloadMbps: 200, PricePHP: 1558, PricePeriod: "month", Category: "Netflix bundle", SourceURL: "https://www.convergeict.com/x-plus-n"}),
		verified(Offer{ID: "converge-netflix-1798", ISP: "Converge ICT", Name: "FiberX Netflix 1798", DownloadMbps: 400, PricePHP: 1798, PricePeriod: "month", Category: "Netflix bundle", SourceURL: "https://www.convergeict.com/x-plus-n"}),
		verified(Offer{ID: "converge-netflix-1998", ISP: "Converge ICT", Name: "FiberX Netflix 1998", DownloadMbps: 500, PricePHP: 1998, PricePeriod: "month", Category: "Netflix bundle", SourceURL: "https://www.convergeict.com/x-plus-n"}),
		verified(Offer{ID: "converge-netflix-2298", ISP: "Converge ICT", Name: "FiberX Netflix 2298", DownloadMbps: 600, PricePHP: 2298, PricePeriod: "month", Category: "Netflix bundle", SourceURL: "https://www.convergeict.com/x-plus-n"}),
		verified(Offer{ID: "converge-gamechanger-ez-1800", ISP: "Converge ICT", Name: "GameChanger EZ 1800", DownloadMbps: 400, PricePHP: 1800, PricePeriod: "month", Category: "Gaming", SourceURL: "https://www.convergeict.com/thegamechanger"}),
		verified(Offer{ID: "converge-gamechanger-elite-5000", ISP: "Converge ICT", Name: "GameChanger ELITE 5000", DownloadMbps: 1000, PricePHP: 5000, PricePeriod: "month", Category: "Gaming", SourceURL: "https://www.convergeict.com/thegamechanger"}),
		verified(Offer{ID: "converge-time-of-day-day-1699", ISP: "Converge ICT", Name: "FiberX Time of Day — Day Plan 1699", DownloadMbps: 600, DayMbps: 600, NightMbps: 400, PricePHP: 1699, PricePeriod: "month", Category: "Time of Day", Note: "600 Mbps from 7:00 AM to 6:59 PM; 400 Mbps from 7:00 PM to 6:59 AM. FiberPulse applies the correct Philippine time period.", SourceURL: "https://www.convergeict.com/fiberx-time-of-day"}),
		verified(Offer{ID: "converge-time-of-day-night-1699", ISP: "Converge ICT", Name: "FiberX Time of Day — Night Plan 1699", DownloadMbps: 600, DayMbps: 400, NightMbps: 600, PricePHP: 1699, PricePeriod: "month", Category: "Time of Day", Note: "400 Mbps from 7:00 AM to 6:59 PM; 600 Mbps from 7:00 PM to 6:59 AM. FiberPulse applies the correct Philippine time period.", SourceURL: "https://www.convergeict.com/fiberx-time-of-day"}),

		// Legacy FiberX plans remain selectable for existing subscribers.
		verified(Offer{ID: "converge-fiberx-200", ISP: "Converge ICT", Name: "FiberX 1500 (legacy)", DownloadMbps: 200, PricePHP: 1500, PricePeriod: "month", Category: "Legacy", Note: "For existing subscribers whose bill still names this offer.", SourceURL: "https://www.convergeict.com/dl-forms"}),
		verified(Offer{ID: "converge-fiberx-400", ISP: "Converge ICT", Name: "FiberX 2000 (legacy)", DownloadMbps: 400, PricePHP: 2000, PricePeriod: "month", Category: "Legacy", Note: "For existing subscribers whose bill still names this offer.", SourceURL: "https://www.convergeict.com/dl-forms"}),
		verified(Offer{ID: "converge-fiberx-600", ISP: "Converge ICT", Name: "FiberX 2500 (legacy)", DownloadMbps: 600, PricePHP: 2500, PricePeriod: "month", Category: "Legacy", Note: "For existing subscribers whose bill still names this offer.", SourceURL: "https://www.convergeict.com/dl-forms"}),
		verified(Offer{ID: "converge-fiberx-800", ISP: "Converge ICT", Name: "FiberX 3500 (legacy)", DownloadMbps: 800, PricePHP: 3500, PricePeriod: "month", Category: "Legacy", Note: "For existing subscribers whose bill still names this offer.", SourceURL: "https://www.convergeict.com/dl-forms"}),
		verified(Offer{ID: "converge-fiberx-1000", ISP: "Converge ICT", Name: "FiberX 7499 (legacy)", DownloadMbps: 1000, PricePHP: 7499, PricePeriod: "month", Category: "Legacy", Note: "For existing subscribers whose bill still names this offer.", SourceURL: "https://www.convergeict.com/dl-forms"}),

		// Current PLDT Home residential offers.
		verified(Offer{ID: "pldt-unli-1299", ISP: "PLDT Home", Name: "Fiber Unli 1299", DownloadMbps: 100, PricePHP: 1299, PricePeriod: "month", Category: "Fiber Unli", SourceURL: "https://www.pldthome.com/internet"}),
		verified(Offer{ID: "pldt-unli-1699", ISP: "PLDT Home", Name: "Fiber Unli 1699", DownloadMbps: 500, PricePHP: 1699, PricePeriod: "month", Category: "Fiber Unli", SourceURL: "https://www.pldthome.com/internet"}),
		verified(Offer{ID: "pldt-unli-2099", ISP: "PLDT Home", Name: "Fiber Unli 2099", DownloadMbps: 700, PricePHP: 2099, PricePeriod: "month", Category: "Fiber Unli", SourceURL: "https://www.pldthome.com/internet"}),
		verified(Offer{ID: "pldt-unli-2699", ISP: "PLDT Home", Name: "Fiber Unli 2699", DownloadMbps: 1000, PricePHP: 2699, PricePeriod: "month", Category: "Fiber Unli", SourceURL: "https://www.pldthome.com/internet"}),
		verified(Offer{ID: "pldt-unli-9499", ISP: "PLDT Home", Name: "Fiber Unli 9499", DownloadMbps: 1000, PricePHP: 9499, PricePeriod: "month", Category: "Fiber Unli", SourceURL: "https://www.pldthome.com/internet"}),
		verified(Offer{ID: "pldt-unli-all-1399", ISP: "PLDT Home", Name: "Fiber Unli All 1399", DownloadMbps: 200, PricePHP: 1399, PricePeriod: "month", Category: "Fiber + TV", SourceURL: "https://www.pldthome.com/fiber-unli-all"}),
		verified(Offer{ID: "pldt-unli-all-1799", ISP: "PLDT Home", Name: "Fiber Unli All 1799", DownloadMbps: 500, PricePHP: 1799, PricePeriod: "month", Category: "Fiber + TV", SourceURL: "https://www.pldthome.com/fiber-unli-all"}),
		verified(Offer{ID: "pldt-unli-all-2499", ISP: "PLDT Home", Name: "Fiber Unli All 2499", DownloadMbps: 700, PricePHP: 2499, PricePeriod: "month", Category: "Fiber + TV", SourceURL: "https://www.pldthome.com/fiber-unli-all"}),
		verified(Offer{ID: "pldt-netflix-1599", ISP: "PLDT Home", Name: "Fiber Netflix 1599", DownloadMbps: 300, PricePHP: 1599, PricePeriod: "month", Category: "Netflix bundle", SourceURL: "https://www.pldthome.com/fiber-netflix"}),
		verified(Offer{ID: "pldt-prepaid-50-1d", ISP: "PLDT Home", Name: "Fiber Prepaid 50 — 1 day", DownloadMbps: 50, PricePHP: 50, PricePeriod: "1 day", Category: "Prepaid", SourceURL: "https://www.pldthome.com/news-media/2026/04/27/pldt-home-fiber-prepaid-now-offers-speeds-of-up-to-300-mbps"}),
		verified(Offer{ID: "pldt-prepaid-50-7d", ISP: "PLDT Home", Name: "Fiber Prepaid 50 — 7 days", DownloadMbps: 50, PricePHP: 199, PricePeriod: "7 days", Category: "Prepaid", SourceURL: "https://www.pldthome.com/news-media/2026/04/27/pldt-home-fiber-prepaid-now-offers-speeds-of-up-to-300-mbps"}),
		verified(Offer{ID: "pldt-prepaid-50-15d", ISP: "PLDT Home", Name: "Fiber Prepaid 50 — 15 days", DownloadMbps: 50, PricePHP: 379, PricePeriod: "15 days", Category: "Prepaid", SourceURL: "https://www.pldthome.com/news-media/2026/04/27/pldt-home-fiber-prepaid-now-offers-speeds-of-up-to-300-mbps"}),
		verified(Offer{ID: "pldt-prepaid-50-30d", ISP: "PLDT Home", Name: "Fiber Prepaid 50 — 30 days", DownloadMbps: 50, PricePHP: 699, PricePeriod: "30 days", Category: "Prepaid", SourceURL: "https://www.pldthome.com/news-media/2026/04/27/pldt-home-fiber-prepaid-now-offers-speeds-of-up-to-300-mbps"}),
		verified(Offer{ID: "pldt-prepaid-50-365d", ISP: "PLDT Home", Name: "Fiber Prepaid 50 — 365 days", DownloadMbps: 50, PricePHP: 6999, PricePeriod: "365 days", Category: "Prepaid", SourceURL: "https://www.pldthome.com/news-media/2026/04/27/pldt-home-fiber-prepaid-now-offers-speeds-of-up-to-300-mbps"}),
		verified(Offer{ID: "pldt-prepaid-100-1d", ISP: "PLDT Home", Name: "Fiber Prepaid 100 — 1 day", DownloadMbps: 100, PricePHP: 99, PricePeriod: "1 day", Category: "Prepaid", SourceURL: "https://www.pldthome.com/news-media/2026/04/27/pldt-home-fiber-prepaid-now-offers-speeds-of-up-to-300-mbps"}),
		verified(Offer{ID: "pldt-prepaid-100-7d", ISP: "PLDT Home", Name: "Fiber Prepaid 100 — 7 days", DownloadMbps: 100, PricePHP: 379, PricePeriod: "7 days", Category: "Prepaid", SourceURL: "https://www.pldthome.com/news-media/2026/04/27/pldt-home-fiber-prepaid-now-offers-speeds-of-up-to-300-mbps"}),
		verified(Offer{ID: "pldt-prepaid-100-15d", ISP: "PLDT Home", Name: "Fiber Prepaid 100 — 15 days", DownloadMbps: 100, PricePHP: 549, PricePeriod: "15 days", Category: "Prepaid", SourceURL: "https://www.pldthome.com/news-media/2026/04/27/pldt-home-fiber-prepaid-now-offers-speeds-of-up-to-300-mbps"}),
		verified(Offer{ID: "pldt-prepaid-100-30d", ISP: "PLDT Home", Name: "Fiber Prepaid 100 — 30 days", DownloadMbps: 100, PricePHP: 999, PricePeriod: "30 days", Category: "Prepaid", SourceURL: "https://www.pldthome.com/news-media/2026/04/27/pldt-home-fiber-prepaid-now-offers-speeds-of-up-to-300-mbps"}),
		verified(Offer{ID: "pldt-prepaid-100-365d", ISP: "PLDT Home", Name: "Fiber Prepaid 100 — 365 days", DownloadMbps: 100, PricePHP: 9999, PricePeriod: "365 days", Category: "Prepaid", SourceURL: "https://www.pldthome.com/news-media/2026/04/27/pldt-home-fiber-prepaid-now-offers-speeds-of-up-to-300-mbps"}),
		verified(Offer{ID: "pldt-prepaid-300-1d", ISP: "PLDT Home", Name: "Fiber Prepaid 300 — 1 day", DownloadMbps: 300, PricePHP: 199, PricePeriod: "1 day", Category: "Prepaid", SourceURL: "https://www.pldthome.com/news-media/2026/04/27/pldt-home-fiber-prepaid-now-offers-speeds-of-up-to-300-mbps"}),
		verified(Offer{ID: "pldt-prepaid-300-7d", ISP: "PLDT Home", Name: "Fiber Prepaid 300 — 7 days", DownloadMbps: 300, PricePHP: 699, PricePeriod: "7 days", Category: "Prepaid", SourceURL: "https://www.pldthome.com/news-media/2026/04/27/pldt-home-fiber-prepaid-now-offers-speeds-of-up-to-300-mbps"}),
		verified(Offer{ID: "pldt-prepaid-300-15d", ISP: "PLDT Home", Name: "Fiber Prepaid 300 — 15 days", DownloadMbps: 300, PricePHP: 999, PricePeriod: "15 days", Category: "Prepaid", SourceURL: "https://www.pldthome.com/news-media/2026/04/27/pldt-home-fiber-prepaid-now-offers-speeds-of-up-to-300-mbps"}),
		verified(Offer{ID: "pldt-prepaid-300-30d", ISP: "PLDT Home", Name: "Fiber Prepaid 300 — 30 days", DownloadMbps: 300, PricePHP: 1499, PricePeriod: "30 days", Category: "Prepaid", SourceURL: "https://www.pldthome.com/news-media/2026/04/27/pldt-home-fiber-prepaid-now-offers-speeds-of-up-to-300-mbps"}),
		verified(Offer{ID: "pldt-prepaid-300-365d", ISP: "PLDT Home", Name: "Fiber Prepaid 300 — 365 days", DownloadMbps: 300, PricePHP: 14999, PricePeriod: "365 days", Category: "Prepaid", SourceURL: "https://www.pldthome.com/news-media/2026/04/27/pldt-home-fiber-prepaid-now-offers-speeds-of-up-to-300-mbps"}),

		// Current Globe residential postpaid and prepaid offers.
		verified(Offer{ID: "globe-gfiber-300", ISP: "Globe AT HOME", Name: "GFiber Plan 1499", DownloadMbps: 300, PricePHP: 1499, PricePeriod: "month", Category: "Postpaid", SourceURL: "https://www.globe.com.ph/shop/gfiber-terms"}),
		verified(Offer{ID: "globe-gfiber-500", ISP: "Globe AT HOME", Name: "GFiber Plan 1999", DownloadMbps: 500, PricePHP: 1999, PricePeriod: "month", Category: "Postpaid", SourceURL: "https://www.globe.com.ph/shop/gfiber-terms"}),
		verified(Offer{ID: "globe-gfiber-1000", ISP: "Globe AT HOME", Name: "GFiber Plan 2499", DownloadMbps: 1000, PricePHP: 2499, PricePeriod: "month", Category: "Postpaid", SourceURL: "https://www.globe.com.ph/shop/gfiber-terms"}),
		verified(Offer{ID: "globe-gfiber-2500", ISP: "Globe AT HOME", Name: "GFiber Plan 4999 (upgrade)", DownloadMbps: 2500, PricePHP: 4999, PricePeriod: "month", Category: "Postpaid upgrade", Note: "Available for eligible upgrades; serviceability varies by area.", SourceURL: "https://www.globe.com.ph/broadband"}),
		verified(Offer{ID: "globe-prepaid-50-7d", ISP: "Globe AT HOME", Name: "GFiber Prepaid UNLISurf 249", DownloadMbps: 50, PricePHP: 249, PricePeriod: "7 days", Category: "Prepaid", SourceURL: "https://www.globe.com.ph/broadband/gfiber-prepaid"}),
		verified(Offer{ID: "globe-prepaid-50-30d", ISP: "Globe AT HOME", Name: "GFiber Prepaid UNLISurf 749", DownloadMbps: 50, PricePHP: 749, PricePeriod: "30 days", Category: "Prepaid", SourceURL: "https://www.globe.com.ph/broadband/gfiber-prepaid"}),
		verified(Offer{ID: "globe-prepaid-50-365d", ISP: "Globe AT HOME", Name: "GFiber Prepaid UNLISurf 6999", DownloadMbps: 50, PricePHP: 6999, PricePeriod: "365 days", Category: "Prepaid", SourceURL: "https://www.globe.com.ph/broadband/gfiber-prepaid"}),
		verified(Offer{ID: "globe-prepaid-100-7d", ISP: "Globe AT HOME", Name: "GFiber Prepaid UNLISurf 399", DownloadMbps: 100, PricePHP: 399, PricePeriod: "7 days", Category: "Prepaid", SourceURL: "https://www.globe.com.ph/broadband/gfiber-prepaid"}),
		verified(Offer{ID: "globe-prepaid-100-30d", ISP: "Globe AT HOME", Name: "GFiber Prepaid UNLISurf 999", DownloadMbps: 100, PricePHP: 999, PricePeriod: "30 days", Category: "Prepaid", SourceURL: "https://www.globe.com.ph/broadband/gfiber-prepaid"}),
		verified(Offer{ID: "globe-prepaid-100-365d", ISP: "Globe AT HOME", Name: "GFiber Prepaid UNLISurf 9999", DownloadMbps: 100, PricePHP: 9999, PricePeriod: "365 days", Category: "Prepaid", SourceURL: "https://www.globe.com.ph/broadband/gfiber-prepaid"}),
		verified(Offer{ID: "globe-prepaid-300-30d", ISP: "Globe AT HOME", Name: "GFiber Prepaid UNLISurf 1499", DownloadMbps: 300, PricePHP: 1499, PricePeriod: "30 days", Category: "Prepaid", SourceURL: "https://www.globe.com.ph/broadband/gfiber-prepaid"}),

		// Other consumer ISPs with a public, current and directly verifiable
		// Philippine plan page. Regional availability is stated in the UI note.
		verified(Offer{ID: "surf2sawa-superfiber-50", ISP: "Surf2Sawa", Name: "SuperFiber", DownloadMbps: 50, Category: "Prepaid fiber", Note: "Powered by Converge; supports up to 6 devices. Load price varies by validity.", SourceURL: "https://buyload.surf2sawa.com/"}),
		verified(Offer{ID: "surf2sawa-extraboost-100", ISP: "Surf2Sawa", Name: "ExtraBoost", DownloadMbps: 100, PricePHP: 1699, PricePeriod: "starter kit", Category: "Prepaid fiber", Note: "Powered by Converge; starter offer includes 30 days and supports up to 6 devices.", SourceURL: "https://applyna.surf2sawa.com/"}),

		verified(Offer{ID: "cablelink-fiberlink-888", ISP: "Cablelink", Name: "Fiberlink@Home 888", DownloadMbps: 50, PricePHP: 888, PricePeriod: "month", Category: "Fiber + TV", Note: "Regional serviceability applies.", SourceURL: "https://www.cablelink.com.ph/fiberlink_home"}),
		verified(Offer{ID: "cablelink-fiberlink-999", ISP: "Cablelink", Name: "Fiberlink@Home 999", DownloadMbps: 75, PricePHP: 999, PricePeriod: "month", Category: "Fiber + TV", Note: "Regional serviceability applies.", SourceURL: "https://www.cablelink.com.ph/fiberlink_home"}),
		verified(Offer{ID: "cablelink-fiberlink-1088", ISP: "Cablelink", Name: "Fiberlink@Home 1088", DownloadMbps: 88, PricePHP: 1088, PricePeriod: "month", Category: "Fiber + TV", Note: "Regional serviceability applies.", SourceURL: "https://www.cablelink.com.ph/fiberlink_home"}),
		verified(Offer{ID: "cablelink-fiberlink-1388", ISP: "Cablelink", Name: "Fiberlink@Home 1388", DownloadMbps: 188, PricePHP: 1388, PricePeriod: "month", Category: "Fiber + TV", Note: "Regional serviceability applies.", SourceURL: "https://www.cablelink.com.ph/fiberlink_home"}),
		verified(Offer{ID: "cablelink-fiberlink-1699", ISP: "Cablelink", Name: "Fiberlink@Home 1699", DownloadMbps: 300, PricePHP: 1699, PricePeriod: "month", Category: "Fiber + TV", Note: "Regional serviceability applies.", SourceURL: "https://www.cablelink.com.ph/fiberlink_home"}),

		verified(Offer{ID: "padeco-999", ISP: "PADECO", Name: "Plan 999", DownloadMbps: 200, PricePHP: 999, PricePeriod: "month", Category: "Regional fiber", Note: "Availability depends on PADECO's listed service areas.", SourceURL: "https://www.padeco.com.ph/residential.html"}),
		verified(Offer{ID: "padeco-1500", ISP: "PADECO", Name: "Plan 1500", DownloadMbps: 300, PricePHP: 1500, PricePeriod: "month", Category: "Regional fiber", Note: "Availability depends on PADECO's listed service areas.", SourceURL: "https://www.padeco.com.ph/residential.html"}),
		verified(Offer{ID: "padeco-2500", ISP: "PADECO", Name: "Plan 2500", DownloadMbps: 400, PricePHP: 2500, PricePeriod: "month", Category: "Regional fiber", Note: "Availability depends on PADECO's listed service areas.", SourceURL: "https://www.padeco.com.ph/residential.html"}),
		verified(Offer{ID: "padeco-3500", ISP: "PADECO", Name: "Plan 3500", DownloadMbps: 500, PricePHP: 3500, PricePeriod: "month", Category: "Regional fiber", Note: "Availability depends on PADECO's listed service areas.", SourceURL: "https://www.padeco.com.ph/residential.html"}),

		verified(Offer{ID: "ludeco-1500", ISP: "LUDECO", Name: "Plan 1500", DownloadMbps: 200, PricePHP: 1500, PricePeriod: "month", Category: "Regional fiber", Note: "Published for San Juan, City of San Fernando and Bauang, La Union.", SourceURL: "https://www.ludeco.com.ph/"}),
		verified(Offer{ID: "ludeco-2500", ISP: "LUDECO", Name: "Plan 2500", DownloadMbps: 300, PricePHP: 2500, PricePeriod: "month", Category: "Regional fiber", Note: "Published for San Juan, City of San Fernando and Bauang, La Union.", SourceURL: "https://www.ludeco.com.ph/"}),
		verified(Offer{ID: "ludeco-3500", ISP: "LUDECO", Name: "Plan 3500", DownloadMbps: 400, PricePHP: 3500, PricePeriod: "month", Category: "Regional fiber", Note: "Published for San Juan, City of San Fernando and Bauang, La Union.", SourceURL: "https://www.ludeco.com.ph/"}),

		verified(Offer{ID: "fiberbro-999", ISP: "Fiberbro Makati", Name: "Plan 50 Mbps", DownloadMbps: 50, PricePHP: 999, PricePeriod: "month", Category: "Regional fiber", Note: "Serviceability and installation distance limits apply.", SourceURL: "https://fiberbro-makati.com/"}),
		verified(Offer{ID: "fiberbro-1299", ISP: "Fiberbro Makati", Name: "Plan 100 Mbps", DownloadMbps: 100, PricePHP: 1299, PricePeriod: "month", Category: "Regional fiber", Note: "Serviceability and installation distance limits apply.", SourceURL: "https://fiberbro-makati.com/"}),
		verified(Offer{ID: "fiberbro-1499", ISP: "Fiberbro Makati", Name: "Plan 140 Mbps", DownloadMbps: 140, PricePHP: 1499, PricePeriod: "month", Category: "Regional fiber", Note: "Serviceability and installation distance limits apply.", SourceURL: "https://fiberbro-makati.com/"}),
		verified(Offer{ID: "fiberbro-1699", ISP: "Fiberbro Makati", Name: "Plan 200 Mbps", DownloadMbps: 200, PricePHP: 1699, PricePeriod: "month", Category: "Regional fiber", Note: "Serviceability and installation distance limits apply.", SourceURL: "https://fiberbro-makati.com/"}),

		verified(Offer{ID: "jquest-888", ISP: "JQuest Network", Name: "Plan 888", DownloadMbps: 35, PricePHP: 888, PricePeriod: "month", Category: "Regional fiber / wireless", Note: "Published for Pampanga; connection medium depends on serviceability.", SourceURL: "https://jquestnetwork.com/"}),
		verified(Offer{ID: "jquest-999", ISP: "JQuest Network", Name: "Plan 999", DownloadMbps: 50, PricePHP: 999, PricePeriod: "month", Category: "Regional fiber / wireless", Note: "Published for Pampanga; connection medium depends on serviceability.", SourceURL: "https://jquestnetwork.com/"}),
		verified(Offer{ID: "jquest-1399", ISP: "JQuest Network", Name: "Plan 1399", DownloadMbps: 100, PricePHP: 1399, PricePeriod: "month", Category: "Regional fiber / wireless", Note: "Published for Pampanga; connection medium depends on serviceability.", SourceURL: "https://jquestnetwork.com/"}),
		verified(Offer{ID: "jquest-1888", ISP: "JQuest Network", Name: "Plan 1888", DownloadMbps: 200, PricePHP: 1888, PricePeriod: "month", Category: "Regional fiber / wireless", Note: "Published for Pampanga; connection medium depends on serviceability.", SourceURL: "https://jquestnetwork.com/"}),

		verified(Offer{ID: "ikonnect-799", ISP: "iKonnect", Name: "Konnect799", DownloadMbps: 50, PricePHP: 799, PricePeriod: "month", Category: "Regional 5G", Note: "Fixed wireless; published for San Agustin and Calatrava, Romblon.", SourceURL: "https://ikonnect.ph/"}),
		verified(Offer{ID: "ikonnect-1000", ISP: "iKonnect", Name: "Konnect1000", DownloadMbps: 100, PricePHP: 1000, PricePeriod: "month", Category: "Regional 5G", Note: "Fixed wireless; published for San Agustin and Calatrava, Romblon.", SourceURL: "https://ikonnect.ph/"}),
		verified(Offer{ID: "ikonnect-1500", ISP: "iKonnect", Name: "Konnect1500", DownloadMbps: 200, PricePHP: 1500, PricePeriod: "month", Category: "Regional 5G", Note: "Fixed wireless; published for San Agustin and Calatrava, Romblon.", SourceURL: "https://ikonnect.ph/"}),
		verified(Offer{ID: "ikonnect-2000", ISP: "iKonnect", Name: "Konnect2000", DownloadMbps: 300, PricePHP: 2000, PricePeriod: "month", Category: "Business 5G", Note: "Fixed wireless business offer published for Romblon.", SourceURL: "https://ikonnect.ph/"}),

		verified(Offer{ID: "technet88-500", ISP: "Tech Net 88", Name: "Starter 5", DownloadMbps: 5, PricePHP: 500, PricePeriod: "month", Category: "Regional fiber", SourceURL: "https://www.technet88.com/"}),
		verified(Offer{ID: "technet88-999", ISP: "Tech Net 88", Name: "Basic 10", DownloadMbps: 10, PricePHP: 999, PricePeriod: "month", Category: "Regional fiber", SourceURL: "https://www.technet88.com/"}),
		verified(Offer{ID: "technet88-1299", ISP: "Tech Net 88", Name: "Most Popular 25", DownloadMbps: 25, PricePHP: 1299, PricePeriod: "month", Category: "Regional fiber", SourceURL: "https://www.technet88.com/"}),
		verified(Offer{ID: "technet88-1599", ISP: "Tech Net 88", Name: "Power 35", DownloadMbps: 35, PricePHP: 1599, PricePeriod: "month", Category: "Regional fiber", SourceURL: "https://www.technet88.com/"}),

		// DITO WoWFi is fixed wireless, not fiber. The distinction is visible in
		// the UI so its result is not presented as a fiber-line diagnosis.
		verified(Offer{ID: "dito-wowfi-lite", ISP: "DITO", Name: "WoWFi Lite", DownloadMbps: 50, PricePHP: 780, PricePeriod: "starter kit", Category: "Prepaid 5G", Note: "Fixed wireless; 70 GB starter allocation.", SourceURL: "https://dito.ph/wowfi"}),
		verified(Offer{ID: "dito-wowfi-pro", ISP: "DITO", Name: "WoWFi Pro", DownloadMbps: 100, PricePHP: 1490, PricePeriod: "starter kit", Category: "Prepaid 5G", Note: "Fixed wireless; unlimited data for the included 15-day period.", SourceURL: "https://dito.ph/wowfi"}),
		verified(Offer{ID: "dito-wowfi-ultra", ISP: "DITO", Name: "WoWFi Ultra", DownloadMbps: 500, PricePHP: 4990, PricePeriod: "starter kit", Category: "Prepaid 5G", Note: "Fixed wireless; unlimited 5G data for the included 30-day period.", SourceURL: "https://dito.ph/wowfi"}),
		verified(Offer{ID: "dito-wowfi-optima", ISP: "DITO", Name: "WoWFi Optima", DownloadMbps: 500, PricePHP: 1490, PricePeriod: "month", Category: "Postpaid 5G", Note: "Fixed wireless; average 55 Mbps and minimum 25 Mbps at 80% reliability are stated by DITO.", SourceURL: "https://dito.ph/wowfi"}),
	}
}

func verified(offer Offer) Offer {
	if offer.CountryCode == "" {
		offer.CountryCode = PhilippinesCode
	}
	if offer.CountryCode == PhilippinesCode && offer.CountryName == "" {
		offer.CountryName = PhilippinesName
	}
	if offer.PriceAmount == 0 && offer.PricePHP > 0 {
		offer.PriceAmount = offer.PricePHP
	}
	if offer.CountryCode == PhilippinesCode && offer.CurrencyCode == "" && offer.PriceAmount > 0 {
		offer.CurrencyCode = PhilippinesCurrency
	}
	offer.VerifiedAt = CatalogVerifiedAt
	return offer
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
