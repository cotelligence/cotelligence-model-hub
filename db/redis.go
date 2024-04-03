package db

import (
	"cotelligence-model-hub/config"
	"sync"

	"github.com/go-redis/redis/v8"
)

var (
	initRedisOnce sync.Once
	redisClient   *redis.Client
)

func GetRedisClient() *redis.Client {
	conf := config.GetConfig()
	initRedisOnce.Do(func() {
		redisAddr := conf.RedisHost + ":" + conf.RedisPort
		redisClient = redis.NewClient(&redis.Options{
			Addr:     redisAddr,
			Password: conf.RedisPassword,
			DB:       0, // Default DB
		})
	})
	return redisClient
}
