package config

import (
	"log"
	"os"
	"strconv"
	"sync"
)

var once sync.Once
var conf Config

type Config struct {
	RunPodAPIKey            string
	RedisHost               string
	RedisPort               string
	RedisPassword           string
	CotelligenceRwaEndpoint string
	DbDsn                   string
	DBConns                 int
	DBConnsIdle             int
}

func GetConfig() Config {
	once.Do(func() {
		dbConns, _ := strconv.Atoi(os.Getenv("DB_CONNS"))
		dbConnsIdle, _ := strconv.Atoi(os.Getenv("DB_CONNS_IDLE"))
		conf = Config{
			RunPodAPIKey:            os.Getenv("RUNPOD_API_KEY"),
			RedisHost:               os.Getenv("REDIS_HOST"),
			RedisPort:               os.Getenv("REDIS_PORT"),
			RedisPassword:           os.Getenv("REDIS_PASSWORD"),
			CotelligenceRwaEndpoint: os.Getenv("COTELLIGENCE_RWA_ENDPOINT"),
			DbDsn:                   os.Getenv("DB_DSN"),
			DBConns:                 dbConns,
			DBConnsIdle:             dbConnsIdle,
		}

		if conf.RunPodAPIKey == "" {
			log.Fatal("RUNPOD_API_KEY must be set in .env")
		}
		if conf.RedisHost == "" {
			log.Fatal("REDIS_HOST must be set in .env")
		}
		if conf.RedisPort == "" {
			log.Fatal("REDIS_PORT must be set in .env")
		}
		if conf.DbDsn == "" {
			log.Fatal("DB_DSN must be set in .env")
		}
	})
	return conf
}
