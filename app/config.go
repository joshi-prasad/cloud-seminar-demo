package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config is non-secret runtime configuration. Credentials are never part of
// this struct; the AWS SDK loads them through its default provider chain.
type Config struct {
	AppName    string
	AppVersion string
	Port       string
	AWSRegion  string
	S3Bucket   string
}

func loadConfig() (Config, error) {
	cfg := Config{
		AppName:    envOr("APP_NAME", "cloud-seminar-demo"),
		AppVersion: envOr("APP_VERSION", "1.0.0"),
		Port:       envOr("PORT", "8080"),
		AWSRegion:  envOr("AWS_REGION", "us-west-2"),
		S3Bucket:   envOr("S3_BUCKET", "prasad-cloud-demo"),
	}
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func envOr(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func (c Config) validate() error {
	if c.AppName == "" {
		return errors.New("APP_NAME is empty")
	}
	if c.AppVersion == "" {
		return errors.New("APP_VERSION is empty")
	}
	if c.AWSRegion == "" {
		return errors.New("AWS_REGION is empty")
	}
	if c.S3Bucket == "" {
		return errors.New("S3_BUCKET is empty")
	}
	port, err := strconv.Atoi(c.Port)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("PORT must be a number from 1 to 65535, got %q", c.Port)
	}
	return nil
}
