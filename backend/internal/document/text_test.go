package document

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

func buildTestDocx(t *testing.T, paragraphs ...string) []byte {
	t.Helper()
	buffer := &bytes.Buffer{}
	writer := zip.NewWriter(buffer)
	files := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="xml" ContentType="application/xml"/></Types>`,
		"_rels/.rels":         `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`,
	}
	builder := strings.Builder{}
	builder.WriteString(`<?xml version="1.0" encoding="UTF-8"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`)
	for _, paragraph := range paragraphs {
		builder.WriteString(`<w:p><w:r><w:t>`)
		builder.WriteString(paragraph)
		builder.WriteString(`</w:t></w:r></w:p>`)
	}
	builder.WriteString(`</w:body></w:document>`)
	files["word/document.xml"] = builder.String()
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestExtractDocxKeepsParagraphs(t *testing.T) {
	data := buildTestDocx(t, "1. 计算 1+1", "答案：2")
	result, err := Extract("exam.docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", data)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if result.Kind != "docx" {
		t.Fatalf("kind = %q", result.Kind)
	}
	if !strings.Contains(result.Text, "1. 计算 1+1") || !strings.Contains(result.Text, "答案：2") {
		t.Fatalf("unexpected text: %q", result.Text)
	}
}

func TestDetectKindRejectsExecutable(t *testing.T) {
	if _, err := DetectKind("virus.exe", "application/octet-stream", []byte("MZ\x90\x00")); err == nil {
		t.Fatal("expected unsupported error")
	}
}

func TestExtractPlainTextNormalizesNewlines(t *testing.T) {
	result, err := Extract("notes.txt", "text/plain", []byte("第一行\r\n\r\n\r\n第二行"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "第一行\n\n第二行" {
		t.Fatalf("text = %q", result.Text)
	}
}

func TestExtractTruncatesLongText(t *testing.T) {
	long := strings.Repeat("题", MaxTextChars+10)
	result, err := Extract("long.txt", "text/plain", []byte(long))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Truncated {
		t.Fatal("expected truncation")
	}
	if len([]rune(result.Text)) != MaxTextChars {
		t.Fatalf("length = %d", len([]rune(result.Text)))
	}
}
