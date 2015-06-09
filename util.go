package main

import (
	logrus "github.com/Sirupsen/logrus"
	"github.com/nightlyone/lockfile"
	"strconv"
)

// Check to ensure only a single instance of process is running
func checkLockFile() lockfile.Lockfile {
	lock, err := lockfile.New(*applicationLockFile)
	if err != nil {
		logrus.Fatalf("Cannot init lock. reason: %v", err)
	}
	err = lock.TryLock()
	// Error handling is essential, as we only try to get the lock.
	if err != nil {
		logrus.Fatalf("Cannot lock \"%v\", reason: %v", lock, err)
	}
	return lock
}

// Signal catcher cleanup, not much here right now
func cleanup() {
	// Any other cleanup logic would go here
	logrus.Info("Done")
}

// Creates fields for start log message
func startUpLoggingFields() logrus.Fields {
	f := logrus.Fields{
		"use_iam":               *useIam,
		"sleep_time":            *sleepTime,
		"aws_region":            *awsRegion,
		"metric_name":           *metricName,
		"to_cloudwatch":         *toCloudwatch,
		"redis_servers":         *redisServers,
		"version":               version,
		"verbose_logging":       *verbose,
		"redis_list_name":       *redisListName,
		"metric_namespace":      *metricNamespace,
		"application_lock_file": *applicationLockFile,
	}

	if *redisDatabase != 0 {
		f["redis_database"] = strconv.Itoa(*redisDatabase)
	}

	if *awsCredentialFile != "" {
		f["aws_credentialfile"] = *awsCredentialFile
	}

	if *awsCredentialProfile != "" {
		f["aws_credential_profile"] = *awsCredentialProfile
	}

	return f
}
