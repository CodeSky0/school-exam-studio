package httpserver_test

import (
	"bytes"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
	"school-exam-studio/backend/internal/auth"
	"school-exam-studio/backend/internal/ocr"
	"school-exam-studio/backend/internal/platform/httpserver"
	"school-exam-studio/backend/internal/platform/platformtest"
	"school-exam-studio/backend/internal/question"
)

func TestQuestionAndOCRAPIEndToEnd(t *testing.T) {
	client := platformtest.Open(t)
	platformtest.EnsureFixtures(t, client)
	authService := auth.NewService(auth.NewRepository(client), auth.NewPasswordHasher(bcrypt.MinCost), apiSecret, 2*time.Hour)
	questionService := question.NewService(question.NewRepository(client), question.NewSanitizer())
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reader, err := r.MultipartReader()
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		part, err := reader.NextPart()
		if err != nil || part.FormName() != "image" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = io.Copy(io.Discard, part)
		_, _ = io.WriteString(w, `{"text":"识别 $x^2$"}`)
	}))
	defer upstream.Close()
	ocrClient, err := ocr.NewClient(upstream.URL, time.Second, 2)
	if err != nil {
		t.Fatal(err)
	}
	server := httpserver.New(client, httpserver.Options{
		AuthService: authService, QuestionService: questionService, OCRClient: ocrClient,
		CORSOrigin: apiOrigin, LoginLimiter: allowAllLimiter{}, RegisterLimiter: allowAllLimiter{},
	})
	teacherCookie := issueCookie(t, authService, platformtest.TeacherPrincipal(client))
	adminCookie := issueCookie(t, authService, platformtest.AdminPrincipal(client))
	foreignCookie := issueCookie(t, authService, platformtest.TeacherBPrincipal())
	ownDraft := platformtest.Question(t, client, platformtest.SchoolA, platformtest.TeacherA, question.StatusDraft)
	foreignPublished := platformtest.Question(t, client, platformtest.SchoolB, platformtest.TeacherB, question.StatusPublished)

	createBody := map[string]any{
		"subject": "数学", "grade": "七年级", "question_type": "single_choice", "difficulty": 2,
		"tags": []string{"代数"}, "content": `<p>题干<script>alert(1)</script><span data-type="inlineMath" data-latex="x^2"></span></p>`,
		"options": []map[string]string{{"key": "A", "content": "1"}, {"key": "B", "content": "2"}},
		"answer":  "B", "explanation": "<p>解析</p>",
	}
	rejected := createBodyWithForbiddenFields(createBody)
	response := apiRequest(t, server, http.MethodPost, "/api/v1/questions", rejected, teacherCookie, apiOrigin)
	assertAPIError(t, response, http.StatusBadRequest, "INVALID_JSON")

	response = apiRequest(t, server, http.MethodPost, "/api/v1/questions", createBody, teacherCookie, apiOrigin)
	if response.Code != http.StatusCreated {
		t.Fatalf("create question returned %d: %s", response.Code, response.Body.String())
	}
	var created question.Question
	decodeAPIBody(t, response, &created)
	if created.CreatorID != platformtest.TeacherA || created.CreatorName != "甲校数学教师" || created.Status != question.StatusDraft || strings.Contains(response.Body.String(), "school_id") {
		t.Fatalf("unexpected create response: %s", response.Body.String())
	}
	if strings.Contains(created.Content, "script") || !strings.Contains(created.Content, `data-type="inlineMath"`) {
		t.Fatalf("content was not safely sanitized: %s", created.Content)
	}
	response = apiRequest(t, server, http.MethodGet, "/api/v1/questions?subject=数学&grade=七年级&page=1&page_size=1", nil, teacherCookie, "")
	if response.Code != http.StatusOK {
		t.Fatalf("list returned %d: %s", response.Code, response.Body.String())
	}
	var listed question.ListResult
	decodeAPIBody(t, response, &listed)
	if listed.Total != 2 || listed.Page != 1 || listed.PageSize != 1 || len(listed.Items) != 1 {
		t.Fatalf("unexpected list contract: %s", response.Body.String())
	}

	response = apiRequest(t, server, http.MethodGet, "/api/v1/questions/"+created.ID.String(), nil, foreignCookie, "")
	assertAPIError(t, response, http.StatusNotFound, "QUESTION_NOT_FOUND")
	response = apiRequest(t, server, http.MethodPatch, "/api/v1/admin/questions/"+created.ID.String()+"/status", map[string]string{"status": "published"}, teacherCookie, apiOrigin)
	assertAPIError(t, response, http.StatusForbidden, "FORBIDDEN")
	response = apiRequest(t, server, http.MethodPatch, "/api/v1/admin/questions/"+created.ID.String()+"/status", map[string]string{"status": "published"}, adminCookie, apiOrigin)
	if response.Code != http.StatusOK {
		t.Fatalf("admin publish returned %d: %s", response.Code, response.Body.String())
	}
	response = apiRequest(t, server, http.MethodPatch, "/api/v1/questions/"+created.ID.String(), createBody, teacherCookie, apiOrigin)
	assertAPIError(t, response, http.StatusForbidden, "FORBIDDEN")

	response = apiRequest(t, server, http.MethodPost, "/api/v1/questions/resolve", map[string]any{
		"ids": []string{foreignPublished.ID.String(), ownDraft.ID.String(), created.ID.String()},
	}, teacherCookie, apiOrigin)
	if response.Code != http.StatusOK {
		t.Fatalf("resolve returned %d: %s", response.Code, response.Body.String())
	}
	var resolved struct {
		Items []question.Question `json:"items"`
	}
	decodeAPIBody(t, response, &resolved)
	if len(resolved.Items) != 1 || resolved.Items[0].ID != created.ID {
		t.Fatalf("resolve returned unavailable questions: %s", response.Body.String())
	}

	beforeOCR := platformtest.QuestionCount(t, client, platformtest.SchoolA)
	unauthorizedRequest := multipartRequest(t, "/api/v1/ocr/recognize", apiPNG(t), nil)
	unauthorizedResponse := httptest.NewRecorder()
	server.ServeHTTP(unauthorizedResponse, unauthorizedRequest)
	assertAPIError(t, unauthorizedResponse, http.StatusUnauthorized, "UNAUTHORIZED")

	ocrRequest := multipartRequest(t, "/api/v1/ocr/recognize", apiPNG(t), teacherCookie)
	ocrResponse := httptest.NewRecorder()
	server.ServeHTTP(ocrResponse, ocrRequest)
	if ocrResponse.Code != http.StatusOK || !strings.Contains(ocrResponse.Body.String(), `"text":"识别 $x^2$"`) {
		t.Fatalf("OCR returned %d: %s", ocrResponse.Code, ocrResponse.Body.String())
	}
	if afterOCR := platformtest.QuestionCount(t, client, platformtest.SchoolA); afterOCR != beforeOCR {
		t.Fatalf("OCR persisted questions: before=%d after=%d", beforeOCR, afterOCR)
	}
}

func createBodyWithForbiddenFields(input map[string]any) map[string]any {
	result := make(map[string]any, len(input)+3)
	for key, value := range input {
		result[key] = value
	}
	result["school_id"] = "00000000-0000-4000-8000-000000000002"
	result["creator_id"] = "00000000-0000-4000-8000-000000000102"
	result["status"] = "published"
	return result
}

func issueCookie(t *testing.T, service *auth.Service, principal auth.Principal) *http.Cookie {
	t.Helper()
	token, err := service.Tokens().Issue(principal)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Cookie{Name: auth.SessionCookieName, Value: token}
}

func multipartRequest(t *testing.T, path string, imageBytes []byte, cookie *http.Cookie) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("image", "question.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(imageBytes); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, path, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Origin", apiOrigin)
	if cookie != nil {
		request.AddCookie(cookie)
	}
	return request
}

func apiPNG(t *testing.T) []byte {
	t.Helper()
	var body bytes.Buffer
	if err := png.Encode(&body, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	return body.Bytes()
}
