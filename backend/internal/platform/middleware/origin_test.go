package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"school-exam-studio/backend/internal/platform/apperror"
	platformmiddleware "school-exam-studio/backend/internal/platform/middleware"
)

func TestOriginProtection(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = apperror.HTTPErrorHandler
	e.Use(platformmiddleware.OriginProtection("http://localhost:3000"))
	handler := func(c echo.Context) error { return c.NoContent(http.StatusNoContent) }
	e.POST("/resource", handler)
	e.GET("/resource", handler)

	tests := []struct {
		name       string
		method     string
		origin     string
		wantStatus int
	}{
		{"matching origin", http.MethodPost, "http://localhost:3000", http.StatusNoContent},
		{"missing origin", http.MethodPost, "", http.StatusForbidden},
		{"untrusted origin", http.MethodPost, "https://attacker.example", http.StatusForbidden},
		{"null origin", http.MethodPost, "null", http.StatusForbidden},
		{"safe method does not require origin", http.MethodGet, "", http.StatusNoContent},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, "/resource", nil)
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			response := httptest.NewRecorder()
			e.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d: %s", response.Code, test.wantStatus, response.Body.String())
			}
		})
	}
}
