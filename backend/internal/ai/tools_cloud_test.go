package ai

import (
	"testing"

	"school-exam-studio/backend/internal/auth"
)

func TestSearchCloudToolArguments(t *testing.T) {
	valid := map[string]any{"query": "一元二次方程", "limit": float64(5)}
	if err := validateToolArguments("search_cloud", valid); err != nil {
		t.Fatalf("expected valid arguments, got %v", err)
	}
	if err := validateToolArguments("search_cloud", map[string]any{"query": "  "}); err == nil {
		t.Fatal("expected blank query to fail")
	}
	if err := validateToolArguments("search_cloud", map[string]any{"query": "函数", "unexpected": true}); err == nil {
		t.Fatal("expected unknown argument to fail")
	}
}

func TestSearchCloudToolIsDiscoverable(t *testing.T) {
	service := &Service{}
	catalog := service.ToolCatalog(auth.Principal{Role: auth.RoleAdmin})
	found := false
	for _, tool := range catalog {
		if tool.Name == "search_cloud" {
			found = true
			if tool.Category != "document" {
				t.Fatalf("unexpected category: %s", tool.Category)
			}
			if tool.Writes {
				t.Fatal("search_cloud must be read-only")
			}
		}
	}
	if !found {
		t.Fatal("search_cloud tool is not in the catalog")
	}
}
