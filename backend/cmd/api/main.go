package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"school-exam-studio/backend/internal/admin"
	"school-exam-studio/backend/internal/ai"
	"school-exam-studio/backend/internal/auth"
	"school-exam-studio/backend/internal/drive"
	"school-exam-studio/backend/internal/embedding"
	"school-exam-studio/backend/internal/importer"
	"school-exam-studio/backend/internal/ocr"
	"school-exam-studio/backend/internal/paper"
	"school-exam-studio/backend/internal/platform/config"
	"school-exam-studio/backend/internal/platform/database"
	"school-exam-studio/backend/internal/platform/httpserver"
	platformmiddleware "school-exam-studio/backend/internal/platform/middleware"
	"school-exam-studio/backend/internal/platform/migrator"
	"school-exam-studio/backend/internal/question"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	if cfg.AutoMigrate {
		if err := migrator.Up(cfg.DatabaseURL); err != nil {
			log.Fatalf("run database migrations: %v", err)
		}
		log.Print("database migrations applied")
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
	// OCR sidecar 是可选的独立能力；AI 工作台的图片阅读由多模态模型完成，不依赖 OCR。
	var ocrClient *ocr.Client
	if strings.TrimSpace(cfg.OCRURL) != "" {
		ocrClient, err = ocr.NewClient(cfg.OCRURL, ocr.DefaultTimeout, ocr.DefaultMaxConcurrent)
		if err != nil {
			log.Fatal(err)
		}
	}
	cloudStore, err := drive.NewFileStore(cfg.CloudUploadDir)
	if err != nil {
		log.Fatal(err)
	}
	embedder, err := embedding.New(
		cfg.EmbeddingBaseURL,
		cfg.EmbeddingAPIKey,
		cfg.EmbeddingModel,
		cfg.EmbeddingLocalDimensions,
	)
	if err != nil {
		log.Fatal(err)
	}
	cloudRepository := drive.NewRepository(client)
	cloudIndexer := drive.NewIndexer(cloudRepository, cloudStore, embedder, log.Default())
	cloudService := drive.NewService(cloudRepository, cloudStore)
	cloudService.SetIndexer(cloudIndexer)
	indexContext, indexCancel := context.WithCancel(context.Background())
	defer indexCancel()
	cloudIndexer.Start(indexContext)
	log.Printf("cloud index provider: %s", cloudIndexer.Provider())
	aiService := ai.NewService(ai.Options{
		QuestionService: questionService,
		PaperService:    paperService,
		Repository:      ai.NewRepository(client),
		LLMBaseURL:      cfg.AIBaseURL,
		LLMAPIKey:       cfg.AIAPIKey,
		LLMModel:        cfg.AIModel,
		LLMTimeout:      time.Duration(cfg.AITimeout) * time.Second,
		SettingsSecret:  cfg.AIEncryptKey,
		UploadDir:       cfg.AIUploadDir,
		// 云盘与 AI 附件双向联动：本地附件自动入云盘，云盘文件可加入对话。
		CloudDrive: cloudService,
	})
	e := httpserver.New(client, httpserver.Options{
		AuthService:     authService,
		QuestionService: questionService,
		PaperService:    paperService,
		AdminService:    adminService,
		ImportService:   importService,
		AIService:       aiService,
		CloudService:    cloudService,
		OCRClient:       ocrClient,
		CORSOrigin:      cfg.CORSOrigin,
		SecureCookie:    cfg.SecureCookie,
		TrustProxy:      cfg.TrustProxy,
		LoginLimiter: platformmiddleware.NewRedisRateLimiter(
			redisClient, "login", cfg.LoginRateLimit, 15*time.Minute,
		),
		RegisterLimiter: platformmiddleware.NewRedisRateLimiter(
			redisClient, "register", cfg.RegisterRateLimit, 15*time.Minute,
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
