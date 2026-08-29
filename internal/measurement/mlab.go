package measurement

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	ndt7 "github.com/m-lab/ndt7-client-go"
	"github.com/m-lab/ndt7-client-go/spec"
)

type MLabProvider struct {
	ClientName    string
	ClientVersion string
	Enabled       bool
	Timeout       time.Duration
}

func (p *MLabProvider) Capabilities() []string {
	return []string{"download", "upload", "min_rtt", "locate_v2"}
}

func (p *MLabProvider) Metadata() Metadata {
	return Metadata{Name: ProviderMLabNDT7, ProtocolVersion: "ndt7", ClientVersion: p.ClientVersion, Enabled: p.Enabled}
}

func (p *MLabProvider) Preflight(_ context.Context, network NetworkContext, consent bool) (PreflightResult, error) {
	result := PreflightResult{Network: network}
	if !p.Enabled {
		result.Reasons = append(result.Reasons, "provider.disabled")
		return result, ErrProviderDisabled
	}
	if !consent {
		result.Reasons = append(result.Reasons, "consent.required")
		return result, ErrConsentRequired
	}
	if !network.Online {
		result.Reasons = append(result.Reasons, "network.offline")
	}
	if network.Metered {
		result.Reasons = append(result.Reasons, "network.metered")
	}
	if network.Roaming {
		result.Reasons = append(result.Reasons, "network.roaming")
	}
	if network.VPNDetected {
		result.Reasons = append(result.Reasons, "network.vpn")
		return result, ErrVPNDetected
	}
	if len(result.Reasons) != 0 {
		return result, ErrNetworkIneligible
	}
	result.Eligible = true
	return result, nil
}

func (p *MLabProvider) Run(parent context.Context, network NetworkContext, progress func(Progress)) (Result, error) {
	if !p.Enabled {
		return Result{}, ErrProviderDisabled
	}
	timeout := p.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	started := time.Now().UTC()
	result := Result{
		ID: uuid.NewString(), Provider: ProviderMLabNDT7, ProtocolVersion: "ndt7", ClientVersion: p.ClientVersion,
		SchemaVersion: SchemaVersion, MethodologyVersion: MethodologyVersion, ConfidenceVersion: ConfidenceVersion,
		StartedAt: started, Status: StatusFailed, NetworkBefore: network,
	}
	client := ndt7.NewClient(p.ClientName, p.ClientVersion)

	if progress != nil {
		progress(Progress{Phase: string(TestDownload)})
	}
	download, err := client.StartDownload(ctx)
	if err != nil {
		return finishFailure(result, "mlab.download_start", err)
	}
	consumeMeasurements(download, "download", &result.BytesDown, &result.DownloadDurationUS, &result.MinRTTUS, progress)
	result.DownloadBPS = bitsPerSecond(result.BytesDown, result.DownloadDurationUS)
	result.ServerFQDN = client.FQDN
	if err := ctx.Err(); err != nil {
		return finishFailure(result, "mlab.download", err)
	}

	if progress != nil {
		progress(Progress{Phase: string(TestUpload)})
	}
	upload, err := client.StartUpload(ctx)
	if err != nil {
		result.Status = StatusPartial
		result.ErrorCode = "mlab.upload_start"
		result.ErrorDetail = err.Error()
		result.CompletedAt = time.Now().UTC()
		return result, err
	}
	consumeMeasurements(upload, "upload", &result.BytesUp, &result.UploadDurationUS, &result.MinRTTUS, progress)
	result.UploadBPS = bitsPerSecond(result.BytesUp, result.UploadDurationUS)
	result.NetworkAfter = network
	result.CompletedAt = time.Now().UTC()
	if err := ctx.Err(); err != nil {
		if errors.Is(err, context.Canceled) {
			result.Status = StatusCancelled
		} else {
			result.Status = StatusPartial
		}
		result.ErrorCode = "mlab.context"
		result.ErrorDetail = err.Error()
		return result, err
	}
	if result.DownloadBPS <= 0 || result.UploadBPS <= 0 {
		result.Status = StatusPartial
		result.ErrorCode = "mlab.incomplete"
		result.ErrorDetail = "one or more NDT7 phases did not produce a positive application-level result"
		return result, fmt.Errorf("%s", result.ErrorDetail)
	}
	result.Status = StatusComplete
	return result, nil
}

func consumeMeasurements(ch <-chan spec.Measurement, phase string, bytes, elapsed, minRTT *int64, progress func(Progress)) {
	for event := range ch {
		if event.AppInfo != nil {
			if event.AppInfo.NumBytes > *bytes {
				*bytes = event.AppInfo.NumBytes
			}
			if event.AppInfo.ElapsedTime > *elapsed {
				*elapsed = event.AppInfo.ElapsedTime
			}
		}
		if event.TCPInfo != nil && event.TCPInfo.MinRTT > 0 && (*minRTT == 0 || int64(event.TCPInfo.MinRTT) < *minRTT) {
			*minRTT = int64(event.TCPInfo.MinRTT)
		}
		if progress != nil {
			progress(Progress{Phase: phase, Bytes: *bytes, ElapsedUS: *elapsed, EstimatedBPS: bitsPerSecond(*bytes, *elapsed)})
		}
	}
}

func bitsPerSecond(bytes, elapsedUS int64) int64 {
	if bytes <= 0 || elapsedUS <= 0 || bytes > (1<<62)/8_000_000 {
		return 0
	}
	return bytes * 8_000_000 / elapsedUS
}

func finishFailure(result Result, code string, err error) (Result, error) {
	result.CompletedAt = time.Now().UTC()
	result.ErrorCode = code
	result.ErrorDetail = err.Error()
	if errors.Is(err, context.Canceled) {
		result.Status = StatusCancelled
	}
	return result, err
}
