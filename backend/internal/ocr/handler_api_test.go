package ocr_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"school-exam-studio/backend/internal/auth"
	"school-exam-studio/backend/internal/ocr"
	"school-exam-studio/backend/internal/platform/apperror"
)

func TestOCRHandlerMapsUpstreamErrorsToHTTP(t *testing.T) {
	tests := []struct {
		name           string
		upstreamStatus int
		upstreamBody   string
		delay          time.Duration
		clientTimeout  time.Duration
		wantStatus     int
		wantCode       string
	}{
		{"not ready", http.StatusServiceUnavailable, `{}`, 0, time.Second, http.StatusServiceUnavailable, "OCR_UNAVAILABLE"},
		{"upstream failure", http.StatusInternalServerError, `{}`, 0, time.Second, http.StatusBadGateway, "OCR_UPSTREAM_ERROR"},
		{"timeout", http.StatusOK, `{"text":"late"}`, 80 * time.Millisecond, 10 * time.Millisecond, http.StatusGatewayTimeout, "OCR_TIMEOUT"},
		{"empty result", http.StatusOK, `{"text":" "}`, 0, time.Second, http.StatusUnprocessableEntity, "EMPTY_OCR_RESULT"},
		{"sidecar empty result", http.StatusUnprocessableEntity, `{"error":{"code":"EMPTY_RECOGNITION"}}`, 0, time.Second, http.StatusUnprocessableEntity, "EMPTY_OCR_RESULT"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if test.delay > 0 {
					time.Sleep(test.delay)
				}
				w.WriteHeader(test.upstreamStatus)
				_, _ = io.WriteString(w, test.upstreamBody)
			}))
			defer upstream.Close()
			client, err := ocr.NewClient(upstream.URL, test.clientTimeout, 1)
			if err != nil {
				t.Fatal(err)
			}
			e := echo.New()
			e.HTTPErrorHandler = apperror.HTTPErrorHandler
			e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
				return func(c echo.Context) error {
					c.Set("auth.principal", auth.Principal{Role: auth.RoleTeacher})
					return next(c)
				}
			})
			ocr.NewHandler(client).Register(e.Group("/api/v1"))
			request := ocrMultipartRequest(t, testPNG(t))
			response := httptest.NewRecorder()
			e.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status=%d, want %d: %s", response.Code, test.wantStatus, response.Body.String())
			}
			var body struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Error.Code != test.wantCode {
				t.Fatalf("error code=%q, want %q: %s", body.Error.Code, test.wantCode, response.Body.String())
			}
		})
	}
}

func ocrMultipartRequest(t *testing.T, image []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("image", "question.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(image); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/ocr/recognize", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}
