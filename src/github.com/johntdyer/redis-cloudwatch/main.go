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
	"gopkg.in/redis.v3"
	"os"
	"time"
)

type RedisInstance struct {
	Name       string
	Connection *redis.Client
}

type RedisCloudWatchMonitor struct {
	Instances  []*RedisInstance
	TotalCount int64
}

const (
	version = "0.0.1"
)

var (
	to_cloudwatch          = kingpin.Flag("aws-cloudwatch", "Send metrics to cloud watch").Short('c').Bool()
	use_iam                = kingpin.Flag("aws-iam-profile", "Use AWSIAM Profile for authentication").Short('i').Bool()
	aws_region             = kingpin.Flag("aws-region", "AWS Region").Short('R').Default("us-east-1").String()
	aws_credential_file    = kingpin.Flag("aws-credential-file", "aws credential file, can be used in place of ENV variables or IAM profile").String()
	aws_credential_profile = kingpin.Flag("aws-credential-profile", "aws credential profile").String()
	metric_name            = kingpin.Flag("metric-name", "Cloudwatch metric name").Default("redis-queue-size").OverrideDefaultFromEnvar("CLOUDWATCH_METRIC_NAME").Short('m').String()
	metric_namespace       = kingpin.Flag("metric-namespace", "Cloudwatch metric namespace.").Default("Tropo Logstash ASG").OverrideDefaultFromEnvar("CLOUDWATCH_NAMESPACE").Short('n').String()
	redis_list_name        = kingpin.Flag("redis-list", "Redis list name").Short('l').Default("logstash").String()
	redis_servers          = kingpin.Flag("redis-server", "Redis server URI").Short('r').Strings()
	redis_database         = kingpin.Flag("redis-db", "Redis db name.").Short('d').Int()
	redis_password         = kingpin.Flag("redis-password", "password for redis instance").Short('p').String()
	verbose                = kingpin.Flag("verbose", "Verbose mode.").Short('v').Bool()
	get_version            = kingpin.Flag("version", "get version").Short('V').Bool()
	auth                   *aws.Config
	cred                   *credentials.Credentials
)

func init() {
	kingpin.Parse()

	if *get_version {
		fmt.Printf("Version: %s\n", version)
		os.Exit(0)
	}

	// RedisPassword
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

	app := &RedisCloudWatchMonitor{
		TotalCount: 0,
	}

	app.Instances = GetRedisWatcherInstances()

	GetTotalRedisSetLength(app)

	logrus.WithFields(logrus.Fields{
		"redis_total": app.TotalCount,
	}).Info("counters")

	if *to_cloudwatch {
		SendCloudWatchMetric(float64(app.TotalCount))
	}

	logrus.Debug("Done")
}

// Iterate over each redis instance and sum the set size for cloud watch
func GetTotalRedisSetLength(ri *RedisCloudWatchMonitor) {

	for _, r := range ri.Instances {
		logrus.Debugf("Looking queue length from %s", r.Name)
		ls_length, err := r.Connection.LLen(*redis_list_name).Result()
		if err != nil {
			logrus.Fatal(err)
		}
		logrus.Debugf("length of set '%s' was %d on '%s'", *redis_list_name, ls_length, r.Name)
		ri.TotalCount = ri.TotalCount + ls_length
	}
}

// Initilize our structs with passed in redis URL's
func GetRedisWatcherInstances() []*RedisInstance {
	var instances []*RedisInstance

	for _, v := range *redis_servers {

		redis_instance := &RedisInstance{
			Name: v,
			Connection: redis.NewClient(&redis.Options{
				Addr:     v,
				Password: *redis_password,
				DB:       int64(*redis_database),
			}),
		}

		_, err := redis_instance.Connection.Ping().Result()
		if err != nil {
			if err.Error() == "NOAUTH Authentication required." {
				logrus.Fatal("Password required")
			} else {
				logrus.Fatal(err)
			}
		}

		instances = append(instances, redis_instance)
	}
	return instances
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
