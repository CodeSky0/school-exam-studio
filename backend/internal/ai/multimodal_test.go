package ai

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"sort"
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

func TestAttachmentPartsExtractsPDFText(t *testing.T) {
	store, err := NewAttachmentStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewAttachmentStore: %v", err)
	}
	data := buildTestPDF(map[int]string{
		1: "<< /Type /Catalog /Pages 2 0 R >>",
		2: "<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		3: "<< /Type /Page /Parent 2 0 R /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
		4: "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		5: pdfTestStream("", "BT /F1 12 Tf 72 720 Td (This PDF has a real text layer for the assistant) Tj ET"),
	})
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
	if len(parts) != 1 || parts[0].Type != "text" {
		t.Fatalf("parts = %#v", parts)
	}
	if !strings.Contains(parts[0].Text, "real text layer") {
		t.Fatalf("text = %q", parts[0].Text)
	}
	for _, part := range parts {
		if part.Type == "file" {
			t.Fatal("file content part must not be sent")
		}
	}
}

func TestAttachmentPartsSendsPDFPageImageWhenTextLayerMissing(t *testing.T) {
	store, err := NewAttachmentStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewAttachmentStore: %v", err)
	}
	var jpegData bytes.Buffer
	if err := jpeg.Encode(&jpegData, testPageImage(), &jpeg.Options{Quality: 80}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	data := buildTestPDF(map[int]string{
		1: "<< /Type /Catalog /Pages 2 0 R >>",
		2: "<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		3: "<< /Type /Page /Parent 2 0 R /Resources << /XObject << /Im1 4 0 R >> >> /Contents 5 0 R >>",
		4: fmt.Sprintf("<< /Type /XObject /Subtype /Image /Width 400 /Height 400 /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /DCTDecode /Length %d >>\nstream\n%s\nendstream", jpegData.Len(), jpegData.String()),
		5: pdfTestStream("", "q Q"),
	})
	stored, err := store.Save(uuid.New(), uuid.New(), "扫描件.pdf", "application/pdf", data)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	service := &Service{attachments: store}
	parts, warnings := service.attachmentParts([]Attachment{{
		ID: uuid.NewString(), Name: stored.Name, MIME: stored.MIME, Kind: AttachmentKindPDF,
		Status: AttachmentStatusParsed, SizeBytes: stored.SizeBytes, StoragePath: stored.StoragePath,
	}})
	if len(parts) != 2 || parts[0].Type != "text" || parts[1].Type != "image_url" || parts[1].ImageURL == nil {
		t.Fatalf("parts = %#v, warnings = %v", parts, warnings)
	}
	if !strings.Contains(parts[1].ImageURL.URL, "data:image/jpeg;base64,") {
		t.Fatalf("image url = %q", parts[1].ImageURL.URL)
	}
}

func TestAttachmentPartsWarnsWhenPDFHasNeitherTextNorImages(t *testing.T) {
	store, err := NewAttachmentStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewAttachmentStore: %v", err)
	}
	data := buildTestPDF(map[int]string{
		1: "<< /Type /Catalog /Pages 2 0 R >>",
		2: "<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		3: "<< /Type /Page /Parent 2 0 R /Resources << >> /Contents 4 0 R >>",
		4: pdfTestStream("", "q Q"),
	})
	stored, err := store.Save(uuid.New(), uuid.New(), "空.pdf", "application/pdf", data)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	service := &Service{attachments: store}
	parts, warnings := service.attachmentParts([]Attachment{{
		ID: uuid.NewString(), Name: stored.Name, MIME: stored.MIME, Kind: AttachmentKindPDF,
		Status: AttachmentStatusParsed, SizeBytes: stored.SizeBytes, StoragePath: stored.StoragePath,
	}})
	if len(parts) != 0 {
		t.Fatalf("parts = %#v, want none", parts)
	}
	if len(warnings) == 0 || !strings.Contains(strings.Join(warnings, "；"), "无法读取") {
		t.Fatalf("warnings = %v", warnings)
	}
}

func buildTestPDF(objects map[int]string) []byte {
	numbers := make([]int, 0, len(objects))
	for number := range objects {
		numbers = append(numbers, number)
	}
	sort.Ints(numbers)
	var builder strings.Builder
	builder.WriteString("%PDF-1.5\n")
	for _, number := range numbers {
		fmt.Fprintf(&builder, "%d 0 obj\n%s\nendobj\n", number, objects[number])
	}
	builder.WriteString("trailer\n<< /Root 1 0 R >>\n%%EOF\n")
	return []byte(builder.String())
}

func pdfTestStream(dictionary, data string) string {
	return fmt.Sprintf("<< %s /Length %d >>\nstream\n%s\nendstream", dictionary, len(data), data)
}

func testPageImage() image.Image {
	target := image.NewRGBA(image.Rect(0, 0, 400, 400))
	for y := 0; y < 400; y++ {
		for x := 0; x < 400; x++ {
			value := uint8(0xFF)
			if x > 40 && x < 360 && y > 40 && y < 360 {
				value = 0x20
			}
			target.Set(x, y, color.RGBA{R: value, G: value, B: value, A: 0xFF})
		}
	}
	return target
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
