package config_test

import (
	"strings"
	"testing"

	"school-exam-studio/backend/internal/platform/config"
)

func TestLoadRequiresStrongJWTSecret(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:password@localhost:5432/school_db?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("OCR_URL", "http://localhost:8001")
	t.Setenv("JWT_SECRET", "short")
	_, err := config.Load()
	if err == nil || !strings.Contains(err.Error(), "JWT_SECRET") {
		t.Fatalf("expected JWT_SECRET error, got %v", err)
	}
}

func TestLoadValidConfig(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:password@localhost:5432/school_db?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("OCR_URL", "http://localhost:8001")
	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != ":8080" || cfg.CORSOrigin != "http://localhost:3000" || cfg.SecureCookie || cfg.TrustProxy {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestLoadTrustProxy(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:password@localhost:5432/school_db?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("OCR_URL", "http://localhost:8001")
	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")
	t.Setenv("TRUST_PROXY", "true")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.TrustProxy {
		t.Fatalf("TRUST_PROXY was not enabled: %+v", cfg)
	}
}
