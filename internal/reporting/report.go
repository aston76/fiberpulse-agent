package reporting

import (
	"bytes"
	"embed"
	"encoding/csv"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"fiberpulse.dev/agent/internal/complaint"
	"fiberpulse.dev/agent/internal/localization"
	"fiberpulse.dev/agent/internal/measurement"
	"fiberpulse.dev/agent/internal/plan"
	"github.com/signintech/gopdf"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
)

//go:embed fiberpulse-mark.png fonts/*.ttf
var reportAssets embed.FS

func CSV(results []measurement.Result) ([]byte, error) {
	return CSVLocalized(results, "en")
}

func CSVLocalized(results []measurement.Result, language string) ([]byte, error) {
	var out bytes.Buffer
	w := csv.NewWriter(&out)
	header := localizedCSVHeader(language)
	if err := w.Write(header); err != nil {
		return nil, err
	}
	for _, result := range results {
		row := []string{result.StartedAt.UTC().Format(time.RFC3339), result.Provider, result.ServerFQDN, strconv.FormatInt(result.DownloadBPS, 10), strconv.FormatInt(result.UploadBPS, 10), strconv.FormatInt(result.MinRTTUS, 10), string(result.Status), strconv.Itoa(result.ConfidenceScore), result.ConfidenceLevel, strconv.FormatBool(result.PublicEligible), fmt.Sprint(result.ConfidenceReasons)}
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}
	w.Flush()
	return out.Bytes(), w.Error()
}

// PDF preserves the original API for callers without a selected ISP plan.
func PDF(results []measurement.Result, periodStart, periodEnd time.Time) ([]byte, error) {
	return PDFWithPlan(results, periodStart, periodEnd, nil)
}

// PDFWithPlan creates a branded, paginated report. It remains deliberately
// conservative: observed NDT7 performance is evidence, not proof of physical
// line capacity or ISP responsibility.
func PDFWithPlan(results []measurement.Result, periodStart, periodEnd time.Time, offer *plan.Offer) ([]byte, error) {
	return PDFWithPlanLocalized(results, periodStart, periodEnd, offer, "en")
}

func PDFWithPlanLocalized(results []measurement.Result, periodStart, periodEnd time.Time, offer *plan.Offer, language string) ([]byte, error) {
	r, err := newReportPDF(language)
	if err != nil {
		return nil, err
	}
	renderPerformanceReport(r, results, periodStart, periodEnd, offer)
	return r.bytes()
}

// ComplaintPDF creates a provider-ready dossier. Subscriber details remain in
// the locally generated document and are never copied to FiberPulse logs.
func ComplaintPDF(results []measurement.Result, periodStart, periodEnd time.Time, offer *plan.Offer, profile complaint.Profile, assessment complaint.Assessment, contact complaint.SupportContact) ([]byte, error) {
	return ComplaintPDFLocalized(results, periodStart, periodEnd, offer, profile, assessment, contact, "en")
}

func ComplaintPDFLocalized(results []measurement.Result, periodStart, periodEnd time.Time, offer *plan.Offer, profile complaint.Profile, assessment complaint.Assessment, contact complaint.SupportContact, language string) ([]byte, error) {
	r, err := newReportPDF(language)
	if err != nil {
		return nil, err
	}
	r.addPage("COMPLAINT DOSSIER")
	r.drawComplaintCover(profile, offer, assessment, contact)
	renderPerformanceReport(r, results, periodStart, periodEnd, offer)
	return r.bytes()
}

func newReportPDF(language string) (*reportPDF, error) {
	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
	if err := pdf.AddTTFFontData("GoRegular", goregular.TTF); err != nil {
		return nil, err
	}
	if err := pdf.AddTTFFontData("GoBold", gobold.TTF); err != nil {
		return nil, err
	}
	if regular, err := reportAssets.ReadFile("fonts/NotoSansDevanagari-Regular.ttf"); err == nil {
		if err := pdf.AddTTFFontData("NotoDevanagariRegular", regular); err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}
	if bold, err := reportAssets.ReadFile("fonts/NotoSansDevanagari-Bold.ttf"); err == nil {
		if err := pdf.AddTTFFontData("NotoDevanagariBold", bold); err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}
	logoPNG, err := reportAssets.ReadFile("fiberpulse-mark.png")
	if err != nil {
		return nil, fmt.Errorf("read embedded report logo: %w", err)
	}
	logo, err := gopdf.ImageHolderByBytes(logoPNG)
	if err != nil {
		return nil, fmt.Errorf("decode embedded report logo: %w", err)
	}
	return &reportPDF{pdf: pdf, logo: logo, language: localization.Normalize(language), pageWidth: 595.28, pageHeight: 841.89, margin: 42}, nil
}

func renderPerformanceReport(r *reportPDF, results []measurement.Result, periodStart, periodEnd time.Time, offer *plan.Offer) {
	reportResults := reportableResults(results)
	excluded := len(results) - len(reportResults)
	complete := completedResults(reportResults)
	r.addPage("PERFORMANCE OVERVIEW")
	r.drawHero(periodStart, periodEnd)
	r.drawSummary(reportResults, complete)
	r.drawLatest(complete)
	r.drawPlanComparison(offer, complete)
	r.drawEvidenceNote(excluded)
	r.drawMeasurements(reportResults)
}

func (r *reportPDF) bytes() ([]byte, error) {
	if r.err != nil {
		return nil, r.err
	}
	var out bytes.Buffer
	if _, err := r.pdf.WriteTo(&out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

type reportPDF struct {
	pdf        *gopdf.GoPdf
	logo       gopdf.ImageHolder
	err        error
	page       int
	pageWidth  float64
	pageHeight float64
	margin     float64
	y          float64
	language   string
}

func (r *reportPDF) drawComplaintCover(profile complaint.Profile, offer *plan.Offer, assessment complaint.Assessment, contact complaint.SupportContact) {
	r.fillRect(0, 0, r.pageWidth, 126, 8, 22, 41)
	r.fillRect(0, 123, r.pageWidth, 3, 8, 199, 245)
	if r.err != nil {
		return
	}
	r.err = r.pdf.ImageByHolder(r.logo, r.margin, 26, &gopdf.Rect{W: 58, H: 58})
	r.setFont("GoBold", 21)
	r.setTextColor(255, 255, 255)
	r.text(r.margin+72, 37, "Connection complaint dossier")
	r.setFont("GoRegular", 9.5)
	r.setTextColor(87, 217, 255)
	r.text(r.margin+72, 64, "Seven-day evidence package for technical support")
	status := "COLLECTION IN PROGRESS"
	statusColor := [3]uint8{255, 183, 67}
	if assessment.ComplaintReady {
		status = "READY TO SEND"
		statusColor = [3]uint8{8, 232, 137}
	}
	r.setFont("GoBold", 8)
	r.setTextColor(statusColor[0], statusColor[1], statusColor[2])
	r.text(r.margin+72, 88, status)
	r.setFont("GoRegular", 8)
	r.setTextColor(158, 177, 197)
	r.textRight(r.pageWidth-r.margin, 37, "Generated "+time.Now().UTC().Format("2006-01-02 15:04")+" UTC")
	r.textRight(r.pageWidth-r.margin, 54, fmt.Sprintf("Evidence: %d/%d tests | %d/%d days", assessment.QualifiedTests, assessment.TargetTests, assessment.ObservedDays, assessment.TargetDays))
	r.y = 148

	r.sectionTitle("SUBSCRIBER AND SERVICE", "Details supplied by the account holder")
	r.drawKeyValueCard([][2]string{
		{"Account holder", printable(profile.FullName)},
		{"Account number", printable(profile.AccountNumber)},
		{"Service address", printable(profile.ServiceAddress)},
		{"Contact", joinedContact(profile.ContactEmail, profile.ContactPhone)},
	}, 92)

	provider, offerName, advertised := printable(contact.ISP), "Not selected", "Not available"
	if offer != nil {
		provider, offerName = offer.ISP, offer.Name
		advertised = fmt.Sprintf("Up to %d Mbps download", assessment.AdvertisedDownloadMbps)
	}
	r.sectionTitle("SUBSCRIBED OFFER AND OBSERVED PERFORMANCE", "Consolidated qualified M-Lab NDT7 measurements")
	r.drawKeyValueCard([][2]string{
		{"Provider / offer", provider + " - " + offerName},
		{"Advertised", advertised},
		{"Seven-day median", fmt.Sprintf("%.1f Mbps download (%d%%) | %.1f Mbps upload", assessment.MedianDownloadMbps, assessment.DownloadPercent, assessment.MedianUploadMbps)},
		{"Test conditions", fmt.Sprintf("%d Ethernet | %d Wi-Fi | median latency %.1f ms", assessment.WiredTests, assessment.WiFiTests, assessment.MedianLatencyMS)},
	}, 92)

	r.sectionTitle("INSTALLATION PROFILE", "Equipment and topology supplied by the subscriber")
	r.drawKeyValueCard([][2]string{
		{"Provider equipment", joinedEquipment(profile.ProviderModem, profile.ProviderRouter)},
		{"Additional router", yesModel(profile.AdditionalRouter, profile.AdditionalRouterModel)},
		{"Mesh system", yesModel(profile.MeshSystem, profile.MeshModel)},
		{"Test / layout", printable(profile.TestConnection) + " | " + printable(strings.ReplaceAll(profile.NetworkLayout, "_", " "))},
		{"Typical devices", printable(profile.TypicalDeviceCount)},
		{"Notes", printable(profile.Notes)},
	}, 119)

	r.sectionTitle("PROVIDER SUPPORT", "Verified official channel or subscriber override")
	support := joinedContact(contact.Email, contact.Phone)
	if support == "Not provided" && contact.SupportURL != "" {
		support = contact.SupportURL
	}
	r.drawKeyValueCard([][2]string{{"Contact", support}, {"Support page", printable(contact.SupportURL)}, {"Contact verified", printable(contact.VerifiedAt)}}, 74)

	r.setFont("GoRegular", 7.4)
	r.setTextColor(93, 112, 132)
	r.wrappedText(r.margin, r.y+2, r.pageWidth-2*r.margin, 9.5, "Methodology note: these are application-level M-Lab NDT7 measurements. Repeated low results support a technical investigation, but do not by themselves prove ISP responsibility or physical line capacity. Personal details on this page were entered locally by the subscriber.")
}

func (r *reportPDF) drawKeyValueCard(rows [][2]string, height float64) {
	r.fillRect(r.margin, r.y, r.pageWidth-2*r.margin, height, 246, 249, 252)
	rowHeight := height / float64(len(rows))
	for i, row := range rows {
		y := r.y + float64(i)*rowHeight
		if i > 0 {
			r.pdf.SetStrokeColor(224, 232, 239)
			r.pdf.SetLineWidth(0.4)
			r.pdf.Line(r.margin+12, y, r.pageWidth-r.margin-12, y)
		}
		r.setFont("GoBold", 7.3)
		r.setTextColor(92, 111, 132)
		r.text(r.margin+13, y+7, strings.ToUpper(row[0]))
		r.setFont("GoRegular", 8.2)
		r.setTextColor(24, 46, 67)
		r.wrappedText(r.margin+137, y+7, r.pageWidth-2*r.margin-150, 9, row[1])
	}
	r.y += height + 14
}

func (r *reportPDF) addPage(section string) {
	if r.err != nil {
		return
	}
	r.pdf.AddPage()
	r.page++
	r.y = r.margin
	r.drawFooter()
	if r.page > 1 {
		r.fillRect(0, 0, r.pageWidth, 48, 8, 22, 41)
		r.setFont("GoBold", 8)
		r.setTextColor(87, 217, 255)
		r.text(r.margin, 19, "FIBERPULSE  /  "+section)
		r.y = 68
	}
}

func (r *reportPDF) drawFooter() {
	if r.err != nil {
		return
	}
	y := r.pageHeight - 30
	r.pdf.SetStrokeColor(215, 225, 235)
	r.pdf.SetLineWidth(0.55)
	r.pdf.Line(r.margin, y, r.pageWidth-r.margin, y)
	r.setFont("GoRegular", 7.5)
	r.setTextColor(112, 130, 151)
	r.text(r.margin, y+8, "FiberPulse - local-first Internet performance evidence")
	r.textRight(r.pageWidth-r.margin, y+8, fmt.Sprintf("Page %d", r.page))
}

func (r *reportPDF) drawHero(periodStart, periodEnd time.Time) {
	r.fillRect(0, 0, r.pageWidth, 112, 8, 22, 41)
	r.fillRect(0, 109, r.pageWidth, 3, 8, 199, 245)
	if r.err != nil {
		return
	}
	r.err = r.pdf.ImageByHolder(r.logo, r.margin, 24, &gopdf.Rect{W: 58, H: 58})
	r.setFont("GoBold", 22)
	r.setTextColor(255, 255, 255)
	r.text(r.margin+72, 36, "FiberPulse")
	r.setFont("GoRegular", 10.5)
	r.setTextColor(87, 217, 255)
	r.text(r.margin+72, 61, "Internet performance report")
	r.setFont("GoRegular", 8.5)
	r.setTextColor(158, 177, 197)
	r.textRight(r.pageWidth-r.margin, 38, "Generated "+time.Now().UTC().Format("2006-01-02 15:04")+" UTC")
	r.textRight(r.pageWidth-r.margin, 54, "Period "+periodStart.UTC().Format("2006-01-02")+" to "+periodEnd.UTC().Format("2006-01-02")+" UTC")
	r.textRight(r.pageWidth-r.margin, 70, "Private local report")
	r.y = 132
}

func (r *reportPDF) drawSummary(all, complete []measurement.Result) {
	down, up, latency := medianMetrics(complete)
	qualified := 0
	for _, result := range complete {
		if result.ConfidenceScore >= 70 {
			qualified++
		}
	}
	cards := []struct {
		label string
		value string
		color [3]uint8
	}{
		{"COMPLETE TESTS", fmt.Sprintf("%d / %d", len(complete), len(all)), [3]uint8{8, 124, 255}},
		{"MEDIAN DOWNLOAD", metricMbps(down), [3]uint8{8, 124, 255}},
		{"MEDIAN UPLOAD", metricMbps(up), [3]uint8{8, 232, 137}},
		{"MEDIAN LATENCY", metricMS(latency), [3]uint8{255, 183, 67}},
	}
	gap := 9.0
	width := (r.pageWidth - 2*r.margin - 3*gap) / 4
	for i, card := range cards {
		x := r.margin + float64(i)*(width+gap)
		r.fillRect(x, r.y, width, 70, 246, 249, 252)
		r.fillRect(x, r.y, 4, 70, card.color[0], card.color[1], card.color[2])
		r.setFont("GoBold", 7)
		r.setTextColor(94, 113, 137)
		r.text(x+12, r.y+15, card.label)
		r.setFont("GoBold", 17)
		r.setTextColor(16, 36, 58)
		r.text(x+12, r.y+35, card.value)
		if i == 0 {
			r.setFont("GoRegular", 7)
			r.setTextColor(113, 131, 151)
			r.text(x+12, r.y+57, fmt.Sprintf("%d qualified", qualified))
		}
	}
	r.y += 88
}

func (r *reportPDF) drawLatest(complete []measurement.Result) {
	r.sectionTitle("LATEST MEASUREMENT", "The most recent complete test in this report")
	if len(complete) == 0 {
		r.infoBox("No complete measurement is available.", 48, 246, 249, 252)
		return
	}
	latest := complete[0]
	r.fillRect(r.margin, r.y, r.pageWidth-2*r.margin, 80, 8, 22, 41)
	labels := []string{"DOWNLOAD", "UPLOAD", "MIN LATENCY", "CONFIDENCE"}
	values := []string{metricMbps(float64(latest.DownloadBPS)), metricMbps(float64(latest.UploadBPS)), metricMS(float64(latest.MinRTTUS)), fmt.Sprintf("%d / 100", latest.ConfidenceScore)}
	column := (r.pageWidth - 2*r.margin) / 4
	for i := range labels {
		x := r.margin + float64(i)*column
		if i > 0 {
			r.pdf.SetStrokeColor(35, 61, 85)
			r.pdf.SetLineWidth(0.5)
			r.pdf.Line(x, r.y+16, x, r.y+64)
		}
		r.setFont("GoBold", 7)
		r.setTextColor(87, 217, 255)
		r.text(x+15, r.y+19, labels[i])
		r.setFont("GoBold", 16)
		r.setTextColor(255, 255, 255)
		r.text(x+15, r.y+38, values[i])
		r.setFont("GoRegular", 7)
		r.setTextColor(153, 173, 193)
		subtitle := latest.StartedAt.UTC().Format("2006-01-02 15:04 UTC")
		if i == 3 {
			subtitle = strings.ToUpper(latest.ConfidenceLevel)
		}
		r.text(x+15, r.y+62, subtitle)
	}
	r.y += 98
}

func (r *reportPDF) drawPlanComparison(offer *plan.Offer, complete []measurement.Result) {
	r.sectionTitle("PLAN CHECK", "Measured performance compared with the selected Internet offer")
	if offer == nil {
		r.infoBox("No Internet plan was selected when this report was generated. Select your provider and offer in FiberPulse to include a plan verdict.", 52, 239, 246, 252)
		return
	}
	if len(complete) == 0 {
		r.infoBox("Plan selected: "+offer.ISP+" - "+offer.Name+". Run a complete test to generate a verdict.", 52, 239, 246, 252)
		return
	}
	verdict := plan.AssessAt(*offer, complete[0].DownloadBPS, complete[0].UploadBPS, complete[0].StartedAt)
	background := [3]uint8{232, 250, 242}
	accent := [3]uint8{8, 170, 101}
	if verdict.Level == plan.LevelBelow {
		background = [3]uint8{255, 247, 224}
		accent = [3]uint8{215, 142, 0}
	}
	if verdict.Level == plan.LevelWellBelow {
		background = [3]uint8{255, 235, 238}
		accent = [3]uint8{220, 67, 82}
	}
	height := 108.0
	r.fillRect(r.margin, r.y, r.pageWidth-2*r.margin, height, background[0], background[1], background[2])
	r.fillRect(r.margin, r.y, 5, height, accent[0], accent[1], accent[2])
	r.setFont("GoBold", 9)
	r.setTextColor(accent[0], accent[1], accent[2])
	r.text(r.margin+17, r.y+15, strings.ToUpper(verdict.Summary))
	r.setFont("GoBold", 13)
	r.setTextColor(21, 42, 61)
	r.text(r.margin+17, r.y+34, offer.ISP+" - "+offer.Name)
	r.setFont("GoRegular", 8.5)
	r.setTextColor(70, 91, 112)
	comparison := fmt.Sprintf("Latest download: %.1f Mbps  |  Advertised: up to %d Mbps  |  Result: %d%%", float64(complete[0].DownloadBPS)/1e6, verdict.AdvertisedDownloadMbps, verdict.DownloadPct)
	r.text(r.margin+17, r.y+53, comparison)
	metadata := offer.CountryName + "  |  Subscriber-entered offer; advertised speed should be checked against the latest bill."
	if !offer.Custom {
		metadata = offer.CountryName + "  |  Official provider catalog checked " + offer.VerifiedAt
		amount := offer.PriceAmount
		currency := offer.CurrencyCode
		if amount == 0 && offer.PricePHP > 0 {
			amount = float64(offer.PricePHP)
			currency = plan.PhilippinesCurrency
		}
		if amount > 0 {
			metadata += fmt.Sprintf("  |  %s %s / %s", currency, formatPrice(amount), offer.PricePeriod)
		}
	}
	r.setFont("GoRegular", 7.5)
	r.setTextColor(92, 110, 129)
	r.text(r.margin+17, r.y+68, metadata)
	r.setFont("GoRegular", 8.5)
	r.setTextColor(70, 91, 112)
	r.wrappedText(r.margin+17, r.y+83, r.pageWidth-2*r.margin-34, 10, verdict.Advice)
	r.y += height + 18
}

func (r *reportPDF) drawEvidenceNote(excludedDevelopmentResults int) {
	r.sectionTitle("HOW TO USE THIS REPORT", "Evidence to support diagnosis and a conversation with your provider")
	text := "FiberPulse reports application-level NDT7 measurements to a nearby neutral M-Lab server. Tests are refused while a VPN route is detected. Repeated low results are useful evidence of delivered performance, but one test alone does not prove ISP responsibility or physical line capacity. For a complaint, run tests at different times, connect by Ethernet directly to the provider router where possible, pause heavy traffic and attach this PDF plus the CSV export."
	if excludedDevelopmentResults > 0 {
		text += fmt.Sprintf(" %d development simulation result(s) were excluded from this evidence report.", excludedDevelopmentResults)
	}
	r.infoBox(text, 68, 246, 249, 252)
}

func (r *reportPDF) drawMeasurements(results []measurement.Result) {
	rows := results
	const maximumPDFRows = 100
	truncated := len(rows) > maximumPDFRows
	if truncated {
		rows = rows[:maximumPDFRows]
	}
	if r.y > 610 {
		r.addPage("MEASUREMENT HISTORY")
	}
	r.sectionTitle("MEASUREMENT HISTORY", "Recent observations in reverse chronological order")
	r.tableHeader()
	for _, result := range rows {
		if r.y > r.pageHeight-65 {
			r.addPage("MEASUREMENT HISTORY")
			r.tableHeader()
		}
		r.tableRow(result)
	}
	if truncated {
		if r.y > r.pageHeight-80 {
			r.addPage("MEASUREMENT HISTORY")
		}
		r.infoBox("This PDF includes the 100 most recent rows. Use the CSV export for the complete dataset.", 38, 246, 249, 252)
	}
}

func (r *reportPDF) tableHeader() {
	r.fillRect(r.margin, r.y, r.pageWidth-2*r.margin, 24, 16, 36, 58)
	headers := []string{"DATE UTC", "DOWN", "UP", "RTT", "CONF.", "STATUS"}
	widths := []float64{132, 82, 82, 62, 70, 83}
	x := r.margin
	r.setFont("GoBold", 7)
	r.setTextColor(255, 255, 255)
	for i, header := range headers {
		r.text(x+8, r.y+9, header)
		x += widths[i]
	}
	r.y += 24
}

func (r *reportPDF) tableRow(result measurement.Result) {
	fill := [3]uint8{255, 255, 255}
	if int(r.y/22)%2 == 0 {
		fill = [3]uint8{246, 249, 252}
	}
	r.fillRect(r.margin, r.y, r.pageWidth-2*r.margin, 22, fill[0], fill[1], fill[2])
	values := []string{
		result.StartedAt.UTC().Format("2006-01-02 15:04"),
		fmt.Sprintf("%.1f Mbps", float64(result.DownloadBPS)/1e6),
		fmt.Sprintf("%.1f Mbps", float64(result.UploadBPS)/1e6),
		fmt.Sprintf("%.1f ms", float64(result.MinRTTUS)/1e3),
		fmt.Sprintf("%d / 100", result.ConfidenceScore),
		strings.ToUpper(string(result.Status)),
	}
	widths := []float64{132, 82, 82, 62, 70, 83}
	x := r.margin
	r.setFont("GoRegular", 7.3)
	r.setTextColor(41, 60, 79)
	for i, value := range values {
		r.text(x+8, r.y+8, value)
		x += widths[i]
	}
	r.pdf.SetStrokeColor(226, 233, 240)
	r.pdf.SetLineWidth(0.35)
	r.pdf.Line(r.margin, r.y+22, r.pageWidth-r.margin, r.y+22)
	r.y += 22
}

func (r *reportPDF) sectionTitle(title, subtitle string) {
	r.setFont("GoBold", 10)
	r.setTextColor(16, 36, 58)
	r.text(r.margin, r.y, title)
	r.setFont("GoRegular", 7.5)
	r.setTextColor(112, 130, 151)
	r.textRight(r.pageWidth-r.margin, r.y+1, subtitle)
	r.y += 20
}

func (r *reportPDF) infoBox(text string, height float64, red, green, blue uint8) {
	r.fillRect(r.margin, r.y, r.pageWidth-2*r.margin, height, red, green, blue)
	r.setFont("GoRegular", 8.3)
	r.setTextColor(62, 82, 103)
	r.wrappedText(r.margin+14, r.y+15, r.pageWidth-2*r.margin-28, 11, text)
	r.y += height + 18
}

func (r *reportPDF) fillRect(x, y, width, height float64, red, green, blue uint8) {
	if r.err != nil {
		return
	}
	r.pdf.SetFillColor(red, green, blue)
	r.pdf.RectFromUpperLeftWithStyle(x, y, width, height, "F")
}

func (r *reportPDF) setFont(name string, size float64) {
	if r.err == nil {
		if r.language == "hi" {
			if name == "GoBold" {
				name = "NotoDevanagariBold"
			} else {
				name = "NotoDevanagariRegular"
			}
		}
		r.err = r.pdf.SetFont(name, "", size)
	}
}

func (r *reportPDF) setTextColor(red, green, blue uint8) {
	if r.err == nil {
		r.pdf.SetTextColor(red, green, blue)
	}
}

func (r *reportPDF) text(x, y float64, value string) {
	if r.err != nil {
		return
	}
	value = localizeReportText(value, r.language)
	r.pdf.SetXY(x, y)
	r.err = r.pdf.Text(value)
}

func (r *reportPDF) textRight(right, y float64, value string) {
	if r.err != nil {
		return
	}
	value = localizeReportText(value, r.language)
	width, err := r.pdf.MeasureTextWidth(value)
	if err != nil {
		r.err = err
		return
	}
	r.text(right-width, y, value)
}

func (r *reportPDF) wrappedText(x, y, width, lineHeight float64, value string) float64 {
	if r.err != nil {
		return 0
	}
	value = localizeReportText(value, r.language)
	lines, err := r.pdf.SplitText(value, width)
	if err != nil {
		r.err = err
		return 0
	}
	for i, line := range lines {
		r.text(x, y+float64(i)*lineHeight, line)
	}
	return float64(len(lines)) * lineHeight
}

func completedResults(results []measurement.Result) []measurement.Result {
	out := make([]measurement.Result, 0, len(results))
	for _, result := range results {
		if result.Status == measurement.StatusComplete && result.DownloadBPS > 0 {
			out = append(out, result)
		}
	}
	return out
}

func reportableResults(results []measurement.Result) []measurement.Result {
	out := make([]measurement.Result, 0, len(results))
	for _, result := range results {
		if result.Provider != measurement.ProviderDevelopmentFake {
			out = append(out, result)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].StartedAt.After(out[j].StartedAt) })
	return out
}

func medianMetrics(results []measurement.Result) (download, upload, latency float64) {
	if len(results) == 0 {
		return 0, 0, 0
	}
	down := make([]float64, 0, len(results))
	up := make([]float64, 0, len(results))
	rtt := make([]float64, 0, len(results))
	for _, result := range results {
		down = append(down, float64(result.DownloadBPS))
		up = append(up, float64(result.UploadBPS))
		rtt = append(rtt, float64(result.MinRTTUS))
	}
	return median(down), median(up), median(rtt)
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	copyValues := append([]float64(nil), values...)
	sort.Float64s(copyValues)
	middle := len(copyValues) / 2
	if len(copyValues)%2 == 1 {
		return copyValues[middle]
	}
	return (copyValues[middle-1] + copyValues[middle]) / 2
}

func metricMbps(bitsPerSecond float64) string {
	if bitsPerSecond <= 0 {
		return "-"
	}
	return fmt.Sprintf("%.0f Mbps", bitsPerSecond/1e6)
}

func metricMS(microseconds float64) string {
	if microseconds <= 0 {
		return "-"
	}
	return fmt.Sprintf("%.1f ms", microseconds/1e3)
}

func formatInteger(value int) string {
	digits := strconv.Itoa(value)
	for i := len(digits) - 3; i > 0; i -= 3 {
		digits = digits[:i] + "," + digits[i:]
	}
	return digits
}

// formatPrice renders catalog prices: whole amounts stay integers,
// fractional amounts (EUR, CHF, AUD tiers) keep two decimals.
func formatPrice(amount float64) string {
	if amount == float64(int64(amount)) {
		return formatInteger(int(amount))
	}
	return strconv.FormatFloat(amount, 'f', 2, 64)
}

func printable(value string) string {
	if strings.TrimSpace(value) == "" {
		return "Not provided"
	}
	return strings.TrimSpace(value)
}

func joinedContact(email, phone string) string {
	values := make([]string, 0, 2)
	if strings.TrimSpace(email) != "" {
		values = append(values, strings.TrimSpace(email))
	}
	if strings.TrimSpace(phone) != "" {
		values = append(values, strings.TrimSpace(phone))
	}
	if len(values) == 0 {
		return "Not provided"
	}
	return strings.Join(values, " | ")
}

func joinedEquipment(modem, router string) string {
	return "Modem / ONT: " + printable(modem) + " | Router: " + printable(router)
}

func yesModel(enabled bool, model string) string {
	if !enabled {
		return "No"
	}
	return "Yes | " + printable(model)
}
