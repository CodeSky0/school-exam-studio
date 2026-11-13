package httpserver_test

import (
	"net/http"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
	"school-exam-studio/backend/internal/auth"
	"school-exam-studio/backend/internal/paper"
	"school-exam-studio/backend/internal/platform/httpserver"
	"school-exam-studio/backend/internal/platform/platformtest"
	"school-exam-studio/backend/internal/question"
)

func TestPaperAPIContractAndPermissions(t *testing.T) {
	client := platformtest.Open(t)
	platformtest.EnsureFixtures(t, client)
	authService := auth.NewService(auth.NewRepository(client), auth.NewPasswordHasher(bcrypt.MinCost), apiSecret, 2*time.Hour)
	paperService := paper.NewService(paper.NewRepository(client))
	server := httpserver.New(client, httpserver.Options{
		AuthService: authService, PaperService: paperService, CORSOrigin: apiOrigin,
		LoginLimiter: allowAllLimiter{}, RegisterLimiter: allowAllLimiter{},
	})
	teacherCookie := issueCookie(t, authService, platformtest.TeacherPrincipal(client))
	adminCookie := issueCookie(t, authService, platformtest.AdminPrincipal(client))
	otherTeacherCookie := issueCookie(t, authService, auth.Principal{
		UserID: platformtest.OtherTeacherA, SchoolID: platformtest.SchoolA, Role: auth.RoleTeacher, SessionVersion: 1,
	})
	foreignCookie := issueCookie(t, authService, platformtest.TeacherBPrincipal())
	published := platformtest.Question(t, client, platformtest.SchoolA, platformtest.TeacherA, question.StatusPublished)
	draft := platformtest.Question(t, client, platformtest.SchoolA, platformtest.TeacherA, question.StatusDraft)

	response := apiRequest(t, server, http.MethodGet, "/api/v1/papers", nil, nil, "")
	assertAPIError(t, response, http.StatusUnauthorized, "UNAUTHORIZED")
	body := paperAPIInput(published.ID.String(), "0.10")
	response = apiRequest(t, server, http.MethodPost, "/api/v1/papers", body, teacherCookie, apiOrigin)
	if response.Code != http.StatusCreated {
		t.Fatalf("create paper returned %d: %s", response.Code, response.Body.String())
	}
	var created paper.Paper
	decodeAPIBody(t, response, &created)
	if created.TotalScore != "0.10" || created.SchoolName != "甲学校" || len(created.Questions) != 1 || created.Questions[0].Question.ID != published.ID {
		t.Fatalf("unexpected create paper response: %s", response.Body.String())
	}

	response = apiRequest(t, server, http.MethodGet, "/api/v1/papers?subject=数学&grade=七年级&page=1&page_size=1000", nil, teacherCookie, "")
	if response.Code != http.StatusOK {
		t.Fatalf("list papers returned %d: %s", response.Code, response.Body.String())
	}
	var listed paper.ListResult
	decodeAPIBody(t, response, &listed)
	if listed.Total != 1 || listed.PageSize != 100 || len(listed.Items) != 1 || listed.Items[0].Questions != nil || listed.Items[0].QuestionCount != 1 {
		t.Fatalf("unexpected paper list response: %s", response.Body.String())
	}

	response = apiRequest(t, server, http.MethodPost, "/api/v1/papers", paperAPIInput(draft.ID.String(), "1.00"), teacherCookie, apiOrigin)
	assertAPIError(t, response, http.StatusUnprocessableEntity, "QUESTION_UNAVAILABLE")
	duplicate := paperAPIInput(published.ID.String(), "0.10")
	duplicate["questions"] = []map[string]string{
		{"question_id": published.ID.String(), "assigned_score": "0.10"},
		{"question_id": published.ID.String(), "assigned_score": "0.20"},
	}
	response = apiRequest(t, server, http.MethodPost, "/api/v1/papers", duplicate, teacherCookie, apiOrigin)
	assertAPIError(t, response, http.StatusUnprocessableEntity, "DUPLICATE_QUESTION")
	response = apiRequest(t, server, http.MethodPost, "/api/v1/papers", paperAPIInput(published.ID.String(), "0.001"), teacherCookie, apiOrigin)
	assertAPIError(t, response, http.StatusBadRequest, "VALIDATION_ERROR")
	response = apiRequest(t, server, http.MethodPost, "/api/v1/papers", body, foreignCookie, apiOrigin)
	assertAPIError(t, response, http.StatusUnprocessableEntity, "QUESTION_UNAVAILABLE")

	response = apiRequest(t, server, http.MethodGet, "/api/v1/papers/"+created.ID.String(), nil, otherTeacherCookie, "")
	assertAPIError(t, response, http.StatusForbidden, "FORBIDDEN")
	response = apiRequest(t, server, http.MethodPatch, "/api/v1/papers/"+created.ID.String(), body, otherTeacherCookie, apiOrigin)
	assertAPIError(t, response, http.StatusForbidden, "FORBIDDEN")
	response = apiRequest(t, server, http.MethodGet, "/api/v1/papers/"+created.ID.String(), nil, foreignCookie, "")
	assertAPIError(t, response, http.StatusNotFound, "PAPER_NOT_FOUND")

	adminUpdate := paperAPIInput(published.ID.String(), "0.20")
	adminUpdate["title"] = "管理员更新"
	response = apiRequest(t, server, http.MethodPatch, "/api/v1/papers/"+created.ID.String(), adminUpdate, adminCookie, apiOrigin)
	if response.Code != http.StatusOK {
		t.Fatalf("admin update returned %d: %s", response.Code, response.Body.String())
	}
	response = apiRequest(t, server, http.MethodDelete, "/api/v1/papers/"+created.ID.String(), nil, teacherCookie, apiOrigin)
	if response.Code != http.StatusNoContent {
		t.Fatalf("delete paper returned %d: %s", response.Code, response.Body.String())
	}
	response = apiRequest(t, server, http.MethodGet, "/api/v1/papers/"+created.ID.String(), nil, teacherCookie, "")
	assertAPIError(t, response, http.StatusNotFound, "PAPER_NOT_FOUND")
}

func paperAPIInput(questionID, score string) map[string]any {
	return map[string]any{
		"title": "API练习", "subject": "数学", "grade": "七年级", "duration_minutes": 60,
		"questions": []map[string]string{{"question_id": questionID, "assigned_score": score}},
	}
}
