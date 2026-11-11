package question

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/net/html"
)

const (
	maxPlainTextRunes = 30
	maxTags           = 10
	maxTagRunes       = 30
	maxOptions        = 4
)

var mathDataType = regexp.MustCompile(`^(inlineMath|blockMath)$`)

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
	value = strings.TrimSpace(s.Sanitize(value))
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
				if attribute.Key == "data-type" && (attribute.Val == "inlineMath" || attribute.Val == "blockMath") {
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
		if _, exists := seen[tag]; exists {
			continue
		}
		seen[tag] = struct{}{}
		normalized = append(normalized, tag)
	}
	return normalized, nil
}

func normalizeAnswer(input *Input) error {
	optionKeys := make(map[string]struct{}, len(input.Options))
	for _, option := range input.Options {
		optionKeys[option.Key] = struct{}{}
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
