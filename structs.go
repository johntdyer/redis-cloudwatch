package main

import "gopkg.in/redis.v3"

type redisInstance struct {
	Name       string
	Connection *redis.Client
}

type redisCloudWatchMonitor struct {
	Instances  []*redisInstance
	TotalCount int64
}
