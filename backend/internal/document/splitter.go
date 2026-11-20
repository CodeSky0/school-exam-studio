package document

import (
	"html"
	"regexp"
	"strconv"
	"strings"
)

var (
	splitNumberLine    = regexp.MustCompile(`^\s*(?:第\s*(\d{1,3})\s*题|(\d{1,3})\s*[.、．)）]\s*)(.*)$`)
	splitDecimalLine   = regexp.MustCompile(`^\s*\d{1,3}\.\d`)
	splitOptionLine    = regexp.MustCompile(`^\s*([A-D])\s*[.、．)）]\s*(.+?)\s*$`)
	splitInlineOption  = regexp.MustCompile(`(?:^|\s)([A-D])\s*[.、．)）]\s*`)
	splitAnswerLine    = regexp.MustCompile(`^\s*(?:【答案】|参考答案|答案)\s*[:：]?\s*(.*)$`)
	splitExplainLine   = regexp.MustCompile(`^\s*(?:【解析】|解析|详解|分析)\s*[:：]?\s*(.*)$`)
	splitSectionHeader = regexp.MustCompile(`^\s*[一二三四五六七八九十]+\s*[、.．]\s*.*(选择|判断|填空|解答|计算|简答|应用|阅读|作文|作图)`)
	splitAnswerHeader  = regexp.MustCompile(`^\s*(?:【\s*)?(?:参考答案|答案)(?:[与及]?(?:解析|详解))?\s*(?:】)?\s*[:：]?\s*$`)
	splitAnswerItem    = regexp.MustCompile(`^\s*(\d{1,3})\s*[.、．)）]\s*(.+?)\s*$`)
	splitAnswerRange   = regexp.MustCompile(`^\s*(\d{1,3})\s*[-—~～至]\s*(\d{1,3})\s*[:：]?\s*([A-Da-d]{2,})\s*$`)
	splitAnswerLetters = regexp.MustCompile(`[A-D]`)
	splitChoiceTag     = regexp.MustCompile(`[（(]\s*[)）]`)
)

type splitBlock struct {
	number      string
	sectionType string
	lines       []string
}

type splitAnswer struct {
	answer      string
	explanation string
}

// SplitQuestions 将文档纯文本切分为题目草稿。切分保持确定性，不调用模型。
func SplitQuestions(text string, defaults Defaults) []DraftQuestion {
	lines := strings.Split(normalizeText(text), "\n")
	blocks := make([]splitBlock, 0, 16)
	answers := make(map[string]splitAnswer)
	sectionType := ""
	current := -1
	inAnswers := false
	lastAnswerNumber := ""
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if !inAnswers && current >= 0 {
				blocks[current].lines = append(blocks[current].lines, "")
			}
			continue
		}
		if splitAnswerHeader.MatchString(trimmed) {
			inAnswers = true
			lastAnswerNumber = ""
			current = -1
			continue
		}
		if inAnswers {
			if splitSectionHeader.MatchString(trimmed) && !splitNumberLine.MatchString(trimmed) {
				lastAnswerNumber = ""
				continue
			}
			if matched := parseAnswerSectionLine(trimmed, answers); matched {
				if match := splitAnswerItem.FindStringSubmatch(trimmed); match != nil {
					lastAnswerNumber = match[1]
				}
				continue
			}
			if lastAnswerNumber != "" {
				entry := answers[lastAnswerNumber]
				if entry.explanation == "" {
					entry.explanation = trimmed
				} else {
					entry.explanation += "\n" + trimmed
				}
				answers[lastAnswerNumber] = entry
			}
			continue
		}
		if splitSectionHeader.MatchString(trimmed) && !splitNumberLine.MatchString(trimmed) {
			sectionType = sectionQuestionType(trimmed)
			current = -1
			continue
		}
		if splitDecimalLine.MatchString(line) {
			if current >= 0 {
				blocks[current].lines = append(blocks[current].lines, line)
			}
			continue
		}
		if match := splitNumberLine.FindStringSubmatch(line); match != nil {
			number := match[1]
			if number == "" {
				number = match[2]
			}
			blocks = append(blocks, splitBlock{number: number, sectionType: sectionType, lines: []string{match[3]}})
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
		item, ok := buildDraft(block, index, defaults)
		if !ok {
			continue
		}
		if noneNumbered {
			item.Warnings = append(item.Warnings, "未识别到题号，已按空行切分，请人工确认边界")
			item.Confidence = 0.55
		}
		result = append(result, item)
	}
	applyAnswerSection(result, answers)
	return result
}

func parseAnswerSectionLine(line string, answers map[string]splitAnswer) bool {
	if match := splitAnswerRange.FindStringSubmatch(line); match != nil {
		start, startErr := strconv.Atoi(match[1])
		end, endErr := strconv.Atoi(match[2])
		letters := []rune(strings.ToUpper(match[3]))
		if startErr == nil && endErr == nil && end >= start && len(letters) > 0 {
			for number, index := start, 0; number <= end && index < len(letters); number, index = number+1, index+1 {
				answers[strconv.Itoa(number)] = splitAnswer{answer: string(letters[index])}
			}
			return true
		}
	}
	match := splitAnswerItem.FindStringSubmatch(line)
	if match == nil {
		return false
	}
	answer, explanation := splitAnswerAndExplanation(match[2])
	if answer == "" {
		return false
	}
	answers[match[1]] = splitAnswer{answer: answer, explanation: explanation}
	return true
}

func applyAnswerSection(items []DraftQuestion, answers map[string]splitAnswer) {
	for index := range items {
		entry, ok := answers[items[index].Number]
		if !ok {
			continue
		}
		if strings.TrimSpace(items[index].Answer) == "" {
			answer, warning := normalizeDraftAnswer(items[index].QuestionType, entry.answer, items[index].Options)
			if answer != "" {
				items[index].Answer = answer
				items[index].Warnings = removeDraftWarning(items[index].Warnings, "未识别到答案")
				items[index].Warnings = removeDraftWarning(items[index].Warnings, "答案与选项不匹配")
				if items[index].Confidence < 0.95 {
					items[index].Confidence += 0.2
					if items[index].Confidence > 0.95 {
						items[index].Confidence = 0.95
					}
				}
			} else if warning != "" {
				items[index].Warnings = append(items[index].Warnings, warning)
			}
		}
		if strings.TrimSpace(items[index].Explanation) == "" && entry.explanation != "" {
			items[index].Explanation = plainToHTML(entry.explanation)
		}
	}
}

func removeDraftWarning(warnings []string, keyword string) []string {
	result := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		if strings.Contains(warning, keyword) {
			continue
		}
		result = append(result, warning)
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

func buildDraft(block splitBlock, index int, defaults Defaults) (DraftQuestion, bool) {
	sectionType := block.sectionType
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
			value, explanation := splitAnswerAndExplanation(strings.TrimSpace(match[1]))
			if explanation != "" && explanationText == "" {
				explanationText = explanation
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

func splitAnswerAndExplanation(value string) (string, string) {
	value = strings.TrimSpace(value)
	for _, marker := range []string{"【解析】", "【详解】", "解析：", "解析:", "解析", "详解：", "详解:", "详解"} {
		index := strings.Index(value, marker)
		if index < 0 {
			continue
		}
		answer := strings.TrimSpace(value[:index])
		explanation := strings.TrimSpace(strings.TrimLeft(value[index+len(marker):], " ：:】"))
		return answer, explanation
	}
	return value, ""
}

func parseOptions(lines []string, content string) []Option {
	options := make([]Option, 0, 4)
	seen := map[string]bool{}
	source := strings.Join(lines, "\n")
	if strings.TrimSpace(source) == "" {
		source = content
	}
	// 优先解析同一行内的多个选项：A. 甲 B. 乙 C. 丙 D. 丁
	matches := splitInlineOption.FindAllStringSubmatchIndex(source, -1)
	if len(matches) >= 2 {
		for index, match := range matches {
			key := strings.ToUpper(source[match[2]:match[3]])
			if seen[key] {
				continue
			}
			end := len(source)
			if index+1 < len(matches) {
				end = matches[index+1][0]
			}
			value := strings.TrimSpace(source[match[1]:end])
			value = strings.TrimSpace(strings.TrimLeft(value, " ,，;；"))
			if value == "" {
				continue
			}
			seen[key] = true
			options = append(options, Option{Key: key, Content: plainToHTML(limitOptionText(value))})
			if len(options) == 4 {
				break
			}
		}
		if len(options) >= 2 {
			return options
		}
	}
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
