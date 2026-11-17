package document

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

const MaxTextChars = 200000

var (
	ErrUnsupported = errors.New("document type not supported")
	ErrEmpty       = errors.New("document has no readable text")
)

// Result 是文档抽取的纯文本结果。
type Result struct {
	Text      string
	Kind      string
	Pages     int
	Truncated bool
	Warnings  []string
}

type Option struct {
	Key     string `json:"key"`
	Content string `json:"content"`
}

type Defaults struct {
	Subject string `json:"subject"`
	Grade   string `json:"grade"`
}

// DraftQuestion 是从文档中切分出的题目候选，尚未写入题库。
type DraftQuestion struct {
	Number        string   `json:"number"`
	Content       string   `json:"content"`
	Answer        string   `json:"answer"`
	Explanation   string   `json:"explanation"`
	QuestionType  string   `json:"question_type"`
	Category      string   `json:"category"`
	Options       []Option `json:"options"`
	Difficulty    int      `json:"difficulty"`
	Confidence    float64  `json:"confidence"`
	Warnings      []string `json:"warnings,omitempty"`
	SourceExcerpt string   `json:"source_excerpt,omitempty"`
}

// Extract 根据文件名与 MIME 抽取文本。图片不在此处理，调用方应先走 OCR。
func Extract(name, mime string, data []byte) (Result, error) {
	if len(data) == 0 {
		return Result{}, ErrEmpty
	}
	kind, err := DetectKind(name, mime, data)
	if err != nil {
		return Result{}, err
	}
	var result Result
	switch kind {
	case "pdf":
		result, err = extractPDF(data)
	case "docx":
		result, err = extractDOCX(data)
	case "text":
		result = extractPlainText(data)
	default:
		return Result{}, ErrUnsupported
	}
	if err != nil {
		return Result{}, err
	}
	result.Kind = kind
	result.Text, result.Truncated = truncateText(normalizeText(result.Text))
	if strings.TrimSpace(result.Text) == "" {
		return Result{}, ErrEmpty
	}
	return result, nil
}

func extractPlainText(data []byte) Result {
	if !utf8.Valid(data) {
		return Result{
			Text:     strings.ToValidUTF8(string(data), ""),
			Warnings: []string{"文件不是有效的 UTF-8 文本，部分字符可能丢失"},
		}
	}
	return Result{Text: string(data)}
}

func truncateText(value string) (string, bool) {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= MaxTextChars {
		return value, false
	}
	return string(runes[:MaxTextChars]), true
}

func normalizeText(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	value = strings.ReplaceAll(value, "\u0000", "")
	lines := strings.Split(value, "\n")
	output := make([]string, 0, len(lines))
	blank := 0
	for _, line := range lines {
		line = strings.TrimRight(line, " \t")
		if strings.TrimSpace(line) == "" {
			blank++
			if blank > 1 {
				continue
			}
			output = append(output, "")
			continue
		}
		blank = 0
		output = append(output, line)
	}
	return strings.TrimSpace(strings.Join(output, "\n"))
}

// DetectKind 以内容特征为准，扩展名和 MIME 仅作为补充。
func DetectKind(name, mime string, data []byte) (string, error) {
	lowerName := strings.ToLower(strings.TrimSpace(name))
	lowerMime := strings.ToLower(strings.TrimSpace(mime))
	switch {
	case len(data) >= 5 && string(data[:5]) == "%PDF-":
		return "pdf", nil
	case len(data) >= 4 && data[0] == 'P' && data[1] == 'K' && data[2] == 0x03 && data[3] == 0x04:
		if strings.Contains(lowerName, ".docx") || strings.Contains(lowerMime, "wordprocessingml") || hasZipEntry(data, "word/document.xml") {
			return "docx", nil
		}
		return "", fmt.Errorf("不支持的压缩包类型：仅支持 DOCX")
	case strings.HasSuffix(lowerName, ".pdf") || lowerMime == "application/pdf":
		return "", fmt.Errorf("PDF 文件头不正确")
	case strings.HasSuffix(lowerName, ".docx") || strings.Contains(lowerMime, "wordprocessingml"):
		return "", fmt.Errorf("DOCX 文件头不正确")
	case strings.HasSuffix(lowerName, ".txt"), strings.HasSuffix(lowerName, ".md"), strings.HasSuffix(lowerName, ".csv"),
		lowerMime == "text/plain", lowerMime == "text/markdown", lowerMime == "text/csv":
		return "text", nil
	}
	return "", ErrUnsupported
}
