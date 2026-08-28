package baseline

import (
	"math"
	"slices"
	"time"
)

type Sample struct {
	At             time.Time
	DownloadBPS    int64
	UploadBPS      int64
	MinRTTUS       int64
	HighConfidence bool
}

type Result struct {
	Maturity       string `json:"maturity"`
	Count          int    `json:"count"`
	Days           int    `json:"days"`
	DownloadMedian int64  `json:"download_median_bps"`
	DownloadMAD    int64  `json:"download_mad_bps"`
	UploadMedian   int64  `json:"upload_median_bps"`
	MinRTTMedian   int64  `json:"min_rtt_median_us"`
}

func Calculate(samples []Sample) Result {
	var down, up, rtt []int64
	days := map[string]struct{}{}
	var first, last time.Time
	for _, s := range samples {
		if !s.HighConfidence {
			continue
		}
		down = append(down, s.DownloadBPS)
		up = append(up, s.UploadBPS)
		rtt = append(rtt, s.MinRTTUS)
		days[s.At.UTC().Format("2006-01-02")] = struct{}{}
		if first.IsZero() || s.At.Before(first) {
			first = s.At
		}
		if last.IsZero() || s.At.After(last) {
			last = s.At
		}
	}
	maturity := "insufficient"
	qualified := len(down) >= 10 && len(days) >= 3
	if qualified {
		maturity = "provisional"
	}
	if qualified && last.Sub(first) >= 14*24*time.Hour {
		maturity = "mature"
	}
	if qualified && last.Sub(first) >= 28*24*time.Hour {
		maturity = "time_profile_ready"
	}
	return Result{Maturity: maturity, Count: len(down), Days: len(days), DownloadMedian: median(down), DownloadMAD: mad(down), UploadMedian: median(up), MinRTTMedian: median(rtt)}
}

func median(values []int64) int64 {
	if len(values) == 0 {
		return 0
	}
	v := slices.Clone(values)
	slices.Sort(v)
	mid := len(v) / 2
	if len(v)%2 == 1 {
		return v[mid]
	}
	return v[mid-1]/2 + v[mid]/2 + (v[mid-1]%2+v[mid]%2)/2
}

func mad(values []int64) int64 {
	if len(values) == 0 {
		return 0
	}
	m := median(values)
	d := make([]int64, 0, len(values))
	for _, v := range values {
		d = append(d, int64(math.Abs(float64(v-m))))
	}
	return median(d)
}
