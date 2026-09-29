package main

import "testing"

func TestLoadConfigDefaults(t *testing.T) {
	for _, key := range []string{"APP_NAME", "APP_VERSION", "PORT", "AWS_REGION", "S3_BUCKET"} {
		t.Setenv(key, "")
	}

	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AppName != "cloud-seminar-demo" || cfg.AppVersion != "1.0.0" || cfg.Port != "8080" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.AWSRegion != "us-west-2" || cfg.S3Bucket != "prasad-cloud-demo" {
		t.Fatalf("unexpected AWS defaults: %+v", cfg)
	}
}

func TestLoadConfigOverrides(t *testing.T) {
	t.Setenv("APP_NAME", "seminar")
	t.Setenv("APP_VERSION", "1.1.0")
	t.Setenv("PORT", "9090")
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("S3_BUCKET", "other-bucket")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AppName != "seminar" || cfg.AppVersion != "1.1.0" || cfg.Port != "9090" {
		t.Fatalf("override failed: %+v", cfg)
	}
	if cfg.AWSRegion != "us-east-1" || cfg.S3Bucket != "other-bucket" {
		t.Fatalf("AWS override failed: %+v", cfg)
	}
}

func TestLoadConfigRejectsBadPort(t *testing.T) {
	for _, port := range []string{"nope", "0", "70000"} {
		t.Setenv("PORT", port)
		if _, err := loadConfig(); err == nil {
			t.Fatalf("PORT %q was accepted", port)
		}
	}
}
