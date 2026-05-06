package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	AppEnv            string
	HTTPAddr          string
	DatabaseURL       string
	S3Endpoint        string
	S3Bucket          string
	S3Region          string
	S3AccessKeyID     string
	S3SecretAccessKey string
	S3ForcePathStyle  bool
}

func Load() (Config, error) {
	appEnv := getEnv("APP_ENV", "development")
	forcePathStyle, err := getEnvBool("S3_FORCE_PATH_STYLE", true)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		AppEnv:            appEnv,
		HTTPAddr:          getEnv("HTTP_ADDR", ":8080"),
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		S3Endpoint:        getEnv("S3_ENDPOINT", "http://localhost:8333"),
		S3Bucket:          getEnv("S3_BUCKET", "snaelda-local"),
		S3Region:          getEnv("S3_REGION", "us-east-1"),
		S3AccessKeyID:     getEnv("S3_ACCESS_KEY_ID", "snaelda"),
		S3SecretAccessKey: getEnv("S3_SECRET_ACCESS_KEY", "snaelda-secret"),
		S3ForcePathStyle:  forcePathStyle,
	}

	if cfg.AppEnv == "" {
		return Config{}, fmt.Errorf("APP_ENV is required")
	}
	if cfg.HTTPAddr == "" {
		return Config{}, fmt.Errorf("HTTP_ADDR is required")
	}
	if cfg.AppEnv != "test" && cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	return cfg, nil
}

func getEnv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getEnvBool(key string, fallback bool) (bool, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean: %w", key, err)
	}

	return parsed, nil
}
