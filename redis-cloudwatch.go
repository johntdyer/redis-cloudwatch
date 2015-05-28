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

type redisInstance struct {
	Name       string
	Connection *redis.Client
}

type redisCloudWatchMonitor struct {
	Instances  []*redisInstance
	TotalCount int64
}

const (
	version = "0.0.1"
)

var (
	toCloudwatch         = kingpin.Flag("aws-cloudwatch", "Send metrics to cloud watch").Short('c').Bool()
	useIam               = kingpin.Flag("aws-iam-profile", "Use AWSIAM Profile for authentication").Short('i').Bool()
	awsRegion            = kingpin.Flag("aws-region", "AWS Region").Short('R').Default("us-east-1").String()
	awsCredentialFile    = kingpin.Flag("aws-credential-file", "aws credential file, can be used in place of ENV variables or IAM profile").String()
	awsCredentialProfile = kingpin.Flag("aws-credential-profile", "aws credential profile").String()
	metricName           = kingpin.Flag("metric-name", "Cloudwatch metric name").Default("redis-queue-size").OverrideDefaultFromEnvar("CLOUDWATCH_METRIC_NAME").Short('m').String()
	metricNamespace      = kingpin.Flag("metric-namespace", "Cloudwatch metric namespace.").Default("Tropo Logstash ASG").OverrideDefaultFromEnvar("CLOUDWATCH_NAMESPACE").Short('n').String()
	redisListName        = kingpin.Flag("redis-list", "Redis list name").Short('l').Default("logstash").String()
	redisServers         = kingpin.Flag("redis-server", "Redis server URI").Short('r').Strings()
	redisDatabase        = kingpin.Flag("redis-db", "Redis db name.").Short('d').Int()
	redisPassword        = kingpin.Flag("redis-password", "password for redis instance").Short('p').String()
	verbose              = kingpin.Flag("verbose", "Verbose mode.").Short('v').Bool()
	getVersion           = kingpin.Flag("version", "get version").Short('V').Bool()
	auth                 *aws.Config
	cred                 *credentials.Credentials
)

func init() {
	kingpin.Parse()

	if *getVersion {
		fmt.Printf("Version: %s\n", version)
		os.Exit(0)
	}

	// RedisPassword
	cred = credentials.NewChainCredentials(
		[]credentials.Provider{
			&credentials.EnvProvider{},
			&credentials.SharedCredentialsProvider{Filename: *awsCredentialFile, Profile: *awsCredentialProfile},
			&credentials.EC2RoleProvider{},
		})
	auth = &aws.Config{Region: *awsRegion, Credentials: cred}

	logrus.SetOutput(os.Stderr)
	if *verbose {
		auth.LogLevel = 1
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}

	if len(*redisServers) == 0 {
		logrus.Fatal("You must specify at least one redis server")
	}
}

func main() {
	logrus.Debug("Starting application")

	app := &redisCloudWatchMonitor{
		TotalCount: 0,
	}

	app.Instances = getRedisWatcherInstances()

	getTotalRedisSetLength(app)

	logrus.WithFields(logrus.Fields{
		"redis_total": app.TotalCount,
	}).Info("counters")

	if *toCloudwatch {
		sendCloudWatchMetric(float64(app.TotalCount))
	}

	logrus.Debug("Done")
}

// GetTotalRedisSetLength - Iterate over each redis instance and sum the set size for cloud watch
func getTotalRedisSetLength(ri *redisCloudWatchMonitor) {

	for _, r := range ri.Instances {
		logrus.Debugf("Looking queue length from %s", r.Name)
		lsLength, err := r.Connection.LLen(*redisListName).Result()
		if err != nil {
			logrus.Fatal(err)
		}
		logrus.Debugf("length of set '%s' was %d on '%s'", *redisListName, lsLength, r.Name)
		ri.TotalCount = ri.TotalCount + lsLength
	}
}

// GetRedisWatcherInstances - Initilize our structs with passed in redis URL's
func getRedisWatcherInstances() []*redisInstance {
	var instances []*redisInstance

	for _, v := range *redisServers {

		redisInstance := &redisInstance{
			Name: v,
			Connection: redis.NewClient(&redis.Options{
				Addr:     v,
				Password: *redisPassword,
				DB:       int64(*redisDatabase),
			}),
		}

		_, err := redisInstance.Connection.Ping().Result()
		if err != nil {
			if err.Error() == "NOAUTH Authentication required." {
				logrus.Fatal("Password required")
			} else {
				logrus.Fatal(err)
			}
		}

		instances = append(instances, redisInstance)
	}
	return instances
}

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
