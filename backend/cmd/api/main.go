package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"school-exam-studio/backend/internal/admin"
	"school-exam-studio/backend/internal/ai"
	"school-exam-studio/backend/internal/auth"
	"school-exam-studio/backend/internal/importer"
	"school-exam-studio/backend/internal/ocr"
	"school-exam-studio/backend/internal/paper"
	"school-exam-studio/backend/internal/platform/config"
	"school-exam-studio/backend/internal/platform/database"
	"school-exam-studio/backend/internal/platform/httpserver"
	platformmiddleware "school-exam-studio/backend/internal/platform/middleware"
	"school-exam-studio/backend/internal/question"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	client, err := database.Open(ctx, cfg.DatabaseURL)
	cancel()
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()
	redisOptions, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Fatal(err)
	}
	redisClient := redis.NewClient(redisOptions)
	defer redisClient.Close()
	redisContext, redisCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := redisClient.Ping(redisContext).Err(); err != nil {
		redisCancel()
		log.Fatal(err)
	}
	redisCancel()
	repository := auth.NewRepository(client)
	passwordHasher := auth.NewPasswordHasher(12)
	authService := auth.NewService(repository, passwordHasher, cfg.JWTSecret, 2*time.Hour)
	sanitizer := question.NewSanitizer()
	questionService := question.NewService(question.NewRepository(client), sanitizer)
	paperService := paper.NewService(paper.NewRepository(client))
	adminService := admin.NewService(admin.NewRepository(client), passwordHasher)
	importService := importer.NewService(importer.NewRepository(client), sanitizer)
	aiService := ai.NewService(ai.Options{
		QuestionService: questionService,
		PaperService:    paperService,
		Repository:      ai.NewRepository(client),
		LLMBaseURL:      cfg.AIBaseURL,
		LLMAPIKey:       cfg.AIAPIKey,
		LLMModel:        cfg.AIModel,
		LLMTimeout:      time.Duration(cfg.AITimeout) * time.Second,
		SettingsSecret:  cfg.AIEncryptKey,
	})
	ocrClient, err := ocr.NewClient(cfg.OCRURL, ocr.DefaultTimeout, ocr.DefaultMaxConcurrent)
	if err != nil {
		log.Fatal(err)
	}
	e := httpserver.New(client, httpserver.Options{
		AuthService:     authService,
		QuestionService: questionService,
		PaperService:    paperService,
		AdminService:    adminService,
		ImportService:   importService,
		AIService:       aiService,
		OCRClient:       ocrClient,
		CORSOrigin:      cfg.CORSOrigin,
		SecureCookie:    cfg.SecureCookie,
		TrustProxy:      cfg.TrustProxy,
		LoginLimiter: platformmiddleware.NewRedisRateLimiter(
			redisClient, "login", 10, 15*time.Minute,
		),
		RegisterLimiter: platformmiddleware.NewRedisRateLimiter(
			redisClient, "register", 5, 15*time.Minute,
		),
	})
	errCh := make(chan error, 1)
	go func() {
		errCh <- e.Start(cfg.HTTPAddr)
	}()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	case <-sigCh:
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := e.Shutdown(shutdownCtx); err != nil {
			log.Fatal(err)
		}
	}
}
