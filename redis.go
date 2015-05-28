package main

import (
	logrus "github.com/Sirupsen/logrus"
	"gopkg.in/redis.v3"
)

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
