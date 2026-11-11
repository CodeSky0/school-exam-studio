package httpserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
	"school-exam-studio/backend/internal/auth"
	"school-exam-studio/backend/internal/platform/httpserver"
	"school-exam-studio/backend/internal/platform/platformtest"
)

const (
	apiSecret          = "01234567890123456789012345678901"
	apiOrigin          = "http://localhost:3000"
	apiInitialPassword = "CorrectHorse123!"
	apiChangedPassword = "NewCorrectHorse456!"
)

type allowAllLimiter struct{}

func (allowAllLimiter) Allow(context.Context, string) (bool, time.Duration, error) {
	return true, 0, nil
}

func TestAuthenticationAPIFlow(t *testing.T) {
	client := platformtest.Open(t)
	service := auth.NewService(
		auth.NewRepository(client),
		auth.NewPasswordHasher(bcrypt.MinCost),
		apiSecret,
		2*time.Hour,
	)
	newServer := func(secure bool) *echo.Echo {
		return httpserver.New(client, httpserver.Options{
			AuthService:     service,
			CORSOrigin:      apiOrigin,
			SecureCookie:    secure,
			LoginLimiter:    allowAllLimiter{},
			RegisterLimiter: allowAllLimiter{},
		})
	}
	server := newServer(false)

	status := apiRequest(t, server, http.MethodGet, "/api/v1/init/status", nil, nil, "")
	if status.Code != http.StatusOK || !strings.Contains(status.Body.String(), `"initialized":false`) {
		t.Fatalf("unexpected init status: %d %s", status.Code, status.Body.String())
	}
	invalidInit := apiRequest(t, server, http.MethodPost, "/api/v1/init", map[string]any{
		"school_name": "实验中学",
		"admin_name":  "管理员",
		"email":       "admin@example.com",
		"password":    apiInitialPassword,
		"school_id":   "client-must-not-set-this",
	}, nil, apiOrigin)
	assertAPIError(t, invalidInit, http.StatusBadRequest, "INVALID_JSON")

	initialized := apiRequest(t, server, http.MethodPost, "/api/v1/init", map[string]any{
		"school_name": "实验中学",
		"admin_name":  "管理员",
		"email":       " ADMIN@EXAMPLE.COM ",
		"password":    apiInitialPassword,
	}, nil, apiOrigin)
	if initialized.Code != http.StatusCreated {
		t.Fatalf("initialize returned %d: %s", initialized.Code, initialized.Body.String())
	}
	var initialization struct {
		SchoolID   string    `json:"school_id"`
		SchoolCode string    `json:"school_code"`
		User       auth.User `json:"user"`
	}
	decodeAPIBody(t, initialized, &initialization)
	if len(initialization.SchoolCode) != 8 || initialization.User.Email != "admin@example.com" {
		t.Fatalf("unexpected initialization response: %+v", initialization)
	}
	duplicateInit := apiRequest(t, server, http.MethodPost, "/api/v1/init", map[string]any{
		"school_name": "另一学校",
		"admin_name":  "管理员",
		"email":       "other@example.com",
		"password":    apiInitialPassword,
	}, nil, apiOrigin)
	assertAPIError(t, duplicateInit, http.StatusConflict, "ALREADY_INITIALIZED")

	registerBody := map[string]any{
		"school_code": initialization.SchoolCode,
		"name":        "数学教师",
		"subject":     "数学",
		"email":       " TEACHER@EXAMPLE.COM ",
		"password":    apiInitialPassword,
	}
	registered := apiRequest(t, server, http.MethodPost, "/api/v1/auth/register", registerBody, nil, apiOrigin)
	if registered.Code != http.StatusCreated || !strings.Contains(registered.Body.String(), `"role":"teacher"`) {
		t.Fatalf("register returned %d: %s", registered.Code, registered.Body.String())
	}
	duplicateRegistration := apiRequest(t, server, http.MethodPost, "/api/v1/auth/register", registerBody, nil, apiOrigin)
	assertAPIError(t, duplicateRegistration, http.StatusConflict, "EMAIL_ALREADY_EXISTS")

	wrongLogin := apiRequest(t, server, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"school_code": initialization.SchoolCode,
		"email":       "teacher@example.com",
		"password":    "WrongPassword123!",
	}, nil, apiOrigin)
	assertAPIError(t, wrongLogin, http.StatusUnauthorized, "INVALID_CREDENTIALS")
	missingLogin := apiRequest(t, server, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"school_code": initialization.SchoolCode,
		"email":       "missing@example.com",
		"password":    apiInitialPassword,
	}, nil, apiOrigin)
	assertAPIError(t, missingLogin, http.StatusUnauthorized, "INVALID_CREDENTIALS")
	var wrongEnvelope, missingEnvelope map[string]any
	decodeAPIBody(t, wrongLogin, &wrongEnvelope)
	decodeAPIBody(t, missingLogin, &missingEnvelope)
	delete(wrongEnvelope, "request_id")
	delete(missingEnvelope, "request_id")
	if fmt.Sprint(wrongEnvelope) != fmt.Sprint(missingEnvelope) {
		t.Fatalf("login failures are distinguishable: %v vs %v", wrongEnvelope, missingEnvelope)
	}

	login := apiRequest(t, server, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"school_code": initialization.SchoolCode,
		"email":       " TEACHER@EXAMPLE.COM ",
		"password":    apiInitialPassword,
	}, nil, apiOrigin)
	if login.Code != http.StatusOK {
		t.Fatalf("login returned %d: %s", login.Code, login.Body.String())
	}
	cookie := sessionCookie(t, login)
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Secure || cookie.MaxAge != 7200 {
		t.Fatalf("unexpected session cookie: %+v", cookie)
	}
	me := apiRequest(t, server, http.MethodGet, "/api/v1/auth/me", nil, cookie, "")
	if me.Code != http.StatusOK || !strings.Contains(me.Body.String(), `"email":"teacher@example.com"`) {
		t.Fatalf("me returned %d: %s", me.Code, me.Body.String())
	}

	changed := apiRequest(t, server, http.MethodPost, "/api/v1/auth/change-password", map[string]any{
		"old_password": apiInitialPassword,
		"new_password": apiChangedPassword,
	}, cookie, apiOrigin)
	if changed.Code != http.StatusOK {
		t.Fatalf("change password returned %d: %s", changed.Code, changed.Body.String())
	}
	cleared := sessionCookie(t, changed)
	if cleared.MaxAge != -1 || cleared.Value != "" || !cleared.HttpOnly || cleared.SameSite != http.SameSiteStrictMode {
		t.Fatalf("change password did not clear cookie: %+v", cleared)
	}
	oldSession := apiRequest(t, server, http.MethodGet, "/api/v1/auth/me", nil, cookie, "")
	assertAPIError(t, oldSession, http.StatusUnauthorized, "UNAUTHORIZED")

	oldPassword := apiRequest(t, server, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"school_code": initialization.SchoolCode,
		"email":       "teacher@example.com",
		"password":    apiInitialPassword,
	}, nil, apiOrigin)
	assertAPIError(t, oldPassword, http.StatusUnauthorized, "INVALID_CREDENTIALS")
	newLogin := apiRequest(t, server, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"school_code": initialization.SchoolCode,
		"email":       "teacher@example.com",
		"password":    apiChangedPassword,
	}, nil, apiOrigin)
	if newLogin.Code != http.StatusOK {
		t.Fatalf("new password login returned %d: %s", newLogin.Code, newLogin.Body.String())
	}
	newCookie := sessionCookie(t, newLogin)
	logout := apiRequest(t, server, http.MethodPost, "/api/v1/auth/logout", nil, newCookie, apiOrigin)
	if logout.Code != http.StatusOK || sessionCookie(t, logout).MaxAge != -1 {
		t.Fatalf("logout did not clear cookie: %d %s", logout.Code, logout.Body.String())
	}

	secureLogin := apiRequest(t, newServer(true), http.MethodPost, "/api/v1/auth/login", map[string]any{
		"school_code": initialization.SchoolCode,
		"email":       "teacher@example.com",
		"password":    apiChangedPassword,
	}, nil, apiOrigin)
	if secureLogin.Code != http.StatusOK || !sessionCookie(t, secureLogin).Secure {
		t.Fatalf("production cookie is not Secure: %d %s", secureLogin.Code, secureLogin.Body.String())
	}

	badOrigin := apiRequest(t, server, http.MethodPost, "/api/v1/auth/logout", nil, nil, "https://attacker.example")
	assertAPIError(t, badOrigin, http.StatusForbidden, "ORIGIN_NOT_ALLOWED")
	preflight := httptest.NewRequest(http.MethodOptions, "/api/v1/auth/login", nil)
	preflight.Header.Set("Origin", apiOrigin)
	preflight.Header.Set("Access-Control-Request-Method", http.MethodPost)
	preflightResponse := httptest.NewRecorder()
	server.ServeHTTP(preflightResponse, preflight)
	if preflightResponse.Header().Get("Access-Control-Allow-Origin") != apiOrigin ||
		preflightResponse.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatalf("unexpected CORS response: %d %v", preflightResponse.Code, preflightResponse.Header())
	}
}

func apiRequest(
	t *testing.T,
	server *echo.Echo,
	method, path string,
	body any,
	cookie *http.Cookie,
	origin string,
) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(encoded)
	}
	request := httptest.NewRequest(method, path, reader)
	request.Header.Set("Content-Type", "application/json")
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	if cookie != nil {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	return response
}

func sessionCookie(t *testing.T, response *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == auth.SessionCookieName {
			return cookie
		}
	}
	t.Fatalf("session cookie missing: %s", response.Body.String())
	return nil
}

func assertAPIError(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status = %d, want %d: %s", response.Code, status, response.Body.String())
	}
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Details []any  `json:"details"`
		} `json:"error"`
		RequestID string `json:"request_id"`
	}
	decodeAPIBody(t, response, &body)
	if body.Error.Code != code || body.RequestID == "" || body.Error.Details == nil {
		t.Fatalf("unexpected API error: %s", response.Body.String())
	}
	if response.Header().Get("X-Request-ID") != body.RequestID {
		t.Fatalf("request ID header/body mismatch: %s", response.Body.String())
	}
	if strings.Contains(response.Body.String(), "pq:") || strings.Contains(response.Body.String(), "goroutine") {
		t.Fatalf("internal details leaked: %s", response.Body.String())
	}
}

func decodeAPIBody(t *testing.T, response *httptest.ResponseRecorder, destination any) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), destination); err != nil {
		t.Fatalf("decode response %q: %v", response.Body.String(), err)
	}
}
