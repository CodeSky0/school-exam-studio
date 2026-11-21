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

func TestAttachmentPartsEncodesImagesWithoutOCR(t *testing.T) {
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
	parts, warnings := service.attachmentParts([]Attachment{{
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

func TestAttachmentPartsSendsPDFAsFile(t *testing.T) {
	store, err := NewAttachmentStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewAttachmentStore: %v", err)
	}
	data := []byte("%PDF-1.7 fake body")
	stored, err := store.Save(uuid.New(), uuid.New(), "试卷.pdf", "application/pdf", data)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	service := &Service{attachments: store}
	parts, warnings := service.attachmentParts([]Attachment{{
		ID: uuid.NewString(), Name: stored.Name, MIME: stored.MIME, Kind: AttachmentKindPDF,
		Status: AttachmentStatusParsed, SizeBytes: stored.SizeBytes, StoragePath: stored.StoragePath,
	}})
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if len(parts) != 1 || parts[0].Type != "file" || parts[0].File == nil {
		t.Fatalf("parts = %#v", parts)
	}
	if parts[0].File.Filename != stored.Name {
		t.Fatalf("filename = %q", parts[0].File.Filename)
	}
	prefix := "data:application/pdf;base64,"
	if !strings.HasPrefix(parts[0].File.FileData, prefix) {
		t.Fatalf("file_data prefix = %q", parts[0].File.FileData)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(parts[0].File.FileData, prefix))
	if err != nil || string(decoded) != string(data) {
		t.Fatalf("decoded pdf mismatch: %v", err)
	}
}

func TestAttachmentPartsInlinesText(t *testing.T) {
	store, err := NewAttachmentStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewAttachmentStore: %v", err)
	}
	stored, err := store.Save(uuid.New(), uuid.New(), "笔记.txt", "text/plain", []byte("第一章 有理数"))
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	service := &Service{attachments: store}
	parts, warnings := service.attachmentParts([]Attachment{{
		ID: uuid.NewString(), Name: stored.Name, MIME: stored.MIME, Kind: AttachmentKindText,
		Status: AttachmentStatusParsed, SizeBytes: stored.SizeBytes, StoragePath: stored.StoragePath,
	}})
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if len(parts) != 1 || parts[0].Type != "text" || !strings.Contains(parts[0].Text, "第一章 有理数") {
		t.Fatalf("parts = %#v", parts)
	}
}

func TestAttachmentPartsSkipsOversizedImages(t *testing.T) {
	service := &Service{attachments: &AttachmentStore{root: t.TempDir()}}
	parts, warnings := service.attachmentParts([]Attachment{{
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

func TestLocalPlannerExplainsAttachmentsNeedModel(t *testing.T) {
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
	if len(found.RequiredTools) != 0 {
		t.Fatalf("required tools = %v, want none", found.RequiredTools)
	}
}

func TestToolCatalogRemovesDocumentParsingTools(t *testing.T) {
	service := &Service{}
	catalog := service.ToolCatalog(testPrincipal())
	found := map[string]bool{}
	for _, item := range catalog {
		found[item.Name] = true
	}
	for _, name := range []string{"read_attachment", "extract_questions", "import_document_questions"} {
		if found[name] {
			t.Fatalf("%s should be removed from tool catalog", name)
		}
	}
	if !found["list_attachments"] || !found["create_questions"] {
		t.Fatalf("expected list_attachments and create_questions in catalog, got %v", found)
	}
}

func TestValidateToolArgumentsRejectsRemovedTools(t *testing.T) {
	for _, name := range []string{"read_attachment", "extract_questions", "import_document_questions"} {
		if err := validateToolArguments(name, map[string]any{}); err == nil {
			t.Fatalf("%s should be rejected", name)
		}
	}
	if err := validateToolArguments("list_attachments", map[string]any{}); err != nil {
		t.Fatalf("list_attachments rejected: %v", err)
	}
}
