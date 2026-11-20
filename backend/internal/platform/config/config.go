package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	DatabaseURL    string
	RedisURL       string
	OCRURL         string
	JWTSecret      string
	CORSOrigin     string
	HTTPAddr       string
	SecureCookie   bool
	TrustProxy     bool
	AutoMigrate    bool
	AIBaseURL      string
	AIAPIKey       string
	AIModel        string
	AITimeout      int
	AIEncryptKey   string
	AIUploadDir    string
	CloudUploadDir string

	// 云盘资料向量化：留空远程地址与 Key 时使用本地哈希向量。
	EmbeddingBaseURL         string
	EmbeddingAPIKey          string
	EmbeddingModel           string
	EmbeddingLocalDimensions int
}

func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:              os.Getenv("DATABASE_URL"),
		RedisURL:                 os.Getenv("REDIS_URL"),
		OCRURL:                   os.Getenv("OCR_URL"),
		JWTSecret:                os.Getenv("JWT_SECRET"),
		CORSOrigin:               envOr("CORS_ORIGIN", "http://localhost:3000"),
		HTTPAddr:                 envOr("HTTP_ADDR", ":8080"),
		SecureCookie:             os.Getenv("APP_ENV") == "production",
		TrustProxy:               envBool("TRUST_PROXY"),
		AutoMigrate:              envBoolDefault("AUTO_MIGRATE", true),
		AIBaseURL:                strings.TrimSpace(os.Getenv("AI_BASE_URL")),
		AIAPIKey:                 strings.TrimSpace(os.Getenv("AI_API_KEY")),
		AIModel:                  envOr("AI_MODEL", "gpt-4o-mini"),
		AITimeout:                envInt("AI_TIMEOUT_SECONDS", 45),
		AIEncryptKey:             strings.TrimSpace(os.Getenv("AI_SETTINGS_SECRET")),
		AIUploadDir:              envOr("AI_UPLOAD_DIR", "./data/ai-uploads"),
		CloudUploadDir:           envOr("CLOUD_UPLOAD_DIR", "./data/cloud-drive"),
		EmbeddingBaseURL:         strings.TrimSpace(os.Getenv("EMBEDDING_BASE_URL")),
		EmbeddingAPIKey:          strings.TrimSpace(os.Getenv("EMBEDDING_API_KEY")),
		EmbeddingModel:           envOr("EMBEDDING_MODEL", "text-embedding-3-small"),
		EmbeddingLocalDimensions: envInt("EMBEDDING_LOCAL_DIMENSIONS", 256),
	}
	if cfg.AIEncryptKey == "" {
		cfg.AIEncryptKey = cfg.JWTSecret
	}
	if cfg.AITimeout < 5 || cfg.AITimeout > 180 {
		cfg.AITimeout = 45
	}
	if cfg.DatabaseURL == "" || cfg.RedisURL == "" {
		return Config{}, errors.New("DATABASE_URL and REDIS_URL are required")
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
