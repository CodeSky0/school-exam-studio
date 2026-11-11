package httpserver_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"school-exam-studio/backend/internal/platform/httpserver"
)

func TestCORSWildcardIsNotEnabled(t *testing.T) {
	server := httpserver.New(nil, httpserver.Options{CORSOrigin: "*"})
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	request.Header.Set("Origin", "https://attacker.example")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("health status = %d", response.Code)
	}
	if origin := response.Header().Get("Access-Control-Allow-Origin"); origin != "" {
		t.Fatalf("wildcard CORS was enabled: %q", origin)
	}
}
