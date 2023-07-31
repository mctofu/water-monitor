package water

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/fcjr/aia-transport-go"
	"github.com/headzoo/surf"
	"github.com/headzoo/surf/browser"
)

// Alerter notifies about alerts
type Alerter interface {
	Alert(ctx context.Context, msg string) error
}

type Report struct {
	LabelHeader string
	ValueHeader string
	Records     []Record
}

func (r *Report) String() string {
	var buf strings.Builder
	if _, err := fmt.Fprintf(&buf, "%s\t%s\n", r.LabelHeader, r.ValueHeader); err != nil {
		panic(err)
	}
	for _, rec := range r.Records {
		if _, err := fmt.Fprintf(&buf, "%s\t%f\n", rec.Label, rec.Value); err != nil {
			panic(err)
		}
	}
	return buf.String()
}

type Record struct {
	Label string
	Value float64
}

func login(user, pass, acct string) (*browser.Browser, error) {
	b := surf.NewBrowser()
	b.SetUserAgent("Mozilla/5.0 (Windows NT 6.1; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/69.0.3497.100 Safari/537.36")
	b.AddRequestHeader("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	// workaround incomplete certificate chain
	tr, err := aia.NewTransport()
	if err != nil {
		return nil, fmt.Errorf("couldn't create incomplete certificate chain workaround transport: %v", err)
	}
	b.SetTransport(tr)

	err = b.Open("https://myaccount.sfwater.org/")
	if err != nil {
		return nil, fmt.Errorf("couldn't open site: %v", err)
	}

	loginForm, err := b.Form("#form1")
	if err != nil {
		return nil, fmt.Errorf("couldn't find login form: %v", err)
	}

	err = loginForm.Input("tb_USER_ID", user)
	if err != nil {
		return nil, fmt.Errorf("couldn't set userId field: %v", err)
	}

	err = loginForm.Input("tb_USER_PSWD", pass)
	if err != nil {
		return nil, fmt.Errorf("couldn't set password field: %v", err)
	}

	err = loginForm.Submit()
	if err != nil {
		return nil, fmt.Errorf("failed to submit login: %v", err)
	}

	accountForm, err := b.Form("#form1")
	if err != nil {
		return nil, fmt.Errorf("couldn't find account form: %v", err)
	}

	err = accountForm.Input("dl_ACCOUNT", acct)
	if err != nil {
		return nil, fmt.Errorf("couldn't set account field: %v", err)
	}

	err = accountForm.Submit()
	if err != nil {
		return nil, fmt.Errorf("failed to submit account selection: %v", err)
	}

	return b, nil
}

// DownloadDailyUsage returns a report of daily water usage data
func DownloadDailyUsage(startDay, endDay time.Time, user, pass, acct string) (*Report, error) {
	if startDay.Year() != endDay.Year() {
		return nil, fmt.Errorf("startDay year should match endDay year")
	}

	b, err := login(user, pass, acct)
	if err != nil {
		return nil, fmt.Errorf("failed login: %v", err)
	}

	err = b.Click("#dailyMenu a")
	if err != nil {
		return nil, fmt.Errorf("failed to select daily usage: %v", err)
	}

	dailyPage := b.Url()
	fullReport := Report{}

	for date := startDay; date.Before(endDay) || date.IsZero(); date = date.Add(30 * time.Hour * 24) {
		batchEnd := date.Add(30 * time.Hour * 24)
		if batchEnd.After(endDay) {
			batchEnd = endDay
		}

		time.Sleep(3 * time.Second)

		log.Printf("Load %s - %s\n", date.Format("1/2/2006"), batchEnd.Format("1/2/2006"))

		dlForm, err := b.Form("#form1")
		if err != nil {
			return nil, fmt.Errorf("couldn't find dl form: %v", err)
		}

		err = dlForm.Set("img_EXCEL_DOWNLOAD_IMAGE.x", "7")
		if err != nil {
			return nil, fmt.Errorf("couldn't set excel.x: %v", err)
		}

		err = dlForm.Set("img_EXCEL_DOWNLOAD_IMAGE.y", "2")
		if err != nil {
			return nil, fmt.Errorf("couldn't set excel.y: %v", err)
		}

		err = dlForm.Input("dl_ACCOUNT", acct)
		if err != nil {
			return nil, fmt.Errorf("couldn't set account field: %v", err)
		}

		if !startDay.IsZero() {
			err = dlForm.Input("SD", date.Format("1/2/2006"))
			if err != nil {
				return nil, fmt.Errorf("couldn't set start date field: %v", err)
			}

			// convert exclusive end to inclusive
			err = dlForm.Input("ED", batchEnd.Add(-24*time.Hour).Format("1/2/2006"))
			if err != nil {
				return nil, fmt.Errorf("couldn't set end date field: %v", err)
			}
		}

		err = dlForm.Submit()
		if err != nil {
			return nil, fmt.Errorf("couldn't submit dlForm: %v", err)
		}

		var capture bytes.Buffer
		_, err = b.Download(&capture)
		if err != nil {
			return nil, fmt.Errorf("failed to download usage data: %v", err)
		}

		report, err := parseUsage(capture.String())
		if err != nil {
			return nil, err
		}

		fullReport.LabelHeader = report.LabelHeader
		fullReport.ValueHeader = report.ValueHeader

		var year string
		if date.IsZero() {
			year = time.Now().Format("2006")
		} else {
			year = date.Format("2006")
		}

		for _, record := range report.Records {
			datetime := fmt.Sprintf("%s %s", year, record.Label)
			parsed, err := time.Parse("2006 1/2", datetime)
			if err != nil {
				return nil, fmt.Errorf("failed to parse %s: %v", datetime, err)
			}

			fullReport.Records = append(fullReport.Records,
				Record{
					Label: parsed.Format("2006-01-02"),
					Value: record.Value,
				})
		}

		if err := b.Open(dailyPage.String()); err != nil {
			return nil, fmt.Errorf("failed to return to daily: %v", err)
		}
	}

	return &fullReport, nil
}

// DownloadHourlyUsage returns a report of hourly water usage data
func DownloadHourlyUsage(start, end time.Time, user, pass, acct string) (*Report, error) {
	b, err := login(user, pass, acct)
	if err != nil {
		return nil, fmt.Errorf("failed login: %v", err)
	}

	log.Printf("click hourly\n")

	err = b.Click("#hourlyMenu a")
	if err != nil {
		return nil, fmt.Errorf("failed to select hourly usage: %v", err)
	}

	hourlyPage := b.Url()

	fullReport := Report{}

	for date := start; date.Before(end); date = date.Add(time.Hour * 24) {
		time.Sleep(3 * time.Second)

		log.Printf("Load %s\n", date.Format("1/2/2006"))

		dlForm, err := b.Form("#form1")
		if err != nil {
			return nil, fmt.Errorf("couldn't find dl form: %v", err)
		}

		err = dlForm.Set("img_EXCEL_DOWNLOAD_IMAGE.x", "7")
		if err != nil {
			return nil, fmt.Errorf("couldn't set excel.x: %v", err)
		}

		err = dlForm.Set("img_EXCEL_DOWNLOAD_IMAGE.y", "2")
		if err != nil {
			return nil, fmt.Errorf("couldn't set excel.y: %v", err)
		}

		err = dlForm.Input("dl_ACCOUNT", acct)
		if err != nil {
			log.Printf("Failed body:\n%s", b.Body())
			return nil, fmt.Errorf("couldn't set account field: %v", err)
		}

		err = dlForm.Input("SD", date.Format("1/2/2006"))
		if err != nil {
			return nil, fmt.Errorf("couldn't set date field: %v", err)
		}

		err = dlForm.Submit()
		if err != nil {
			return nil, fmt.Errorf("couldn't submit dlForm: %v", err)
		}

		var capture bytes.Buffer
		_, err = b.Download(&capture)
		if err != nil {
			log.Printf("Failed out:\n%s", capture.String())
			return nil, fmt.Errorf("failed to download usage data: %v", err)
		}

		report, err := parseUsage(capture.String())
		if err != nil {
			log.Printf("Failed parse:\n%s", capture.String())
			return nil, err
		}

		fullReport.LabelHeader = report.LabelHeader
		fullReport.ValueHeader = report.ValueHeader
		for _, record := range report.Records {
			datetime := fmt.Sprintf("%s %s", date.Format("2006-01-02"), record.Label)
			parsed, err := time.Parse("2006-01-02 3 PM", datetime)
			if err != nil {
				return nil, fmt.Errorf("failed to parse %s: %v", datetime, err)
			}

			fullReport.Records = append(fullReport.Records,
				Record{
					Label: parsed.Format("2006-01-02 15:04"),
					Value: record.Value,
				})
		}

		if err := b.Open(hourlyPage.String()); err != nil {
			return nil, fmt.Errorf("failed to return to hourly: %v", err)
		}
	}
	return &fullReport, nil
}

func parseUsage(rawUsage string) (*Report, error) {
	reader := csv.NewReader(strings.NewReader(rawUsage))
	reader.Comma = '\t'
	reader.FieldsPerRecord = 2

	allRecords, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse usage data: %v", err)
	}

	numRecords := len(allRecords)
	if numRecords < 2 {
		return nil, fmt.Errorf("too few records parsed from usage: %d", numRecords)
	}

	report := &Report{
		LabelHeader: allRecords[0][0],
		ValueHeader: allRecords[0][1],
	}

	for i := 1; i < len(allRecords); i++ {
		label := allRecords[i][0]
		value, err := strconv.ParseFloat(allRecords[i][1], 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s - %s: %v", allRecords[i][0], allRecords[i][1], err)
		}
		report.Records = append(report.Records, Record{label, value})
	}

	return report, nil
}

// AnalyzeUsage does a simple analysis of maximum and 2 day avg water usage and alert if thresholds are crossed.
func AnalyzeUsage(ctx context.Context, report *Report, twoDayAvgMax, oneDayMax int, alerter Alerter) error {
	numRecords := len(report.Records)
	if numRecords < 3 {
		return fmt.Errorf("too few records parsed from usage: %d", numRecords)
	}

	lastRecord := report.Records[numRecords-1]
	lastUsage := int(lastRecord.Value)
	if lastUsage >= oneDayMax {
		if err := alerter.Alert(ctx, fmt.Sprintf("Last day water usage of %d gallons is greater than %d gallon limit.\n%s", lastUsage, oneDayMax, report.String())); err != nil {
			return fmt.Errorf("failed to alert: %v", err)
		}
	}

	penultimateRecord := report.Records[numRecords-2]
	penultimateUsage := int(penultimateRecord.Value)
	twoDayAvgUsage := (penultimateUsage + lastUsage) / 2
	if twoDayAvgUsage >= twoDayAvgMax {
		if err := alerter.Alert(ctx, fmt.Sprintf("Two day avg water usage of %d gallons is greater than %d gallon limit.\n%s", twoDayAvgUsage, twoDayAvgMax, report.String())); err != nil {
			return fmt.Errorf("failed to alert: %v", err)
		}
	}

	return nil
}
