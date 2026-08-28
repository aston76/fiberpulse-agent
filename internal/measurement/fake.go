package measurement

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type FakeProvider struct {
	Delay time.Duration
	Now   func() time.Time
}

func (p *FakeProvider) Capabilities() []string { return []string{"download", "upload", "min_rtt"} }

func (p *FakeProvider) Metadata() Metadata {
	return Metadata{Name: ProviderDevelopmentFake, ProtocolVersion: "fake-v1", ClientVersion: "dev", Enabled: true}
}

func (p *FakeProvider) Preflight(_ context.Context, network NetworkContext, consent bool) (PreflightResult, error) {
	if !consent {
		return PreflightResult{Network: network, Reasons: []string{"consent.required"}}, ErrConsentRequired
	}
	if !network.Online || network.Metered || network.Roaming {
		return PreflightResult{Network: network, Reasons: []string{"network.ineligible"}}, ErrNetworkIneligible
	}
	return PreflightResult{Eligible: true, Network: network}, nil
}

func (p *FakeProvider) Run(ctx context.Context, network NetworkContext, progress func(Progress)) (Result, error) {
	now := p.Now
	if now == nil {
		now = time.Now
	}
	delay := p.Delay
	if delay <= 0 {
		delay = 20 * time.Millisecond
	}
	started := now().UTC()
	select {
	case <-ctx.Done():
		return Result{ID: uuid.NewString(), Provider: ProviderDevelopmentFake, StartedAt: started, CompletedAt: now().UTC(), Status: StatusCancelled}, ctx.Err()
	case <-time.After(delay):
	}
	if progress != nil {
		progress(Progress{Phase: "download", Bytes: 125_000_000, ElapsedUS: 10_000_000, EstimatedBPS: 100_000_000})
		progress(Progress{Phase: "upload", Bytes: 25_000_000, ElapsedUS: 10_000_000, EstimatedBPS: 20_000_000})
	}
	return Result{
		ID: uuid.NewString(), Provider: ProviderDevelopmentFake, ProtocolVersion: "fake-v1", ClientVersion: "dev",
		SchemaVersion: SchemaVersion, MethodologyVersion: MethodologyVersion, ConfidenceVersion: ConfidenceVersion,
		StartedAt: started, CompletedAt: now().UTC(), ServerFQDN: "fake.invalid",
		DownloadBPS: 100_000_000, UploadBPS: 20_000_000, MinRTTUS: 12_000,
		BytesDown: 125_000_000, BytesUp: 25_000_000, DownloadDurationUS: 10_000_000, UploadDurationUS: 10_000_000,
		Status: StatusComplete, NetworkBefore: network, NetworkAfter: network,
	}, nil
}
