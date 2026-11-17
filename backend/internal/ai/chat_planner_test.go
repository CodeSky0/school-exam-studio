package ai

import (
	"testing"

	"school-exam-studio/backend/internal/auth"
)

func TestPlannerRoutesDocumentImport(t *testing.T) {
	plan := planChatTurn("把这份 PDF 里的题导入八年级数学题库", []AttachmentRef{
		{ID: "11111111-1111-1111-1111-111111111111", Kind: AttachmentKindPDF, Status: AttachmentStatusParsed},
	}, nil, auth.Principal{Role: auth.RoleAdmin})
	if len(plan.Calls) != 2 {
		t.Fatalf("calls = %#v", plan.Calls)
	}
	if plan.Calls[0].Name != "extract_questions" || plan.Calls[1].Name != "import_document_questions" {
		t.Fatalf("calls = %#v", plan.Calls)
	}
}

func TestPlannerAsksForSubjectAndGradeBeforeImport(t *testing.T) {
	plan := planChatTurn("导入这些题", []AttachmentRef{
		{ID: "11111111-1111-1111-1111-111111111111", Kind: AttachmentKindDocx, Status: AttachmentStatusParsed},
	}, nil, auth.Principal{Role: auth.RoleAdmin})
	if !plan.Clarify || len(plan.Calls) != 0 {
		t.Fatalf("plan = %#v", plan)
	}
}

func TestPlannerUsesTeacherSubject(t *testing.T) {
	plan := planChatTurn("给八年级出 3 道题", nil, nil, auth.Principal{Role: auth.RoleTeacher, Subject: "数学"})
	if len(plan.Calls) != 1 || plan.Calls[0].Name != "generate_questions" {
		t.Fatalf("calls = %#v", plan.Calls)
	}
	if plan.Calls[0].Arguments["subject"] != "数学" {
		t.Fatalf("args = %#v", plan.Calls[0].Arguments)
	}
}

func TestPlannerRoutesAudit(t *testing.T) {
	plan := planChatTurn("帮我体检一下题库，看看重复题", nil, nil, auth.Principal{Role: auth.RoleAdmin})
	if len(plan.Calls) != 1 || plan.Calls[0].Name != "bank_audit" {
		t.Fatalf("calls = %#v", plan.Calls)
	}
}

func TestPlannerExplainsCapabilities(t *testing.T) {
	plan := planChatTurn("你好", nil, nil, auth.Principal{Role: auth.RoleAdmin})
	if len(plan.Calls) != 0 || plan.Reply == "" {
		t.Fatalf("plan = %#v", plan)
	}
}

func TestPlannerRequiresUploadForImport(t *testing.T) {
	plan := planChatTurn("把题导入题库", nil, nil, auth.Principal{Role: auth.RoleAdmin})
	if len(plan.Calls) != 0 {
		t.Fatalf("calls = %#v", plan.Calls)
	}
}

func TestPlannerUsesLastQuestionIDForVariants(t *testing.T) {
	prior := []ChatMessage{{
		Role: MessageRoleAssistant,
		ToolCalls: []ToolCall{{
			Result: map[string]any{"question_ids": []any{"22222222-2222-2222-2222-222222222222"}},
		}},
	}}
	plan := planChatTurn("给它做 2 道变式", nil, prior, auth.Principal{Role: auth.RoleAdmin})
	if len(plan.Calls) != 1 || plan.Calls[0].Name != "generate_variants" {
		t.Fatalf("calls = %#v", plan.Calls)
	}
	if plan.Calls[0].Arguments["question_id"] != "22222222-2222-2222-2222-222222222222" {
		t.Fatalf("args = %#v", plan.Calls[0].Arguments)
	}
}

func TestValidateToolArgumentsRejectsUnknownField(t *testing.T) {
	if err := validateToolArguments("bank_audit", map[string]any{"unknown": true}); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestToolCatalogCoversDocumentImport(t *testing.T) {
	names := map[string]bool{}
	for _, item := range (&Service{}).ToolCatalog(auth.Principal{Role: auth.RoleAdmin}) {
		names[item.Name] = true
	}
	for _, want := range []string{"extract_questions", "import_document_questions", "create_questions", "update_draft", "compose_paper"} {
		if !names[want] {
			t.Fatalf("missing tool %s", want)
		}
	}
}
