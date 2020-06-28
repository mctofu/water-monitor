// Daily job to check recent water usage and alert if it exceeds
// normal levels.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/mctofu/water-monitor/water"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "local" {
		if err := checkMonitor(context.Background()); err != nil {
			log.Fatalf("checkMonitor failed: %v\n", err)
		}
		return
	}

	lambda.Start(checkLambda)
}

func checkLambda(ctx context.Context, evt events.CloudWatchEvent) error {
	log.Printf("lambda execution starting\n")
	err := checkMonitor(ctx)
	log.Printf("lambda execution finished with response: %v\n", err)
	return err
}

func checkMonitor(ctx context.Context) error {
	user := os.Getenv("WATER_USER")
	pass := os.Getenv("WATER_PASS")
	acct := os.Getenv("WATER_ACCT")
	usageSNSARN := os.Getenv("USAGE_SNS_ARN")
	failSNSARN := os.Getenv("FAIL_SNS_ARN")
	reportSNSARN := os.Getenv("REPORT_SNS_ARN")

	sess := session.Must(session.NewSession())
	snsAPI := sns.New(sess)
	usageAlerter := snsAlerter{snsAPI, usageSNSARN}
	failAlerter := snsAlerter{snsAPI, failSNSARN}

	report, err := water.DownloadDailyUsage(time.Time{}, time.Time{}, user, pass, acct)
	if err != nil {
		alertErr := failAlerter.Alert(ctx, fmt.Sprintf("Failed to retrieve water usage: %v", err))
		if alertErr != nil {
			return fmt.Errorf("failed to alert: %v\n orig alert: %v", alertErr, err)
		}
		return fmt.Errorf("failed to retrieve water usage: %v", err)
	}

	log.Printf("Usage data:\n%s\n", report)

	err = water.AnalyzeUsage(ctx, report, 1500, 2000, usageAlerter)
	if err != nil {
		alertErr := failAlerter.Alert(ctx, fmt.Sprintf("Failed to analyze water usage: %v", err))
		if alertErr != nil {
			return fmt.Errorf("failed to alert: %v\n orig alert: %v", alertErr, err)
		}
		return fmt.Errorf("failed to analyze water usage: %v", err)
	}

	if time.Now().Weekday() == time.Sunday {
		if err := publish(ctx, snsAPI, reportSNSARN, "Water monitor summary", report.String()); err != nil {
			alertErr := failAlerter.Alert(ctx, fmt.Sprintf("Failed to report water usage: %v", err))
			if alertErr != nil {
				return fmt.Errorf("failed to alert: %v\n orig alert: %v", alertErr, err)
			}
			return fmt.Errorf("failed to report water usage: %v", err)
		}
	}

	return nil
}

type snsAlerter struct {
	Publisher *sns.SNS
	ARN       string
}

func (a snsAlerter) Alert(ctx context.Context, msg string) error {
	log.Printf("Alerting: %v\n", msg)
	return publish(
		ctx,
		a.Publisher,
		a.ARN,
		"Water monitor alert",
		msg,
	)
}

func publish(ctx context.Context, pub *sns.SNS, arn string, subj, msg string) error {
	_, err := pub.PublishWithContext(ctx, &sns.PublishInput{
		Message:  aws.String(msg),
		Subject:  aws.String(subj),
		TopicArn: aws.String(arn),
	})
	return err
}
