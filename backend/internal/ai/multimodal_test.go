package ai

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestLLMMessageMarshalKeepsStringContent(t *testing.T) {
	raw, err := json.Marshal(LLMMessage{Role: "user", Content: "普通文本"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if text, ok := payload["content"].(string); !ok || text != "普通文本" {
		t.Fatalf("content = %#v, want string", payload["content"])
	}
}

func TestLLMMessageMarshalContentParts(t *testing.T) {
	message := LLMMessage{
		Role:    "user",
		Content: "请看图片",
		Parts: []LLMContentPart{
			{Type: "text", Text: "请看图片"},
			{Type: "image_url", ImageURL: &LLMImageURL{URL: "data:image/png;base64,AAAA"}},
		},
	}
	raw, err := json.Marshal(message)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var payload struct {
		Role    string           `json:"role"`
		Content []LLMContentPart `json:"content"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload.Role != "user" || len(payload.Content) != 2 {
		t.Fatalf("payload = %#v", payload)
	}
	if payload.Content[0].Type != "text" || payload.Content[0].Text != "请看图片" {
		t.Fatalf("text part = %#v", payload.Content[0])
	}
	if payload.Content[1].Type != "image_url" || payload.Content[1].ImageURL == nil {
		t.Fatalf("image part = %#v", payload.Content[1])
	}
}

func TestAttachmentImagePartsEncodesImagesWithoutOCR(t *testing.T) {
	store, err := NewAttachmentStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewAttachmentStore: %v", err)
	}
	image := append([]byte("\x89PNG\r\n\x1a\n"), []byte("fake-image-body")...)
	stored, err := store.Save(uuid.New(), uuid.New(), "题目.png", "image/png", image)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	service := &Service{attachments: store}
	parts, warnings := service.attachmentImageParts([]Attachment{{
		ID: uuid.NewString(), Name: stored.Name, MIME: stored.MIME, Kind: AttachmentKindImage,
		Status: AttachmentStatusParsed, SizeBytes: stored.SizeBytes, StoragePath: stored.StoragePath,
	}})
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if len(parts) != 1 || parts[0].Type != "image_url" || parts[0].ImageURL == nil {
		t.Fatalf("parts = %#v", parts)
	}
	prefix := "data:image/png;base64,"
	if !strings.HasPrefix(parts[0].ImageURL.URL, prefix) {
		t.Fatalf("url prefix = %q", parts[0].ImageURL.URL)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(parts[0].ImageURL.URL, prefix))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if string(decoded) != string(image) {
		t.Fatal("decoded image does not match uploaded bytes")
	}
}

func TestAttachmentImagePartsSkipsOversizedImages(t *testing.T) {
	service := &Service{attachments: &AttachmentStore{root: t.TempDir()}}
	parts, warnings := service.attachmentImageParts([]Attachment{{
		Name: "big.png", Kind: AttachmentKindImage, Status: AttachmentStatusParsed,
		SizeBytes: maxVisionImageBytes + 1,
	}})
	if len(parts) != 0 {
		t.Fatalf("parts = %#v, want none", parts)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "已跳过视觉输入") {
		t.Fatalf("warnings = %v", warnings)
	}
}

func TestLocalPlannerExplainsImageNeedsVisionModel(t *testing.T) {
	plan := planChatTurn("请阅读这张图片", []AttachmentRef{{
		ID: "image-1", Name: "题目.png", Kind: AttachmentKindImage, Status: AttachmentStatusParsed,
	}}, nil, testPrincipal())
	if len(plan.Calls) != 0 {
		t.Fatalf("calls = %#v, want none", plan.Calls)
	}
	if !strings.Contains(plan.Reply, "多模态模型") {
		t.Fatalf("reply = %q", plan.Reply)
	}
}

func TestDocumentReaderSkillIsDiscoverable(t *testing.T) {
	service := &Service{}
	var found *Skill
	for _, item := range service.ListSkills() {
		if item.ID == "document_reader" {
			copy := item
			found = &copy
		}
	}
	if found == nil {
		t.Fatal("document_reader skill not found")
	}
	if !found.AcceptsAttachments {
		t.Fatal("document_reader must accept attachments")
	}
	required := map[string]bool{}
	for _, name := range found.RequiredTools {
		required[name] = true
	}
	if !required["read_attachment"] || !required["list_attachments"] {
		t.Fatalf("required tools = %v", found.RequiredTools)
	}
}

func TestToolCatalogIncludesReadAttachment(t *testing.T) {
	service := &Service{}
	catalog := service.ToolCatalog(testPrincipal())
	for _, item := range catalog {
		if item.Name == "read_attachment" {
			if item.Category != "document" || item.Writes {
				t.Fatalf("descriptor = %#v", item)
			}
			return
		}
	}
	t.Fatal("read_attachment not found in tool catalog")
}

func TestValidateToolArgumentsAcceptsReadAttachment(t *testing.T) {
	if err := validateToolArguments("read_attachment", map[string]any{
		"attachment_ids": []any{"6f1b0d7e-1f2a-4a4b-9c7d-3f0f3c9a1b2c"},
		"offset":         float64(0),
		"limit":          float64(4000),
	}); err != nil {
		t.Fatalf("valid args rejected: %v", err)
	}
	if err := validateToolArguments("read_attachment", map[string]any{"unknown": true}); err == nil {
		t.Fatal("unknown field should be rejected")
	}
}
