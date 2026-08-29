package sponsor

import (
	"errors"
	"net/url"
	"strings"
)

// Offer is a privacy-safe native sponsorship placement. FiberPulse renders the
// copy itself: no advertiser script, image pixel, cookie, or measurement data
// is sent to the sponsor.
type Offer struct {
	CampaignID string `json:"campaign_id"`
	Label      string `json:"label"`
	Headline   string `json:"headline"`
	Body       string `json:"body"`
	CTA        string `json:"cta"`
	URL        string `json:"url"`
}

func Validate(input Offer) (Offer, error) {
	offer := input
	fields := []struct {
		value *string
		max   int
	}{
		{&offer.CampaignID, 80}, {&offer.Label, 40}, {&offer.Headline, 100},
		{&offer.Body, 240}, {&offer.CTA, 40}, {&offer.URL, 2_000},
	}
	for _, field := range fields {
		*field.value = strings.TrimSpace(*field.value)
		if len(*field.value) > field.max {
			return Offer{}, errors.New("sponsor field exceeds its maximum length")
		}
	}
	if offer.URL == "" && offer.Headline == "" {
		return Offer{}, nil
	}
	if offer.CampaignID == "" || offer.Headline == "" || offer.Body == "" || offer.CTA == "" || offer.URL == "" {
		return Offer{}, errors.New("an enabled sponsor requires campaign id, headline, body, call to action, and URL")
	}
	parsed, err := url.ParseRequestURI(offer.URL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return Offer{}, errors.New("sponsor URL must be a public HTTPS URL without embedded credentials")
	}
	if offer.Label == "" {
		offer.Label = "Sponsored"
	}
	return offer, nil
}

func (o Offer) Enabled() bool { return o.URL != "" }
