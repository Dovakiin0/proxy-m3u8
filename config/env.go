package config

import (
	"os"
)

type envConfig struct {
	Port           string
	CorsDomain     string
	RedisURL       string
	RedisPassword  string
	DefaultReferer string
}

var Env envConfig

func getEnv(varName, defaultValue string) string {
	value, exists := os.LookupEnv(varName)
	if !exists {
		return defaultValue
	}
	return value
}

func InitConfig() {
	Env = envConfig{
		Port:           getEnv("PORT", "8080"),
		CorsDomain:     getEnv("CORS_DOMAIN", "*"),
		RedisURL:       getEnv("REDIS_URL", ""),
		RedisPassword:  getEnv("REDIS_PASSWORD", ""),
		DefaultReferer: getEnv("DEFAULT_REFERER", ""),
	}
}
