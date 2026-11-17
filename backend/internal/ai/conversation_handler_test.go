package ai

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"school-exam-studio/backend/internal/auth"
)

func newTestContext(t *testing.T, principal *auth.Principal) echo.Context {
	t.Helper()
	e := echo.New()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()
	context := e.NewContext(request, recorder)
	if principal != nil {
		context.Set("auth.principal", *principal)
	}
	return context
}

func TestHandlerListToolsRequiresAuthentication(t *testing.T) {
	handler := NewHandler(&Service{})
	context := newTestContext(t, nil)
	err := handler.ListTools(context)
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
}

func TestHandlerListToolsAndSkills(t *testing.T) {
	handler := NewHandler(&Service{})
	principal := auth.Principal{Role: auth.RoleTeacher, Subject: "数学"}
	context := newTestContext(t, &principal)
	if err := handler.ListTools(context); err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	context = newTestContext(t, &principal)
	if err := handler.ListSkills(context); err != nil {
		t.Fatalf("ListSkills: %v", err)
	}
}

func TestSkillCatalogHasDocumentImport(t *testing.T) {
	skills := (&Service{}).ListSkills()
	found := false
	for _, skill := range skills {
		if skill.ID == "document_import" {
			found = true
			if !skill.AcceptsAttachments {
				t.Fatal("document_import should accept attachments")
			}
		}
	}
	if !found {
		t.Fatal("document_import skill missing")
	}
}
