// read a data file and group data by periods of the day
package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type daySummary struct {
	day         time.Time
	periodUsage [6]float64
}

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("Usage: <datafile>")
	}

	file, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatalf("Couldn't open file: %v", err)
	}
	defer file.Close()

	var summaries []daySummary
	var currentSummary *daySummary

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, "\t")
		tm, err := time.Parse("2006-01-02 15:04", parts[0])
		if err != nil {
			log.Fatalf("Failed to parse date: %s: %v", parts[0], err)
		}
		usage, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			log.Fatalf("Failed to parse usage: %s: %v", parts[1], err)
		}
		day := tm.Truncate(24 * time.Hour)
		if currentSummary == nil || currentSummary.day != day {
			summaries = append(summaries, daySummary{day: day})
			currentSummary = &summaries[len(summaries)-1]
		}
		currentSummary.periodUsage[tm.Hour()/4] += usage
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Error parsing file: %v", err)
	}

	fmt.Println(strings.Join([]string{"date", "12am-4am", "4am-8am", "8am-12pm", "12pm-4pm", "4pm-8pm", "8pm-12am"}, "\t"))
	for _, summary := range summaries {
		fmt.Printf("%s\t%f\t%f\t%f\t%f\t%f\t%f\n",
			summary.day.Format("2006-01-02"),
			summary.periodUsage[0],
			summary.periodUsage[1],
			summary.periodUsage[2],
			summary.periodUsage[3],
			summary.periodUsage[4],
			summary.periodUsage[5],
		)
	}
}
