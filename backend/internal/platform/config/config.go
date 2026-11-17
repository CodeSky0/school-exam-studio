package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	DatabaseURL  string
	RedisURL     string
	OCRURL       string
	JWTSecret    string
	CORSOrigin   string
	HTTPAddr     string
	SecureCookie bool
	TrustProxy   bool
	AutoMigrate  bool
	AIBaseURL    string
	AIAPIKey     string
	AIModel      string
	AITimeout    int
	AIEncryptKey string
	AIUploadDir  string
}

func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		RedisURL:     os.Getenv("REDIS_URL"),
		OCRURL:       os.Getenv("OCR_URL"),
		JWTSecret:    os.Getenv("JWT_SECRET"),
		CORSOrigin:   envOr("CORS_ORIGIN", "http://localhost:3000"),
		HTTPAddr:     envOr("HTTP_ADDR", ":8080"),
		SecureCookie: os.Getenv("APP_ENV") == "production",
		TrustProxy:   envBool("TRUST_PROXY"),
		AutoMigrate:  envBoolDefault("AUTO_MIGRATE", true),
		AIBaseURL:    strings.TrimSpace(os.Getenv("AI_BASE_URL")),
		AIAPIKey:     strings.TrimSpace(os.Getenv("AI_API_KEY")),
		AIModel:      envOr("AI_MODEL", "gpt-4o-mini"),
		AITimeout:    envInt("AI_TIMEOUT_SECONDS", 45),
		AIEncryptKey: strings.TrimSpace(os.Getenv("AI_SETTINGS_SECRET")),
		AIUploadDir:  envOr("AI_UPLOAD_DIR", "./data/ai-uploads"),
	}
	if cfg.AIEncryptKey == "" {
		cfg.AIEncryptKey = cfg.JWTSecret
	}
	if cfg.AITimeout < 5 || cfg.AITimeout > 180 {
		cfg.AITimeout = 45
	}
	if cfg.DatabaseURL == "" || cfg.RedisURL == "" || cfg.OCRURL == "" {
		return Config{}, errors.New("DATABASE_URL, REDIS_URL and OCR_URL are required")
	}
	if len(cfg.JWTSecret) < 32 {
		return Config{}, errors.New("JWT_SECRET must contain at least 32 bytes")
	}
	return cfg, nil
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envBool(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}

func envBoolDefault(key string, fallback bool) bool {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes":
		return true
	case "0", "false", "no":
		return false
	default:
		return fallback
	}
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(key)))
	if err != nil {
		return fallback
	}
	return value
}
