package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/mctofu/water-monitor/water"
)

func main() {
	user := os.Getenv("WATER_USER")
	pass := os.Getenv("WATER_PASS")
	acct := os.Getenv("WATER_ACCT")

	if len(os.Args) != 3 {
		log.Fatalf("usage: <hourly|monthly> <yyyyMM>")
	}

	start, err := time.Parse("200601", os.Args[2])
	if err != nil {
		log.Fatalf("failed to parse start date %s: %v", os.Args[2], err)
	}
	end := start.AddDate(0, 1, 0)
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if end.After(today) {
		end = today
	}

	switch os.Args[1] {
	case "hourly":
		report, err := water.DownloadHourlyUsage(start, end, user, pass, acct)
		if err != nil {
			log.Fatalf("failed to retrieve water usage: %v\n", err)
		}

		log.Printf("Usage data:\n%s\t%s\n", report.LabelHeader, report.ValueHeader)
		for _, record := range report.Records {
			fmt.Printf("%s\t%f\n", record.Label, record.Value)
		}
	case "daily":
		report, err := water.DownloadDailyUsage(start, end, user, pass, acct)
		if err != nil {
			log.Fatalf("failed to retrieve water usage: %v\n", err)
		}

		log.Printf("Usage data:\n%s\t%s\n", report.LabelHeader, report.ValueHeader)
		for _, record := range report.Records {
			fmt.Printf("%s\t%f\n", record.Label, record.Value)
		}
	}
}
