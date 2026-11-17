package document

import (
	"fmt"
	"strings"
	"testing"
)

func buildTestPDF() []byte {
	cmap := "/CIDInit /ProcSet findresource begin\n12 dict begin\nbegincmap\n1 begincodespacerange\n<0000> <FFFF>\nendcodespacerange\n1 beginbfchar\n<0001> <9898>\nendbfchar\nendcmap\nend\n"
	content := "BT /F1 12 Tf 72 720 Td <0001> Tj 0 -18 Td (Hello 2+2) Tj ET"
	var builder strings.Builder
	builder.WriteString("%PDF-1.4\n")
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /Resources << /Font << /F1 4 0 R >> >> /Contents 6 0 R >>",
		"<< /Type /Font /Subtype /Type0 /BaseFont /Test /Encoding /Identity-H /ToUnicode 5 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(cmap), cmap),
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
	}
	for index, object := range objects {
		builder.WriteString(fmt.Sprintf("%d 0 obj\n%s\nendobj\n", index+1, object))
	}
	builder.WriteString("trailer\n<< /Root 1 0 R >>\n%%EOF\n")
	return []byte(builder.String())
}

func TestExtractPDFReadsTextAndUnicodeMap(t *testing.T) {
	result, err := Extract("paper.pdf", "application/pdf", buildTestPDF())
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if result.Kind != "pdf" {
		t.Fatalf("kind = %q", result.Kind)
	}
	if !strings.Contains(result.Text, "题") {
		t.Fatalf("expected mapped Chinese text, got %q", result.Text)
	}
	if !strings.Contains(result.Text, "Hello 2+2") {
		t.Fatalf("expected literal text, got %q", result.Text)
	}
	if result.Pages != 1 {
		t.Fatalf("pages = %d", result.Pages)
	}
}

func TestExtractPDFScannerWarning(t *testing.T) {
	result, err := Extract("scan.pdf", "application/pdf", buildTestPDF())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Warnings) == 0 {
		t.Fatal("expected warning for short text")
	}
}
