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
	if !cfg.AutoMigrate {
		t.Fatalf("AUTO_MIGRATE should default to true: %+v", cfg)
	}
}

func TestLoadAutoMigrateCanBeDisabled(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:password@localhost:5432/school_db?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("OCR_URL", "http://localhost:8001")
	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")
	t.Setenv("AUTO_MIGRATE", "false")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AutoMigrate {
		t.Fatalf("AUTO_MIGRATE=false was not honored: %+v", cfg)
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

func TestLoadRateLimitDefaultsAndOverrides(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:password@localhost:5432/school_db?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LoginRateLimit != 10 || cfg.RegisterRateLimit != 5 {
		t.Fatalf("unexpected rate limit defaults: %+v", cfg)
	}
	t.Setenv("RATE_LIMIT_LOGIN", "500")
	t.Setenv("RATE_LIMIT_REGISTER", "500")
	cfg, err = config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LoginRateLimit != 500 || cfg.RegisterRateLimit != 500 {
		t.Fatalf("rate limit overrides were not honored: %+v", cfg)
	}
	t.Setenv("RATE_LIMIT_REGISTER", "0")
	if _, err := config.Load(); err == nil || !strings.Contains(err.Error(), "RATE_LIMIT_REGISTER") {
		t.Fatalf("expected RATE_LIMIT_REGISTER error, got %v", err)
	}
}
