package httpserver_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"school-exam-studio/backend/internal/platform/httpserver"
	"school-exam-studio/backend/internal/platform/platformtest"
)

func TestHealthAndReadiness(t *testing.T) {
	client := platformtest.Open(t)
	server := httpserver.New(client)
	for _, path := range []string{"/healthz", "/readyz"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		server.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("%s returned %d: %s", path, response.Code, response.Body.String())
		}
	}
}
