package httpserver

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"school-exam-studio/backend/ent"
	"school-exam-studio/backend/internal/admin"
	"school-exam-studio/backend/internal/auth"
	"school-exam-studio/backend/internal/importer"
	"school-exam-studio/backend/internal/ocr"
	"school-exam-studio/backend/internal/paper"
	"school-exam-studio/backend/internal/platform/apperror"
	platformmiddleware "school-exam-studio/backend/internal/platform/middleware"
	"school-exam-studio/backend/internal/question"
)

type Options struct {
	AuthService     *auth.Service
	QuestionService *question.Service
	PaperService    *paper.Service
	AdminService    *admin.Service
	ImportService   *importer.Service
	OCRClient       *ocr.Client
	CORSOrigin      string
	SecureCookie    bool
	TrustProxy      bool
	LoginLimiter    platformmiddleware.RateLimiter
	RegisterLimiter platformmiddleware.RateLimiter
}

func New(client *ent.Client, options ...Options) *echo.Echo {
	configuration := Options{CORSOrigin: "http://localhost:3000"}
	if len(options) > 0 {
		configuration = options[0]
	}
	if strings.Contains(configuration.CORSOrigin, "*") {
		configuration.CORSOrigin = ""
	}
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.HTTPErrorHandler = apperror.HTTPErrorHandler
	e.Use(echomiddleware.RequestID())
	e.Use(echomiddleware.Recover())
	// JSON handlers apply stricter limits. Multipart OCR permits headers in addition to its 5 MiB image.
	e.Use(echomiddleware.BodyLimit("6M"))
	e.Use(echomiddleware.CORSWithConfig(echomiddleware.CORSConfig{
		AllowOrigins:     []string{configuration.CORSOrigin},
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderXRequestID},
		ExposeHeaders:    []string{echo.HeaderXRequestID},
		AllowCredentials: true,
		MaxAge:           600,
	}))
	e.Use(platformmiddleware.OriginProtection(configuration.CORSOrigin))
	e.GET("/healthz", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})
	e.GET("/readyz", func(c echo.Context) error {
		ctx, cancel := context.WithTimeout(c.Request().Context(), 2*time.Second)
		defer cancel()
		if _, err := client.School.Query().Count(ctx); err != nil {
			return apperror.New(http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "服务暂不可用")
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ready"})
	})
	if configuration.AuthService != nil {
		registerAuthRoutes(e, configuration)
	}
	return e
}

func registerAuthRoutes(e *echo.Echo, configuration Options) {
	handler := auth.NewHandler(configuration.AuthService, configuration.SecureCookie)
	api := e.Group("/api/v1")
	api.GET("/init/status", handler.InitializationStatus)
	api.POST("/init", handler.Initialize)
	api.POST("/auth/register", handler.Register, platformmiddleware.RegisterRateLimit(configuration.RegisterLimiter, configuration.TrustProxy))
	api.POST("/auth/login", handler.Login, platformmiddleware.LoginRateLimit(configuration.LoginLimiter, configuration.TrustProxy))
	api.POST("/auth/logout", handler.Logout)
	authenticated := api.Group("", platformmiddleware.Authenticate(
		configuration.AuthService.Tokens(),
		configuration.AuthService,
	))
	authenticated.GET("/auth/me", handler.Me)
	authenticated.POST("/auth/change-password", handler.ChangePassword)
	if configuration.ImportService != nil {
		importer.NewHandler(configuration.ImportService).Register(authenticated)
	}
	if configuration.AdminService != nil {
		admin.NewHandler(configuration.AdminService).Register(authenticated)
	}
	if configuration.QuestionService != nil {
		question.NewHandler(configuration.QuestionService).Register(authenticated)
	}
	if configuration.PaperService != nil {
		paper.NewHandler(configuration.PaperService).Register(authenticated)
	}
	if configuration.OCRClient != nil {
		ocr.NewHandler(configuration.OCRClient).Register(authenticated)
	}
}
