package reporting

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
	"time"

	"fiberpulse.dev/agent/internal/measurement"
	"github.com/signintech/gopdf"
	"golang.org/x/image/font/gofont/goregular"
)

func CSV(results []measurement.Result) ([]byte, error) {
	var out bytes.Buffer
	w := csv.NewWriter(&out)
	header := []string{"started_at_utc", "provider", "server", "download_bps", "upload_bps", "min_rtt_us", "status", "confidence_score", "confidence_level", "public_eligible", "reason_codes"}
	if err := w.Write(header); err != nil {
		return nil, err
	}
	for _, r := range results {
		row := []string{r.StartedAt.UTC().Format(time.RFC3339), r.Provider, r.ServerFQDN, strconv.FormatInt(r.DownloadBPS, 10), strconv.FormatInt(r.UploadBPS, 10), strconv.FormatInt(r.MinRTTUS, 10), string(r.Status), strconv.Itoa(r.ConfidenceScore), r.ConfidenceLevel, strconv.FormatBool(r.PublicEligible), fmt.Sprint(r.ConfidenceReasons)}
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}
	w.Flush()
	return out.Bytes(), w.Error()
}

func PDF(results []measurement.Result, periodStart, periodEnd time.Time) ([]byte, error) {
	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
	if err := pdf.AddTTFFontData("GoRegular", goregular.TTF); err != nil {
		return nil, err
	}
	pdf.AddPage()
	if err := pdf.SetFont("GoRegular", "", 16); err != nil {
		return nil, err
	}
	pdf.SetX(42)
	pdf.SetY(42)
	if err := pdf.Text("FiberPulse measured Internet performance report"); err != nil {
		return nil, err
	}
	if err := pdf.SetFont("GoRegular", "", 10); err != nil {
		return nil, err
	}
	pdf.SetY(72)
	lines := []string{
		fmt.Sprintf("Period: %s to %s UTC", periodStart.UTC().Format("2006-01-02"), periodEnd.UTC().Format("2006-01-02")),
		fmt.Sprintf("Measurements included: %d", len(results)),
		"These are observed measurements to test servers, not physical line capacity or proof of ISP responsibility.",
		"RTT is the minimum RTT observed by NDT7 toward its selected server.",
	}
	for _, line := range lines {
		pdf.SetX(42)
		wrapped, err := pdf.SplitText(line, 510)
		if err != nil {
			return nil, err
		}
		for _, part := range wrapped {
			if err := pdf.Text(part); err != nil {
				return nil, err
			}
			pdf.Br(14)
		}
	}
	pdf.Br(8)
	pdf.SetX(42)
	if err := pdf.Text("Recent measurements"); err != nil {
		return nil, err
	}
	pdf.Br(16)
	for i, r := range results {
		if i >= 25 {
			pdf.SetX(42)
			_ = pdf.Text("Additional rows are available in the CSV export.")
			break
		}
		line := fmt.Sprintf("%s  down %.1f Mbps  up %.1f Mbps  min RTT %.1f ms  confidence %s (%d)", r.StartedAt.UTC().Format("2006-01-02 15:04"), float64(r.DownloadBPS)/1e6, float64(r.UploadBPS)/1e6, float64(r.MinRTTUS)/1e3, r.ConfidenceLevel, r.ConfidenceScore)
		pdf.SetX(42)
		wrapped, err := pdf.SplitText(line, 510)
		if err != nil {
			return nil, err
		}
		for _, part := range wrapped {
			if err := pdf.Text(part); err != nil {
				return nil, err
			}
			pdf.Br(13)
		}
		if pdf.GetY() > 780 {
			pdf.AddPage()
			_ = pdf.SetFont("GoRegular", "", 10)
			pdf.SetY(42)
		}
	}
	var out bytes.Buffer
	if _, err := pdf.WriteTo(&out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
