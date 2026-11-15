package ai

import (
	"context"
	"strings"
	"testing"
	"time"

	"school-exam-studio/backend/ent/schoolaisetting"
	"school-exam-studio/backend/internal/auth"
	"school-exam-studio/backend/internal/platform/platformtest"
)

func TestSettingsCipherRoundTrip(t *testing.T) {
	cipher := newSettingsCipher("test-settings-secret")
	if cipher == nil {
		t.Fatal("expected cipher to be configured")
	}
	encrypted, err := cipher.encrypt("sk-secret-1234")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if !strings.HasPrefix(encrypted, settingsCipherPrefix) {
		t.Fatalf("expected versioned ciphertext, got %q", encrypted)
	}
	if strings.Contains(encrypted, "sk-secret-1234") {
		t.Fatal("ciphertext must not contain the plaintext key")
	}
	decrypted, err := cipher.decrypt(encrypted)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if decrypted != "sk-secret-1234" {
		t.Fatalf("unexpected decrypted key %q", decrypted)
	}
	if _, err := newSettingsCipher("test-settings-secret").decrypt(encrypted); err != nil {
		t.Fatalf("decrypt with same secret: %v", err)
	}
	if _, err := newSettingsCipher("another-secret").decrypt(encrypted); err == nil {
		t.Fatal("expected decryption with a different secret to fail")
	}
}

func TestNormalizeSchoolAISettings(t *testing.T) {
	normalized, err := normalizeSchoolAISettings(SchoolAISettingsInput{
		Provider: " DeepSeek ",
		BaseURL:  "https://api.deepseek.com/v1/",
		Models:   []string{"deepseek-chat", "deepseek-chat", " deepseek-reasoner "},
		Enabled:  true,
		APIKey:   " sk-test ",
	})
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if normalized.Provider != AIProviderDeepSeek {
		t.Fatalf("unexpected provider %q", normalized.Provider)
	}
	if normalized.BaseURL != "https://api.deepseek.com/v1" {
		t.Fatalf("unexpected base url %q", normalized.BaseURL)
	}
	if len(normalized.Models) != 2 || normalized.DefaultModel != "deepseek-chat" {
		t.Fatalf("unexpected models %#v default %q", normalized.Models, normalized.DefaultModel)
	}
	if normalized.APIKey != "sk-test" {
		t.Fatalf("unexpected api key %q", normalized.APIKey)
	}

	if _, err := normalizeSchoolAISettings(SchoolAISettingsInput{Provider: "unknown"}); err == nil {
		t.Fatal("expected unsupported provider to fail")
	}
	if _, err := normalizeSchoolAISettings(SchoolAISettingsInput{Provider: "custom", BaseURL: "ftp://example.com"}); err == nil {
		t.Fatal("expected invalid scheme to fail")
	}
	if _, err := normalizeSchoolAISettings(SchoolAISettingsInput{Provider: "custom", Models: []string{"a"}, DefaultModel: "b"}); err == nil {
		t.Fatal("expected default model outside enabled list to fail")
	}
}

func TestSchoolAISettingsAreSharedBySchoolAndEncrypted(t *testing.T) {
	client := platformtest.Open(t)
	platformtest.EnsureFixtures(t, client)
	service := NewService(Options{
		Repository:     NewRepository(client),
		SettingsSecret: "test-settings-secret",
		LLMTimeout:     time.Second,
	})
	ctx := context.Background()
	admin := auth.Principal{UserID: platformtest.AdminA, SchoolID: platformtest.SchoolA, Role: auth.RoleAdmin}
	teacherA := auth.Principal{UserID: platformtest.TeacherA, SchoolID: platformtest.SchoolA, Role: auth.RoleTeacher, Subject: "数学"}
	teacherB := auth.Principal{UserID: platformtest.TeacherB, SchoolID: platformtest.SchoolB, Role: auth.RoleTeacher, Subject: "数学"}

	view, err := service.UpdateSchoolAISettings(ctx, admin, SchoolAISettingsInput{
		Provider:     AIProviderDeepSeek,
		BaseURL:      "https://api.deepseek.com/v1",
		APIKey:       "sk-school-secret-1234",
		Models:       []string{"deepseek-chat", "deepseek-reasoner"},
		DefaultModel: "deepseek-chat",
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("update settings: %v", err)
	}
	if !view.Configured || !view.Enabled || !view.HasAPIKey || view.Source != "school" {
		t.Fatalf("unexpected settings view %#v", view)
	}
	if strings.Contains(view.APIKeyHint, "sk-school-secret") {
		t.Fatalf("api key hint leaked the secret: %q", view.APIKeyHint)
	}

	entity, err := client.SchoolAISetting.Query().
		Where(schoolaisetting.SchoolIDEQ(platformtest.SchoolA)).
		Only(ctx)
	if err != nil {
		t.Fatalf("load stored setting: %v", err)
	}
	if entity.APIKey == "sk-school-secret-1234" || !strings.HasPrefix(entity.APIKey, settingsCipherPrefix) {
		t.Fatalf("api key must be encrypted at rest, got %q", entity.APIKey)
	}

	statusA := service.Status(ctx, teacherA)
	if statusA.Source != "school" || len(statusA.Models) != 2 || statusA.DefaultModel != "deepseek-chat" {
		t.Fatalf("unexpected teacher status %#v", statusA)
	}
	statusB := service.Status(ctx, teacherB)
	if statusB.Source != "local" || len(statusB.Models) != 0 {
		t.Fatalf("school B must not inherit school A settings: %#v", statusB)
	}

	view, err = service.UpdateSchoolAISettings(ctx, admin, SchoolAISettingsInput{
		Provider:     AIProviderDeepSeek,
		BaseURL:      "https://api.deepseek.com/v1",
		APIKey:       "",
		Models:       []string{"deepseek-chat"},
		DefaultModel: "deepseek-chat",
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("update without key: %v", err)
	}
	if !view.HasAPIKey {
		t.Fatal("expected existing api key to be retained when the field is empty")
	}

	if _, err := service.UpdateSchoolAISettings(ctx, auth.Principal{UserID: platformtest.TeacherA, SchoolID: platformtest.SchoolA, Role: auth.RoleTeacher}, SchoolAISettingsInput{}); err == nil {
		t.Fatal("expected teacher to be forbidden from updating school settings")
	}

	if _, _, _, err := service.resolveLLM(ctx, teacherA, "not-enabled"); err == nil {
		t.Fatal("expected unenabled model selection to fail")
	}

	view, err = service.UpdateSchoolAISettings(ctx, admin, SchoolAISettingsInput{
		Provider:     AIProviderDeepSeek,
		BaseURL:      "https://api.deepseek.com/v1",
		Models:       []string{"deepseek-chat"},
		DefaultModel: "deepseek-chat",
		Enabled:      false,
	})
	if err != nil {
		t.Fatalf("disable settings: %v", err)
	}
	if view.Enabled || service.Status(ctx, teacherA).Source != "local" {
		t.Fatal("expected disabled school settings to fall back to the local engine")
	}
}
