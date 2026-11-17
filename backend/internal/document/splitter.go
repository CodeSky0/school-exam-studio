package document

import (
	"html"
	"regexp"
	"strconv"
	"strings"
)

var (
	splitNumberLine    = regexp.MustCompile(`^\s*(?:第\s*(\d{1,3})\s*题|(\d{1,3})\s*[.、．)）]\s*)(.*)$`)
	splitOptionLine    = regexp.MustCompile(`^\s*([A-D])\s*[.、．)）]\s*(.+?)\s*$`)
	splitInlineOption  = regexp.MustCompile(`(?:^|\s)([A-D])\s*[.、．)）]\s*`)
	splitAnswerLine    = regexp.MustCompile(`^\s*(?:【答案】|参考答案|答案)\s*[:：]?\s*(.*)$`)
	splitExplainLine   = regexp.MustCompile(`^\s*(?:【解析】|解析|详解|分析)\s*[:：]?\s*(.*)$`)
	splitSectionHeader = regexp.MustCompile(`^\s*[一二三四五六七八九十]+\s*[、.．]\s*.*(选择|判断|填空|解答|计算|简答|应用|阅读|作文|作图)`)
	splitAnswerLetters = regexp.MustCompile(`[A-D]`)
	splitChoiceTag     = regexp.MustCompile(`[（(]\s*[)）]`)
)

type splitBlock struct {
	number string
	lines  []string
}

// SplitQuestions 将文档纯文本切分为题目草稿。切分保持确定性，不调用模型。
func SplitQuestions(text string, defaults Defaults) []DraftQuestion {
	lines := strings.Split(normalizeText(text), "\n")
	blocks := make([]splitBlock, 0, 16)
	sectionType := ""
	current := -1
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if current >= 0 {
				blocks[current].lines = append(blocks[current].lines, "")
			}
			continue
		}
		if splitSectionHeader.MatchString(trimmed) && !splitNumberLine.MatchString(trimmed) {
			sectionType = sectionQuestionType(trimmed)
			if current >= 0 && len(blocks[current].lines) > 0 {
				current = -1
			}
			continue
		}
		if match := splitNumberLine.FindStringSubmatch(line); match != nil {
			number := match[1]
			if number == "" {
				number = match[2]
			}
			blocks = append(blocks, splitBlock{number: number, lines: []string{match[3]}})
			current = len(blocks) - 1
			continue
		}
		if current >= 0 {
			blocks[current].lines = append(blocks[current].lines, line)
		}
	}
	noneNumbered := false
	if len(blocks) == 0 {
		noneNumbered = true
		blocks = splitByBlankLines(lines)
	}
	result := make([]DraftQuestion, 0, len(blocks))
	for index, block := range blocks {
		item, ok := buildDraft(block, index, sectionType, defaults)
		if !ok {
			continue
		}
		if noneNumbered {
			item.Warnings = append(item.Warnings, "未识别到题号，已按空行切分，请人工确认边界")
			item.Confidence = 0.55
		}
		result = append(result, item)
	}
	return result
}

func splitByBlankLines(lines []string) []splitBlock {
	blocks := make([]splitBlock, 0, 8)
	var current []string
	flush := func() {
		content := strings.TrimSpace(strings.Join(current, "\n"))
		if content == "" {
			current = nil
			return
		}
		blocks = append(blocks, splitBlock{number: strconv.Itoa(len(blocks) + 1), lines: append([]string(nil), current...)})
		current = nil
	}
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		if splitSectionHeader.MatchString(strings.TrimSpace(line)) && !splitNumberLine.MatchString(line) {
			flush()
			continue
		}
		current = append(current, line)
	}
	flush()
	return blocks
}

func sectionQuestionType(header string) string {
	switch {
	case strings.Contains(header, "多选"):
		return "multiple_choice"
	case strings.Contains(header, "选择"):
		return "single_choice"
	case strings.Contains(header, "判断"):
		return "true_false"
	case strings.Contains(header, "填空"):
		return "fill_blank"
	case strings.Contains(header, "解答"), strings.Contains(header, "计算"), strings.Contains(header, "简答"), strings.Contains(header, "应用"), strings.Contains(header, "作图"):
		return "essay"
	default:
		return ""
	}
}

func buildDraft(block splitBlock, index int, sectionType string, defaults Defaults) (DraftQuestion, bool) {
	contentLines := make([]string, 0, len(block.lines))
	optionLines := make([]string, 0, 4)
	answerText := ""
	explanationText := ""
	warnings := make([]string, 0, 2)
	for _, line := range block.lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			contentLines = append(contentLines, "")
			continue
		}
		if match := splitAnswerLine.FindStringSubmatch(trimmed); match != nil {
			value := strings.TrimSpace(match[1])
			if explanation := splitInlineExplanation(value); explanation != "" {
				if explanationText == "" {
					explanationText = explanation
				}
				value = strings.TrimSpace(strings.Split(value, "解析")[0])
			}
			if answerText == "" {
				answerText = value
			}
			continue
		}
		if match := splitExplainLine.FindStringSubmatch(trimmed); match != nil {
			if explanationText == "" {
				explanationText = strings.TrimSpace(match[1])
			}
			continue
		}
		if splitOptionLine.MatchString(trimmed) {
			optionLines = append(optionLines, trimmed)
			continue
		}
		contentLines = append(contentLines, line)
	}

	content := strings.TrimSpace(strings.Join(contentLines, "\n"))
	if strings.TrimSpace(html.UnescapeString(content)) == "" {
		return DraftQuestion{}, false
	}
	if limited, truncated := limitText(content, 6000); truncated {
		content = limited
		warnings = append(warnings, "题干超过 6000 字，已截断")
	}
	if limited, truncated := limitText(explanationText, 4000); truncated {
		explanationText = limited
		warnings = append(warnings, "解析超过 4000 字，已截断")
	}
	if limited, truncated := limitText(answerText, 1000); truncated {
		answerText = limited
	}
	options := parseOptions(optionLines, content)
	if len(options) > 4 {
		warnings = append(warnings, "选项超过 4 项，已保留 A-D")
		options = options[:4]
	}
	questionType := resolveQuestionType(sectionType, options, answerText, content)
	if len(options) > 0 && questionType != "single_choice" && questionType != "multiple_choice" {
		questionType = "single_choice"
	}
	answer, answerWarning := normalizeDraftAnswer(questionType, answerText, options)
	if answerWarning != "" {
		warnings = append(warnings, answerWarning)
	}
	if len(options) > 0 && !splitChoiceTag.MatchString(content) {
		warnings = append(warnings, "题干未检测到括号填空位，请确认选项与题干匹配")
	}
	confidence := 0.88
	if block.number == "" {
		confidence = 0.6
	}
	if strings.TrimSpace(answer) == "" {
		confidence -= 0.2
	}
	if len(options) == 0 && (sectionType == "single_choice" || sectionType == "multiple_choice") {
		confidence -= 0.15
		warnings = append(warnings, "选择题未识别到选项")
	}
	if confidence < 0.2 {
		confidence = 0.2
	}
	return DraftQuestion{
		Number:        block.number,
		Content:       plainToHTML(content),
		Answer:        answer,
		Explanation:   plainToHTML(explanationText),
		QuestionType:  questionType,
		Category:      "",
		Options:       options,
		Difficulty:    3,
		Confidence:    confidence,
		Warnings:      warnings,
		SourceExcerpt: previewText(content, 160),
	}, true
}

func splitInlineExplanation(value string) string {
	index := strings.Index(value, "解析")
	if index < 0 {
		return ""
	}
	return strings.TrimSpace(strings.TrimLeft(value[index+len("解析"):], "：: "))
}

func parseOptions(lines []string, content string) []Option {
	options := make([]Option, 0, 4)
	seen := map[string]bool{}
	for _, line := range lines {
		match := splitOptionLine.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		key := strings.ToUpper(match[1])
		if seen[key] {
			continue
		}
		seen[key] = true
		options = append(options, Option{Key: key, Content: plainToHTML(limitOptionText(strings.TrimSpace(match[2])))})
	}
	if len(options) >= 2 {
		return options
	}
	// 同一行内联选项：A. 甲 B. 乙 C. 丙 D. 丁
	matches := splitInlineOption.FindAllStringSubmatchIndex(content, -1)
	if len(matches) < 2 {
		return options
	}
	for index, match := range matches {
		key := strings.ToUpper(content[match[2]:match[3]])
		end := len(content)
		if index+1 < len(matches) {
			end = matches[index+1][0]
		}
		value := strings.TrimSpace(content[match[1]:end])
		value = strings.TrimSpace(strings.TrimLeft(value, " ,，;；"))
		if value == "" || seen[key] {
			continue
		}
		seen[key] = true
		options = append(options, Option{Key: key, Content: plainToHTML(limitOptionText(value))})
		if len(options) == 4 {
			break
		}
	}
	return options
}

func resolveQuestionType(sectionType string, options []Option, answer, content string) string {
	if len(options) >= 2 {
		if sectionType == "multiple_choice" || len(splitAnswerLetters.FindAllString(strings.ToUpper(answer), -1)) > 1 {
			return "multiple_choice"
		}
		return "single_choice"
	}
	switch sectionType {
	case "true_false":
		return "true_false"
	case "fill_blank":
		return "fill_blank"
	case "essay":
		return "essay"
	}
	if _, ok := trueFalseAnswer(answer); ok && (strings.Contains(content, "判断") || strings.Contains(content, "正确") || strings.Contains(content, "错误")) {
		return "true_false"
	}
	if strings.Contains(content, "___") || strings.Contains(content, "____") {
		return "fill_blank"
	}
	return "essay"
}

func normalizeDraftAnswer(questionType, answer string, options []Option) (string, string) {
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return "", "未识别到答案，导入前需要补充"
	}
	switch questionType {
	case "single_choice", "multiple_choice":
		letters := splitAnswerLetters.FindAllString(strings.ToUpper(answer), -1)
		valid := map[string]bool{}
		for _, option := range options {
			valid[option.Key] = true
		}
		filtered := make([]string, 0, len(letters))
		seen := map[string]bool{}
		for _, letter := range letters {
			if !valid[letter] || seen[letter] {
				continue
			}
			seen[letter] = true
			filtered = append(filtered, letter)
		}
		if len(filtered) == 0 {
			return "", "答案与选项不匹配，导入前需要核对"
		}
		if questionType == "single_choice" {
			return filtered[0], ""
		}
		return strings.Join(filtered, ","), ""
	case "true_false":
		if value, ok := trueFalseAnswer(answer); ok {
			return value, ""
		}
		return "", "判断题答案无法识别为正确/错误"
	default:
		return plainToHTML(answer), ""
	}
}

func trueFalseAnswer(value string) (string, bool) {
	normalized := strings.TrimSpace(strings.ToLower(value))
	switch normalized {
	case "true", "t", "对", "正确", "√", "是":
		return "true", true
	case "false", "f", "错", "错误", "×", "否":
		return "false", true
	default:
		return "", false
	}
}

func limitText(value string, limit int) (string, bool) {
	runes := []rune(value)
	if len(runes) <= limit {
		return value, false
	}
	return string(runes[:limit]), true
}

func limitOptionText(value string) string {
	limited, _ := limitText(value, 1000)
	return limited
}

func previewText(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "…"
}

func plainToHTML(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	lines := strings.Split(value, "\n")
	builder := strings.Builder{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		builder.WriteString("<p>")
		builder.WriteString(html.EscapeString(line))
		builder.WriteString("</p>")
	}
	return builder.String()
}
