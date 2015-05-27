package main

import (
	"fmt"
	logrus "github.com/Sirupsen/logrus"
	"github.com/alecthomas/kingpin"
	"github.com/awslabs/aws-sdk-go/aws"
	"github.com/awslabs/aws-sdk-go/aws/awserr"
	"github.com/awslabs/aws-sdk-go/aws/awsutil"
	"github.com/awslabs/aws-sdk-go/aws/credentials"
	"github.com/awslabs/aws-sdk-go/service/cloudwatch"
	"github.com/hoisie/redis"
	"os"
	"time"
)

type RedisInstance struct {
	Name       string
	Connection redis.Client
}

type App struct {
	Instances  []*RedisInstance
	TotalCount int
}

const (
	version = "0.0.1"
)

var (
	to_cloudwatch          = kingpin.Flag("push-metrics", "Send metrics to cloud watch").Short('c').Bool()
	use_iam                = kingpin.Flag("use-iam", "Use IAM Profile for authentication").Short('i').Bool()
	verbose                = kingpin.Flag("verbose", "Verbose mode.").Short('v').Bool()
	metric_name            = kingpin.Flag("metric-name", "Cloudwatch metric name").Default("redis-queue-size").OverrideDefaultFromEnvar("CLOUDWATCH_METRIC_NAME").Short('m').String()
	metric_namespace       = kingpin.Flag("metric-namespace", "Cloudwatch metric namespace.").Default("Tropo Logstash ASG").OverrideDefaultFromEnvar("CLOUDWATCH_NAMESPACE").Short('n').String()
	aws_region             = kingpin.Flag("region", "AWS Region").Default("us-east-1").String()
	redis_list_name        = kingpin.Flag("redis-db", "Redis db").Short('r').Default("logstash").String()
	redis_servers          = kingpin.Flag("redis-servers", "Redis server.").Short('s').Strings()
	redis_password         = kingpin.Flag("redis-password", "password for redis instance").Short('P').String()
	aws_credential_file    = kingpin.Flag("aws-credential-file", "aws credential file, can be used in place of ENV variables or IAM profile").Short('f').String()
	aws_credential_profile = kingpin.Flag("aws-credential-profile", "aws credential profile").Short('p').String()

	Password    string
	get_version = kingpin.Flag("version", "get version").Short('V').Bool()

	RedisCloudWatch = &App{}
	auth            *aws.Config
	cred            *credentials.Credentials
)

func init() {
	kingpin.Parse()

	if *get_version {
		fmt.Printf("Version: %s\n", version)
		os.Exit(0)
	}

	RedisCloudWatch.TotalCount = 0

	cred = credentials.NewChainCredentials(
		[]credentials.Provider{
			&credentials.EnvProvider{},
			&credentials.SharedCredentialsProvider{Filename: *aws_credential_file, Profile: *aws_credential_profile},
			&credentials.EC2RoleProvider{},
		})
	auth = &aws.Config{Region: *aws_region, Credentials: cred}

	logrus.SetOutput(os.Stderr)
	if *verbose {
		auth.LogLevel = 1
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}

	if len(*redis_servers) == 0 {
		logrus.Fatal("You must specify at least one redis server")
	}
}

func main() {
	logrus.Debug("Starting application")
	for _, v := range *redis_servers {
		redis_instance := &RedisInstance{
			Name: v,
			Connection: redis.Client{
				Addr: v,
			},
		}

		// if password provided we'll use it
		if *redis_password != "" {
			redis_instance.Connection.Password = *redis_password
		}

		RedisCloudWatch.Instances = append(RedisCloudWatch.Instances, redis_instance)
	}

	for _, r := range RedisCloudWatch.Instances {
		logrus.Debugf("Looking queue length from %s", r.Name)
		ls_length, err := r.Connection.Llen(*redis_list_name)
		if err != nil {
			logrus.Fatal(err)
		}

		RedisCloudWatch.TotalCount = RedisCloudWatch.TotalCount + ls_length
	}

	logrus.WithFields(logrus.Fields{
		"redis_total": RedisCloudWatch.TotalCount,
	}).Info("counters")

	if *to_cloudwatch {
		SendCloudWatchMetric(float64(RedisCloudWatch.TotalCount))
	}

	logrus.Debug("Done")
}

func SendCloudWatchMetric(count float64) {

	svc := cloudwatch.New(auth)

	params := &cloudwatch.PutMetricDataInput{
		MetricData: []*cloudwatch.MetricDatum{
			&cloudwatch.MetricDatum{
				MetricName: aws.String(*metric_name),
				Timestamp:  aws.Time(time.Now()),
				Unit:       aws.String("Count"),
				Value:      aws.Double(count),
			},
		},
		Namespace: aws.String(*metric_namespace),
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
