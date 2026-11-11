package importer

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"school-exam-studio/backend/internal/question"
)

var expectedHeader = []string{"题型", "年级", "学科", "难度", "知识点标签", "题干", "选项A", "选项B", "选项C", "选项D", "答案", "解析"}

var questionTypes = map[string]string{
	"单选题": question.TypeSingleChoice,
	"多选题": question.TypeMultipleChoice,
	"判断题": question.TypeTrueFalse,
	"填空题": question.TypeFillBlank,
	"解答题": question.TypeEssay,
}

func ParseCSV(reader io.Reader) ([]ImportRow, []RowError, error) {
	return parseCSV(reader, question.NewSanitizer())
}

func parseCSV(reader io.Reader, sanitizer *question.Sanitizer) ([]ImportRow, []RowError, error) {
	data, err := io.ReadAll(io.LimitReader(reader, MaxFileSize+1))
	if err != nil {
		return nil, nil, fmt.Errorf("read CSV: %w", err)
	}
	if len(data) > MaxFileSize {
		return nil, nil, ErrFileTooLarge
	}
	rows := make([]ImportRow, 0)
	rowErrors := make([]RowError, 0)
	if !utf8.Valid(data) {
		return rows, []RowError{{Line: 1, Field: "file", Message: "文件必须使用UTF-8编码"}}, nil
	}
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	csvReader := csv.NewReader(bytes.NewReader(data))
	csvReader.FieldsPerRecord = -1
	header, err := csvReader.Read()
	if errors.Is(err, io.EOF) {
		return rows, []RowError{{Line: 1, Field: "header", Message: "CSV不能为空且必须包含固定表头"}}, nil
	}
	if err != nil {
		return rows, []RowError{{Line: csvErrorLine(err, 1), Field: "header", Message: "CSV表头格式错误"}}, nil
	}
	if !equalHeader(header, expectedHeader) {
		return rows, []RowError{{Line: 1, Field: "header", Message: "表头必须严格为：" + strings.Join(expectedHeader, ",")}}, nil
	}
	csvReader.FieldsPerRecord = len(expectedHeader)
	dataRows := 0
	for {
		record, readErr := csvReader.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		line := dataRows + 2
		if len(record) > 0 {
			line, _ = csvReader.FieldPos(0)
		} else if readErr != nil {
			line = csvErrorLine(readErr, line)
		}
		if dataRows >= MaxDataRows {
			rowErrors = append(rowErrors, RowError{Line: line, Field: "file", Message: "数据行不能超过1000行"})
			break
		}
		dataRows++
		if readErr != nil {
			rowErrors = append(rowErrors, RowError{Line: line, Field: "record", Message: "CSV记录格式错误且必须恰好包含12列"})
			var parseError *csv.ParseError
			if errors.As(readErr, &parseError) && !errors.Is(readErr, csv.ErrFieldCount) {
				break
			}
			continue
		}
		row, rowError, normalizeErr := normalizeRecord(record, line, sanitizer)
		if normalizeErr != nil {
			return nil, nil, normalizeErr
		}
		if rowError != nil {
			rowErrors = append(rowErrors, *rowError)
			continue
		}
		rows = append(rows, row)
	}
	return rows, rowErrors, nil
}

func normalizeRecord(record []string, line int, sanitizer *question.Sanitizer) (ImportRow, *RowError, error) {
	questionType, supported := questionTypes[strings.TrimSpace(record[0])]
	if !supported {
		return ImportRow{}, &RowError{Line: line, Field: "question_type", Message: "题型必须为单选题、多选题、判断题、填空题或解答题"}, nil
	}
	difficulty, err := strconv.Atoi(strings.TrimSpace(record[3]))
	if err != nil {
		return ImportRow{}, &RowError{Line: line, Field: "difficulty", Message: "难度必须为1-5的整数"}, nil
	}
	tags := make([]string, 0)
	if strings.TrimSpace(record[4]) != "" {
		tags = strings.Split(strings.ReplaceAll(record[4], "；", ";"), ";")
	}
	content, err := convertRichText(record[5], sanitizer)
	if err != nil {
		return ImportRow{}, nil, err
	}
	options := make([]question.Option, 0, 4)
	for index, key := range []string{"A", "B", "C", "D"} {
		if strings.TrimSpace(record[6+index]) == "" {
			continue
		}
		converted, convertErr := convertRichText(record[6+index], sanitizer)
		if convertErr != nil {
			return ImportRow{}, nil, convertErr
		}
		options = append(options, question.Option{Key: key, Content: converted})
	}
	answer := record[10]
	if questionType == question.TypeTrueFalse {
		switch strings.TrimSpace(answer) {
		case "正确":
			answer = "true"
		case "错误":
			answer = "false"
		default:
			answer = strings.ToLower(strings.TrimSpace(answer))
		}
	}
	answer, err = convertRichText(answer, sanitizer)
	if err != nil {
		return ImportRow{}, nil, err
	}
	explanation, err := convertRichText(record[11], sanitizer)
	if err != nil {
		return ImportRow{}, nil, err
	}
	input := question.Input{
		Subject:      record[2],
		Grade:        record[1],
		QuestionType: questionType,
		Difficulty:   difficulty,
		Tags:         tags,
		Content:      content,
		Options:      options,
		Answer:       answer,
		Explanation:  explanation,
	}
	normalized, err := sanitizer.Normalize(input)
	if err != nil {
		var validation *question.ValidationError
		if errors.As(err, &validation) {
			return ImportRow{}, &RowError{Line: line, Field: validation.Violation.Field, Message: validation.Violation.Message}, nil
		}
		return ImportRow{}, nil, err
	}
	return importRowFromInput(line, normalized), nil, nil
}

type mathPlaceholder struct {
	token    string
	latex    string
	mathType string
}

func convertRichText(value string, sanitizer *question.Sanitizer) (string, error) {
	if value == "" {
		return "", nil
	}
	protected, placeholders := protectMath(value)
	sanitized := sanitizer.Sanitize(protected)
	context := &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div}
	nodes, err := html.ParseFragment(strings.NewReader(sanitized), context)
	if err != nil {
		return "", fmt.Errorf("parse CSV rich text: %w", err)
	}
	for _, node := range nodes {
		context.AppendChild(node)
	}
	for child := context.FirstChild; child != nil; {
		next := child.NextSibling
		convertTextMath(child, placeholders)
		child = next
	}
	var output strings.Builder
	for child := context.FirstChild; child != nil; child = child.NextSibling {
		if err := html.Render(&output, child); err != nil {
			return "", fmt.Errorf("render CSV rich text: %w", err)
		}
	}
	return output.String(), nil
}

func protectMath(value string) (string, []mathPlaceholder) {
	prefix := "SCHOOL_EXAM_STUDIO_MATH_PLACEHOLDER_"
	for strings.Contains(value, prefix) {
		prefix += "X"
	}
	placeholders := make([]mathPlaceholder, 0)
	var output strings.Builder
	for len(value) > 0 {
		opening := unescapedIndex(value, "$", 0)
		if opening < 0 {
			output.WriteString(value)
			break
		}
		output.WriteString(value[:opening])
		delimiter := "$"
		mathType := "inlineMath"
		if strings.HasPrefix(value[opening:], "$$") {
			delimiter = "$$"
			mathType = "blockMath"
		}
		contentStart := opening + len(delimiter)
		closingRelative := unescapedIndex(value[contentStart:], delimiter, 0)
		if closingRelative < 0 {
			output.WriteString(value[opening:])
			break
		}
		closing := contentStart + closingRelative
		latex := value[contentStart:closing]
		if strings.TrimSpace(latex) == "" || (delimiter == "$" && strings.ContainsAny(latex, "\r\n")) {
			output.WriteString(value[opening : closing+len(delimiter)])
			value = value[closing+len(delimiter):]
			continue
		}
		token := fmt.Sprintf("%s%d_END", prefix, len(placeholders))
		placeholders = append(placeholders, mathPlaceholder{token: token, latex: latex, mathType: mathType})
		output.WriteString(token)
		value = value[closing+len(delimiter):]
	}
	return output.String(), placeholders
}

func convertTextMath(node *html.Node, placeholders []mathPlaceholder) {
	if len(placeholders) == 0 {
		return
	}
	if node.Type == html.TextNode {
		parts := placeholderNodes(node.Data, placeholders)
		if len(parts) == 1 && parts[0].Type == html.TextNode && parts[0].Data == node.Data {
			return
		}
		for _, part := range parts {
			node.Parent.InsertBefore(part, node)
		}
		node.Parent.RemoveChild(node)
		return
	}
	for child := node.FirstChild; child != nil; {
		next := child.NextSibling
		convertTextMath(child, placeholders)
		child = next
	}
}

func placeholderNodes(value string, placeholders []mathPlaceholder) []*html.Node {
	nodes := make([]*html.Node, 0, 3)
	for len(value) > 0 {
		position := -1
		var matched mathPlaceholder
		for _, placeholder := range placeholders {
			index := strings.Index(value, placeholder.token)
			if index >= 0 && (position < 0 || index < position) {
				position = index
				matched = placeholder
			}
		}
		if position < 0 {
			nodes = appendTextNode(nodes, value)
			break
		}
		nodes = appendTextNode(nodes, value[:position])
		element := "span"
		elementAtom := atom.Span
		if matched.mathType == "blockMath" {
			element = "div"
			elementAtom = atom.Div
		}
		nodes = append(nodes, &html.Node{
			Type:     html.ElementNode,
			Data:     element,
			DataAtom: elementAtom,
			Attr:     []html.Attribute{{Key: "data-type", Val: matched.mathType}, {Key: "data-latex", Val: matched.latex}},
		})
		value = value[position+len(matched.token):]
	}
	if len(nodes) == 0 {
		nodes = appendTextNode(nodes, "")
	}
	return nodes
}

func appendTextNode(nodes []*html.Node, value string) []*html.Node {
	if value != "" {
		nodes = append(nodes, &html.Node{Type: html.TextNode, Data: value})
	}
	return nodes
}

func unescapedIndex(value, delimiter string, start int) int {
	for start <= len(value)-len(delimiter) {
		index := strings.Index(value[start:], delimiter)
		if index < 0 {
			return -1
		}
		index += start
		backslashes := 0
		for position := index - 1; position >= 0 && value[position] == '\\'; position-- {
			backslashes++
		}
		if backslashes%2 == 0 {
			return index
		}
		start = index + len(delimiter)
	}
	return -1
}

func equalHeader(actual, expected []string) bool {
	if len(actual) != len(expected) {
		return false
	}
	for index := range expected {
		if actual[index] != expected[index] {
			return false
		}
	}
	return true
}

func csvErrorLine(err error, fallback int) int {
	var parseError *csv.ParseError
	if errors.As(err, &parseError) && parseError.Line > 0 {
		return parseError.Line
	}
	return fallback
}
