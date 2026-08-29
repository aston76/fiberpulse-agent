package sponsor

import "testing"

func TestValidateSponsor(t *testing.T) {
	offer, err := Validate(Offer{CampaignID: "ethernet-2026", Headline: "Reliable home networking", Body: "A clearly disclosed partner placement.", CTA: "View offer", URL: "https://example.com/product?affiliate=fiberpulse"})
	if err != nil {
		t.Fatal(err)
	}
	if !offer.Enabled() || offer.Label != "Sponsored" {
		t.Fatalf("unexpected offer: %+v", offer)
	}
	if _, err := Validate(Offer{CampaignID: "bad", Headline: "Bad", Body: "Bad", CTA: "Open", URL: "http://example.com"}); err == nil {
		t.Fatal("insecure sponsor URL accepted")
	}
	if empty, err := Validate(Offer{}); err != nil || empty.Enabled() {
		t.Fatalf("disabled sponsor rejected: %+v %v", empty, err)
	}
}
