package document

import (
	"bytes"
	"compress/zlib"
	"encoding/ascii85"
	"fmt"
	"sort"
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

func assemblePDF(objects map[int]string) []byte {
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

func pdfStreamObject(dictionary, data string) string {
	return fmt.Sprintf("<< %s /Length %d >>\nstream\n%s\nendstream", dictionary, len(data), data)
}

func zlibCompress(t *testing.T, data []byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zlib.NewWriter(&buffer)
	if _, err := writer.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func pdfBaseObjects(contentObject int, pageResources string) map[int]string {
	return map[int]string{
		1: "<< /Type /Catalog /Pages 2 0 R >>",
		2: "<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		3: fmt.Sprintf("<< /Type /Page /Parent 2 0 R /Resources << %s >> /Contents %d 0 R >>", pageResources, contentObject),
	}
}

func buildPDFWithObjectStream(t *testing.T) []byte {
	t.Helper()
	cmap := "/CIDInit /ProcSet findresource begin\nbegincmap\n1 begincodespacerange\n<0000> <FFFF>\nendcodespacerange\n1 beginbfchar\n<0001> <9898>\nendbfchar\nendcmap\n"
	content := "BT /F1 12 Tf 72 720 Td <0001> Tj ET"
	font := "<< /Type /Font /Subtype /Type0 /BaseFont /Test /Encoding /Identity-H /ToUnicode 5 0 R >>"
	header := "4 0 "
	compressed := zlibCompress(t, []byte(header+font))
	objects := pdfBaseObjects(6, "/Font << /F1 4 0 R >>")
	objects[5] = pdfStreamObject("", cmap)
	objects[6] = pdfStreamObject("", content)
	objects[7] = pdfStreamObject("/Type /ObjStm /N 1 /First 4 /Filter /FlateDecode", string(compressed))
	return assemblePDF(objects)
}

func TestExtractPDFReadsFontFromObjectStream(t *testing.T) {
	result, err := Extract("compressed.pdf", "application/pdf", buildPDFWithObjectStream(t))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if !strings.Contains(result.Text, "题") {
		t.Fatalf("expected text from compressed object stream, got %q", result.Text)
	}
}

func TestExtractPDFUsesPerFontUnicodeMap(t *testing.T) {
	cmapOne := "/CIDInit /ProcSet findresource begin\nbegincmap\n1 begincodespacerange\n<0000> <FFFF>\nendcodespacerange\n1 beginbfchar\n<0001> <7532>\nendbfchar\nendcmap\n"
	cmapTwo := "/CIDInit /ProcSet findresource begin\nbegincmap\n1 begincodespacerange\n<0000> <FFFF>\nendcodespacerange\n1 beginbfchar\n<0001> <4E59>\nendbfchar\nendcmap\n"
	content := "BT /F1 12 Tf 72 720 Td <0001> Tj /F2 12 Tf <0001> Tj ET"
	objects := pdfBaseObjects(6, "/Font << /F1 4 0 R /F2 5 0 R >>")
	objects[4] = "<< /Type /Font /Subtype /Type0 /BaseFont /One /Encoding /Identity-H /ToUnicode 7 0 R >>"
	objects[5] = "<< /Type /Font /Subtype /Type0 /BaseFont /Two /Encoding /Identity-H /ToUnicode 8 0 R >>"
	objects[6] = pdfStreamObject("", content)
	objects[7] = pdfStreamObject("", cmapOne)
	objects[8] = pdfStreamObject("", cmapTwo)
	result, err := Extract("fonts.pdf", "application/pdf", assemblePDF(objects))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if !strings.Contains(result.Text, "甲乙") {
		t.Fatalf("expected per-font decoding, got %q", result.Text)
	}
}

func TestExtractPDFReadsAllBFCharBlocks(t *testing.T) {
	cmap := "/CIDInit /ProcSet findresource begin\nbegincmap\n1 begincodespacerange\n<0000> <FFFF>\nendcodespacerange\n" +
		"1 beginbfchar\n<0001> <4E2D>\nendbfchar\n1 beginbfchar\n<0002> <6587>\nendbfchar\nendcmap\n"
	content := "BT /F1 12 Tf 72 720 Td <0001> Tj <0002> Tj ET"
	objects := pdfBaseObjects(6, "/Font << /F1 4 0 R >>")
	objects[4] = "<< /Type /Font /Subtype /Type0 /BaseFont /Test /Encoding /Identity-H /ToUnicode 7 0 R >>"
	objects[6] = pdfStreamObject("", content)
	objects[7] = pdfStreamObject("", cmap)
	result, err := Extract("blocks.pdf", "application/pdf", assemblePDF(objects))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if !strings.Contains(result.Text, "中文") {
		t.Fatalf("expected mappings from every bfchar block, got %q", result.Text)
	}
}

func TestExtractPDFKeepsMappedTextWhenCMapIsPartial(t *testing.T) {
	cmap := "/CIDInit /ProcSet findresource begin\nbegincmap\n1 begincodespacerange\n<0000> <FFFF>\nendcodespacerange\n1 beginbfchar\n<0001> <4E2D>\nendbfchar\nendcmap\n"
	content := "BT /F1 12 Tf 72 720 Td <0001> Tj <0002> Tj ET"
	objects := pdfBaseObjects(6, "/Font << /F1 4 0 R >>")
	objects[4] = "<< /Type /Font /Subtype /Type0 /BaseFont /Test /Encoding /Identity-H /ToUnicode 7 0 R >>"
	objects[6] = pdfStreamObject("", content)
	objects[7] = pdfStreamObject("", cmap)
	result, err := Extract("partial.pdf", "application/pdf", assemblePDF(objects))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if !strings.Contains(result.Text, "中") {
		t.Fatalf("expected mapped text to survive partial CMap, got %q", result.Text)
	}
}

func TestExtractPDFFallsBackToIdentityUnicode(t *testing.T) {
	content := "BT /F1 12 Tf 72 720 Td <4F60> Tj <597D> Tj ET"
	objects := pdfBaseObjects(6, "/Font << /F1 4 0 R >>")
	objects[4] = "<< /Type /Font /Subtype /Type0 /BaseFont /Sans /Encoding /Identity-H >>"
	objects[6] = pdfStreamObject("", content)
	result, err := Extract("identity.pdf", "application/pdf", assemblePDF(objects))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if !strings.Contains(result.Text, "你好") {
		t.Fatalf("expected identity CID fallback, got %q", result.Text)
	}
}

func TestExtractPDFDecodesUCS2CIDFont(t *testing.T) {
	content := "BT /F1 12 Tf 72 720 Td <4EFB> Tj <610F> Tj ET"
	objects := pdfBaseObjects(6, "/Font << /F1 4 0 R >>")
	objects[4] = "<< /Type /Font /Subtype /Type0 /BaseFont /STSong-Light /Encoding /UniGB-UCS2-H >>"
	objects[6] = pdfStreamObject("", content)
	result, err := Extract("ucs2.pdf", "application/pdf", assemblePDF(objects))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if !strings.Contains(result.Text, "任意") {
		t.Fatalf("expected UCS2 encoding decoding, got %q", result.Text)
	}
	if len(result.Warnings) != 0 {
		for _, warning := range result.Warnings {
			if strings.Contains(warning, "Unicode 映射") {
				t.Fatalf("warnings = %v", result.Warnings)
			}
		}
	}
}

func TestExtractPDFSupportsFilterChains(t *testing.T) {
	content := "BT /F1 12 Tf 72 720 Td (Filtered text) Tj ET"
	compressed := zlibCompress(t, []byte(content))
	encoded := make([]byte, ascii85.MaxEncodedLen(len(compressed)))
	length := ascii85.Encode(encoded, compressed)
	objects := pdfBaseObjects(6, "/Font << /F1 4 0 R >>")
	objects[4] = "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"
	objects[6] = pdfStreamObject("/Filter [/ASCII85Decode /FlateDecode]", string(encoded[:length]))
	result, err := Extract("filters.pdf", "application/pdf", assemblePDF(objects))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if !strings.Contains(result.Text, "Filtered text") {
		t.Fatalf("expected filter chain decoding, got %q", result.Text)
	}
}
