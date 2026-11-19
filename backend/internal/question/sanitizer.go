package question

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/net/html"
)

const (
	maxPlainTextRunes  = 30
	maxTags            = 10
	maxTagRunes        = 30
	maxOptions         = 4
	maxKnowledgePoints = 8
	maxCategoryRunes   = 60
)

var mathDataType = regexp.MustCompile(`^(inlineMath|blockMath|inline-math|block-math)$`)

type Sanitizer struct {
	policy *bluemonday.Policy
}

func NewSanitizer() *Sanitizer {
	policy := bluemonday.NewPolicy()
	policy.AllowElements(
		"p", "br", "hr", "div", "span",
		"h1", "h2", "h3", "h4", "h5", "h6",
		"strong", "b", "em", "i", "u", "s", "del", "sub", "sup",
		"code", "pre", "blockquote",
		"ul", "ol", "li",
		"table", "caption", "colgroup", "col", "thead", "tbody", "tfoot", "tr", "th", "td",
	)
	policy.AllowAttrs("data-type").Matching(mathDataType).OnElements("span", "div")
	policy.AllowAttrs("data-latex").OnElements("span", "div")
	policy.AllowAttrs("colspan", "rowspan").Matching(regexp.MustCompile(`^[1-9][0-9]{0,2}$`)).OnElements("th", "td")
	return &Sanitizer{policy: policy}
}

func (s *Sanitizer) Sanitize(value string) string {
	return s.policy.Sanitize(value)
}

func (s *Sanitizer) Normalize(input Input) (Input, error) {
	var err error
	if input.Subject, err = normalizePlainText(input.Subject, "subject", true); err != nil {
		return Input{}, err
	}
	if input.Grade, err = normalizePlainText(input.Grade, "grade", true); err != nil {
		return Input{}, err
	}
	input.QuestionType = strings.TrimSpace(input.QuestionType)
	if !ValidType(input.QuestionType) {
		return Input{}, validationError("question_type", "题型不受支持")
	}
	if input.Difficulty < 1 || input.Difficulty > 5 {
		return Input{}, validationError("difficulty", "难度必须为1-5")
	}
	if input.Tags, err = normalizeTags(input.Tags); err != nil {
		return Input{}, err
	}
	if input.Category, err = normalizeCategory(input.Category); err != nil {
		return Input{}, err
	}
	if input.KnowledgePoints, err = normalizeKnowledgePoints(input.KnowledgePoints); err != nil {
		return Input{}, err
	}
	input.Tags = dropDuplicateMetadataTags(input.Tags, input.Category, input.KnowledgePoints)
	if input.Content, err = s.normalizeRichText(input.Content, "content", true); err != nil {
		return Input{}, err
	}
	if input.Options, err = s.normalizeOptions(input.Options); err != nil {
		return Input{}, err
	}
	if input.Answer, err = s.normalizeRichText(input.Answer, "answer", true); err != nil {
		return Input{}, err
	}
	if input.Explanation, err = s.normalizeRichText(input.Explanation, "explanation", false); err != nil {
		return Input{}, err
	}
	if err := normalizeAnswer(&input); err != nil {
		return Input{}, err
	}
	return input, nil
}

func (s *Sanitizer) normalizeRichText(value, field string, required bool) (string, error) {
	if !utf8.ValidString(value) {
		return "", validationError(field, "字段包含无效字符")
	}
	value = normalizeMathMarkup(strings.TrimSpace(s.Sanitize(value)))
	if required && !richTextPresent(value) {
		return "", validationError(field, "字段不能为空")
	}
	return value, nil
}

func richTextPresent(value string) bool {
	document, err := html.Parse(strings.NewReader(value))
	if err != nil {
		return false
	}
	var present func(*html.Node) bool
	present = func(node *html.Node) bool {
		if node.Type == html.TextNode && strings.TrimSpace(node.Data) != "" {
			return true
		}
		if node.Type == html.ElementNode && (node.Data == "span" || node.Data == "div") {
			mathNode := false
			latexPresent := false
			for _, attribute := range node.Attr {
				if attribute.Key == "data-type" && isMathDataType(attribute.Val) {
					mathNode = true
				}
				if attribute.Key == "data-latex" && strings.TrimSpace(attribute.Val) != "" {
					latexPresent = true
				}
			}
			if mathNode && latexPresent {
				return true
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if present(child) {
				return true
			}
		}
		return false
	}
	return present(document)
}

func (s *Sanitizer) normalizeOptions(options []Option) ([]Option, error) {
	if len(options) > maxOptions {
		return nil, validationError("options", "选项最多为A-D四项")
	}
	normalized := make([]Option, 0, len(options))
	keys := make(map[string]struct{}, len(options))
	for index, option := range options {
		if option.Key < "A" || option.Key > "D" || len(option.Key) != 1 {
			return nil, validationError(fmt.Sprintf("options.%d.key", index), "选项键必须为A-D")
		}
		if _, exists := keys[option.Key]; exists {
			return nil, validationError(fmt.Sprintf("options.%d.key", index), "选项键不能重复")
		}
		keys[option.Key] = struct{}{}
		content, err := s.normalizeRichText(option.Content, fmt.Sprintf("options.%d.content", index), true)
		if err != nil {
			return nil, err
		}
		normalized = append(normalized, Option{Key: option.Key, Content: content})
	}
	return normalized, nil
}

func normalizeTags(tags []string) ([]string, error) {
	if len(tags) > maxTags {
		return nil, validationError("tags", "标签最多10个")
	}
	normalized := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		length := utf8.RuneCountInString(tag)
		if !utf8.ValidString(tag) || length < 1 || length > maxTagRunes {
			return nil, validationError("tags", "每个标签长度必须为1-30个字符")
		}
		for _, r := range tag {
			if unicode.IsControl(r) {
				return nil, validationError("tags", "标签不能包含控制字符")
			}
		}
		key := metadataKey(tag)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, tag)
	}
	return normalized, nil
}

// metadataKey 用于判断标签、分类与知识点是否指向同一内容：忽略大小写与所有空白差异。
func metadataKey(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), ""))
}

// dropDuplicateMetadataTags 去掉已经在分类或知识点字段中出现过的标签，避免同一内容在多个字段重复展示。
func dropDuplicateMetadataTags(tags []string, category string, knowledgePoints []string) []string {
	if len(tags) == 0 {
		return tags
	}
	excluded := make(map[string]struct{}, len(knowledgePoints)+1)
	if key := metadataKey(category); key != "" {
		excluded[key] = struct{}{}
	}
	for _, point := range knowledgePoints {
		if key := metadataKey(point); key != "" {
			excluded[key] = struct{}{}
		}
	}
	if len(excluded) == 0 {
		return tags
	}
	result := make([]string, 0, len(tags))
	for _, tag := range tags {
		if _, exists := excluded[metadataKey(tag)]; exists {
			continue
		}
		result = append(result, tag)
	}
	return result
}

func normalizeCategory(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > maxCategoryRunes {
		return "", validationError("category", "分类不能超过60个字符")
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return "", validationError("category", "分类不能包含控制字符")
		}
	}
	return value, nil
}

func normalizeKnowledgePoints(values []string) ([]string, error) {
	if len(values) > maxKnowledgePoints {
		return nil, validationError("knowledge_points", "知识点最多8个")
	}
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		length := utf8.RuneCountInString(value)
		if !utf8.ValidString(value) || length < 1 || length > maxTagRunes {
			return nil, validationError("knowledge_points", "每个知识点长度必须为1-30个字符")
		}
		for _, r := range value {
			if unicode.IsControl(r) {
				return nil, validationError("knowledge_points", "知识点不能包含控制字符")
			}
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized, nil
}

// NormalizeTags / NormalizeCategory / NormalizeKnowledgePoints 供 AI 写操作复用同一套约束。
func NormalizeTags(values []string) ([]string, error) {
	return normalizeTags(values)
}

func NormalizeCategory(value string) (string, error) {
	return normalizeCategory(value)
}

func NormalizeKnowledgePoints(values []string) ([]string, error) {
	return normalizeKnowledgePoints(values)
}

// NormalizeExplanation 对 AI 生成的解析执行与题目解析相同的富文本消毒。
func (s *Sanitizer) NormalizeExplanation(value string) (string, error) {
	return s.normalizeRichText(value, "explanation", false)
}

// ValidateAnswer 校验 AI 建议答案是否符合题型约束，并返回规范化答案。
func ValidateAnswer(questionType string, options []Option, answer string) (string, error) {
	input := Input{QuestionType: questionType, Options: options, Answer: answer}
	if err := normalizeAnswer(&input); err != nil {
		return "", err
	}
	return input.Answer, nil
}

// ContentHash 基于可见题干与选项生成稳定哈希，用于重复题检测。
func ContentHash(input Input) string {
	parts := []string{visibleText(input.Content)}
	for _, option := range input.Options {
		parts = append(parts, option.Key+":"+visibleText(option.Content))
	}
	normalized := strings.Join(strings.Fields(strings.ToLower(strings.Join(parts, "|"))), " ")
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

var numberPattern = regexp.MustCompile(`\d+(?:\.\d+)?`)

// ShiftRichTextNumbers 仅改写文本节点中的数字，保留富文本结构与公式节点。
func ShiftRichTextNumbers(value string, delta int) (string, bool) {
	document, err := html.Parse(strings.NewReader(value))
	if err != nil {
		return value, false
	}
	changed := false
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			replaced := numberPattern.ReplaceAllStringFunc(node.Data, func(match string) string {
				if strings.Contains(match, ".") {
					parsed, err := strconv.ParseFloat(match, 64)
					if err != nil {
						return match
					}
					changed = true
					return strconv.FormatFloat(math.Max(1, parsed+float64(delta)), 'f', -1, 64)
				}
				parsed, err := strconv.Atoi(match)
				if err != nil {
					return match
				}
				changed = true
				return strconv.Itoa(int(math.Max(1, float64(parsed+delta))))
			})
			node.Data = replaced
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(document)
	if !changed {
		return value, false
	}
	var builder strings.Builder
	if err := html.Render(&builder, document); err != nil {
		return value, false
	}
	return builder.String(), true
}

func visibleText(value string) string {
	document, err := html.Parse(strings.NewReader(value))
	if err != nil {
		return strings.TrimSpace(value)
	}
	var builder strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			builder.WriteString(node.Data)
			builder.WriteString(" ")
		}
		if node.Type == html.ElementNode {
			for _, attribute := range node.Attr {
				if attribute.Key == "data-latex" && strings.TrimSpace(attribute.Val) != "" {
					builder.WriteString(attribute.Val)
					builder.WriteString(" ")
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(document)
	return strings.TrimSpace(builder.String())
}

func isMathDataType(value string) bool {
	switch value {
	case "inlineMath", "blockMath", "inline-math", "block-math":
		return true
	default:
		return false
	}
}

func normalizeAnswer(input *Input) error {
	optionKeys := make(map[string]struct{}, len(input.Options))
	for _, option := range input.Options {
		optionKeys[option.Key] = struct{}{}
	}
	if input.QuestionType != TypeSingleChoice && input.QuestionType != TypeMultipleChoice && len(input.Options) > 0 {
		return validationError("options", "当前题型不应包含选项")
	}
	switch input.QuestionType {
	case TypeSingleChoice:
		if len(input.Options) < 2 {
			return validationError("options", "单选题至少需要2个选项")
		}
		answer := strings.TrimSpace(input.Answer)
		if _, exists := optionKeys[answer]; len(answer) != 1 || !exists {
			return validationError("answer", "单选题答案必须是一个合法选项键")
		}
		input.Answer = answer
	case TypeMultipleChoice:
		if len(input.Options) < 2 {
			return validationError("options", "多选题至少需要2个选项")
		}
		parts := strings.Split(input.Answer, ",")
		seen := make(map[string]struct{}, len(parts))
		answers := make([]string, 0, len(parts))
		for _, part := range parts {
			answer := strings.TrimSpace(part)
			_, legal := optionKeys[answer]
			_, duplicate := seen[answer]
			if len(answer) != 1 || !legal || duplicate {
				return validationError("answer", "多选题答案必须为逗号分隔的唯一合法选项键")
			}
			seen[answer] = struct{}{}
			answers = append(answers, answer)
		}
		input.Answer = strings.Join(answers, ",")
	case TypeTrueFalse:
		answer := strings.TrimSpace(input.Answer)
		if answer != "true" && answer != "false" {
			return validationError("answer", "判断题答案必须为true或false")
		}
		input.Answer = answer
	}
	return nil
}

func normalizePlainText(value, field string, required bool) (string, error) {
	value = strings.TrimSpace(value)
	length := utf8.RuneCountInString(value)
	if !utf8.ValidString(value) || (required && length == 0) || length > maxPlainTextRunes {
		return "", validationError(field, "字段长度必须为1-30个有效字符")
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return "", validationError(field, "字段不能包含控制字符")
		}
	}
	return value, nil
}
