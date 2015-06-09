package main

import (
	"fmt"
	logrus "github.com/Sirupsen/logrus"
	"github.com/alecthomas/kingpin"
	"github.com/awslabs/aws-sdk-go/aws"
	"github.com/awslabs/aws-sdk-go/aws/credentials"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	toCloudwatch         = kingpin.Flag("aws-cloudwatch", "Send metrics to cloud watch").Short('c').Bool()
	useIam               = kingpin.Flag("aws-iam-profile", "Use AWSIAM Profile for authentication").Short('i').Bool()
	awsRegion            = kingpin.Flag("aws-region", "AWS Region").Short('R').Default("us-east-1").String()
	awsCredentialFile    = kingpin.Flag("aws-credential-file", "aws credential file, can be used in place of ENV variables or IAM profile").String()
	awsCredentialProfile = kingpin.Flag("aws-credential-profile", "aws credential profile").String()
	applicationLockFile  = kingpin.Flag("lock-file", "lock file, insures only a single instance is running").Default("/tmp/redis-cloudwatch.lock").String()
	metricName           = kingpin.Flag("metric-name", "Cloudwatch metric name").Default("redis-queue-size").OverrideDefaultFromEnvar("CLOUDWATCH_METRIC_NAME").Short('m').String()
	metricNamespace      = kingpin.Flag("metric-namespace", "Cloudwatch metric namespace.").Default("Logstash ASG").OverrideDefaultFromEnvar("CLOUDWATCH_NAMESPACE").Short('n').String()
	redisListName        = kingpin.Flag("redis-list", "Redis list name").Short('l').Default("logstash").String()
	redisServers         = kingpin.Flag("redis-server", "Redis server URI").Short('r').Strings()
	redisDatabase        = kingpin.Flag("redis-db", "Redis db name.").Short('d').Int()
	sleepTime            = kingpin.Flag("sleep-time", "How long to sleep between each check.  Defaults to 0 which checks once and exits.").Short('s').Duration()
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
	lock := checkLockFile()
	defer lock.Unlock()

	// startup log
	logrus.WithFields(startUpLoggingFields()).Info("Staring application")

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)
	go func() {
		<-c
		cleanup()
		os.Exit(0)
	}()

	// loop
	for {

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

		// If there is no sleeptime set then we break
		if *sleepTime == 0 {
			break
		} else {
			logrus.Debugf("Sleeping for for %s", *sleepTime)
			time.Sleep(*sleepTime)
		}

	}

}
