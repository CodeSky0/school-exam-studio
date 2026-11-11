package config

import (
	"errors"
	"os"
)

type Config struct {
	DatabaseURL  string
	RedisURL     string
	OCRURL       string
	JWTSecret    string
	CORSOrigin   string
	HTTPAddr     string
	SecureCookie bool
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
