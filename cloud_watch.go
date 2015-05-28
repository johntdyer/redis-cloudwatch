package main

import (
	logrus "github.com/Sirupsen/logrus"
	"github.com/awslabs/aws-sdk-go/aws"
	"github.com/awslabs/aws-sdk-go/aws/awserr"
	"github.com/awslabs/aws-sdk-go/aws/awsutil"
	"github.com/awslabs/aws-sdk-go/service/cloudwatch"
	"time"
)

func sendCloudWatchMetric(count float64) {

	svc := cloudwatch.New(auth)

	params := &cloudwatch.PutMetricDataInput{
		MetricData: []*cloudwatch.MetricDatum{
			&cloudwatch.MetricDatum{
				MetricName: aws.String(*metricName),
				Timestamp:  aws.Time(time.Now()),
				Unit:       aws.String("Count"),
				Value:      aws.Double(count),
			},
		},
		Namespace: aws.String(*metricNamespace),
	}
	resp, err := svc.PutMetricData(params)

	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			// Generic AWS Error with Code, Message, and original error (if any)
			logrus.Error(awsErr.Code(), awsErr.Message(), awsErr.OrigErr())
			if reqErr, ok := err.(awserr.RequestFailure); ok {
				// A service error occurred
				logrus.Error(reqErr.Code(), reqErr.Message(), reqErr.StatusCode(), reqErr.RequestID())
			}
		} else {
			// This case should never be hit, The SDK should alwsy return an
			// error which satisfies the awserr.Error interface.

			logrus.Error(err.Error())
		}
	}

	// Pretty-print the response data.
	logrus.Debug(awsutil.StringValue(resp))

}
