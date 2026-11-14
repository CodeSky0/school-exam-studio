package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"school-exam-studio/backend/internal/platform/apperror"
	platformmiddleware "school-exam-studio/backend/internal/platform/middleware"
)

type fakeLimiter struct {
	allowed     bool
	retryAfter  time.Duration
	lastContext context.Context
	lastKey     string
}

func (l *fakeLimiter) Allow(ctx context.Context, key string) (bool, time.Duration, error) {
	l.lastContext = ctx
	l.lastKey = key
	return l.allowed, l.retryAfter, nil
}

func TestLoginRateLimitRestoresBodyAndBuildsAccountKey(t *testing.T) {
	limiter := &fakeLimiter{allowed: true}
	e := echo.New()
	e.HTTPErrorHandler = apperror.HTTPErrorHandler
	e.POST("/login", func(c echo.Context) error {
		var body map[string]string
		if err := c.Bind(&body); err != nil {
			return err
		}
		if body["password"] != "CorrectHorse123!" {
			t.Fatalf("request body was not restored: %+v", body)
		}
		return c.NoContent(http.StatusNoContent)
	}, platformmiddleware.LoginRateLimit(limiter, false))
	request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(
		`{"school_code":"12345678","email":" TEACHER@EXAMPLE.COM ","password":"CorrectHorse123!"}`,
	))
	request.RemoteAddr = "192.0.2.10:4567"
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Forwarded-For", "203.0.113.9")
	response := httptest.NewRecorder()
	e.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	if limiter.lastKey != "192.0.2.10\x00teacher@example.com" {
		t.Fatalf("unexpected limiter key %q", limiter.lastKey)
	}
	secondRequest := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(
		`{"school_code":"87654321","email":"teacher@example.com","password":"CorrectHorse123!"}`,
	))
	secondRequest.RemoteAddr = "192.0.2.10:4567"
	secondRequest.Header.Set("Content-Type", "application/json")
	secondResponse := httptest.NewRecorder()
	e.ServeHTTP(secondResponse, secondRequest)
	if secondResponse.Code != http.StatusNoContent {
		t.Fatalf("second login status = %d: %s", secondResponse.Code, secondResponse.Body.String())
	}
	if limiter.lastKey != "192.0.2.10\x00teacher@example.com" {
		t.Fatalf("school code changed the limiter key %q", limiter.lastKey)
	}
}

func TestRateLimitTrustsForwardedClientOnlyWhenEnabled(t *testing.T) {
	limiter := &fakeLimiter{allowed: true}
	e := echo.New()
	e.HTTPErrorHandler = apperror.HTTPErrorHandler
	e.POST("/login", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) },
		platformmiddleware.LoginRateLimit(limiter, true))
	request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(
		`{"email":" Teacher@Example.com "}`,
	))
	request.RemoteAddr = "127.0.0.1:1234"
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Forwarded-For", "203.0.113.9, 10.0.0.1")
	response := httptest.NewRecorder()
	e.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	if limiter.lastKey != "203.0.113.9\x00teacher@example.com" {
		t.Fatalf("unexpected trusted-proxy limiter key %q", limiter.lastKey)
	}
}

func TestRateLimitRejectionAndFailure(t *testing.T) {
	limiter := &fakeLimiter{allowed: false, retryAfter: 1500 * time.Millisecond}
	e := echo.New()
	e.HTTPErrorHandler = apperror.HTTPErrorHandler
	e.POST("/register", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) },
		platformmiddleware.RegisterRateLimit(limiter, false))
	request := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(`{}`))
	request.RemoteAddr = "192.0.2.20:4567"
	response := httptest.NewRecorder()
	e.ServeHTTP(response, request)
	if response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") != "2" {
		t.Fatalf("unexpected rate limit response: %d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
	}
}
