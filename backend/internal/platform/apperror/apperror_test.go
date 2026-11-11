package apperror_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"school-exam-studio/backend/internal/platform/apperror"
)

func TestErrorEnvelopeDoesNotLeakInternalError(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = apperror.HTTPErrorHandler
	e.GET("/failure", func(c echo.Context) error {
		c.Response().Header().Set(echo.HeaderXRequestID, "request-test-id")
		return apperror.Internal(errors.New("pq: duplicate key value contains secret SQL"))
	})
	request := httptest.NewRequest(http.MethodGet, "/failure", nil)
	response := httptest.NewRecorder()
	e.ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", response.Code)
	}
	if strings.Contains(response.Body.String(), "pq:") || strings.Contains(response.Body.String(), "SQL") {
		t.Fatalf("internal details leaked: %s", response.Body.String())
	}
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Details []any  `json:"details"`
		} `json:"error"`
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != "INTERNAL_ERROR" || body.RequestID != "request-test-id" || body.Error.Details == nil {
		t.Fatalf("unexpected error envelope: %s", response.Body.String())
	}
}
