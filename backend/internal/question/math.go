package question

import (
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// normalizeMathMarkup 把富文本文本节点中的 LaTeX 分隔符转换为编辑器可识别的公式节点。
// 导入文档、AI 生成的内容常直接写 $...$ / $$...$$ / \(...\) / \[...\]，
// 统一转换后题库、试卷与编辑器都能正确渲染。
func normalizeMathMarkup(value string) string {
	if !strings.ContainsAny(value, "$\\") {
		return value
	}
	context := &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div}
	nodes, err := html.ParseFragment(strings.NewReader(value), context)
	if err != nil {
		return value
	}
	for _, node := range nodes {
		context.AppendChild(node)
	}
	convertMathInNode(context)
	var builder strings.Builder
	for child := context.FirstChild; child != nil; child = child.NextSibling {
		if err := html.Render(&builder, child); err != nil {
			return value
		}
	}
	return builder.String()
}

func convertMathInNode(node *html.Node) {
	if node.Type == html.ElementNode && (node.Data == "code" || node.Data == "pre" || node.Data == "script" || node.Data == "style") {
		return
	}
	for child := node.FirstChild; child != nil; {
		next := child.NextSibling
		if child.Type == html.TextNode {
			replaceMathTextNode(child)
			child = next
			continue
		}
		convertMathInNode(child)
		child = next
	}
}

func replaceMathTextNode(node *html.Node) {
	segments := splitMathText(node.Data)
	if len(segments) == 1 && segments[0].kind == mathSegmentText {
		return
	}
	for _, segment := range segments {
		var replacement *html.Node
		switch segment.kind {
		case mathSegmentInline, mathSegmentBlock:
			element := "span"
			mathType := "inlineMath"
			if segment.kind == mathSegmentBlock {
				element = "div"
				mathType = "blockMath"
			}
			replacement = &html.Node{
				Type: html.ElementNode,
				Data: element,
				Attr: []html.Attribute{{Key: "data-type", Val: mathType}, {Key: "data-latex", Val: segment.value}},
			}
		default:
			if segment.value == "" {
				continue
			}
			replacement = &html.Node{Type: html.TextNode, Data: segment.value}
		}
		node.Parent.InsertBefore(replacement, node)
	}
	node.Parent.RemoveChild(node)
}

type mathSegmentKind int

const (
	mathSegmentText mathSegmentKind = iota
	mathSegmentInline
	mathSegmentBlock
)

type mathSegment struct {
	kind  mathSegmentKind
	value string
}

// splitMathText 扫描纯文本中的公式分隔符，未闭合或内容为空的分隔符保持原样。
func splitMathText(value string) []mathSegment {
	segments := make([]mathSegment, 0, 3)
	var text strings.Builder
	flushText := func() {
		if text.Len() == 0 {
			return
		}
		segments = append(segments, mathSegment{kind: mathSegmentText, value: text.String()})
		text.Reset()
	}
	for index := 0; index < len(value); {
		if value[index] == '\\' && !isEscaped(value, index) && index+1 < len(value) && (value[index+1] == '(' || value[index+1] == '[') {
			closing := `\)`
			kind := mathSegmentInline
			if value[index+1] == '[' {
				closing = `\]`
				kind = mathSegmentBlock
			}
			end := unescapedDelimiterIndex(value, closing, index+2)
			if end < 0 {
				text.WriteByte(value[index])
				index++
				continue
			}
			latex := strings.TrimSpace(value[index+2 : end])
			if latex == "" {
				text.WriteString(value[index : end+len(closing)])
				index = end + len(closing)
				continue
			}
			flushText()
			segments = append(segments, mathSegment{kind: kind, value: latex})
			index = end + len(closing)
			continue
		}
		if value[index] == '$' && !isEscaped(value, index) {
			delimiter := "$"
			kind := mathSegmentInline
			if strings.HasPrefix(value[index:], "$$") {
				delimiter = "$$"
				kind = mathSegmentBlock
			}
			contentStart := index + len(delimiter)
			end := unescapedDelimiterIndex(value, delimiter, contentStart)
			if end < 0 {
				text.WriteString(value[index:])
				break
			}
			latex := value[contentStart:end]
			if strings.TrimSpace(latex) == "" || (delimiter == "$" && strings.ContainsAny(latex, "\r\n")) {
				text.WriteString(value[index : end+len(delimiter)])
				index = end + len(delimiter)
				continue
			}
			flushText()
			segments = append(segments, mathSegment{kind: kind, value: latex})
			index = end + len(delimiter)
			continue
		}
		text.WriteByte(value[index])
		index++
	}
	flushText()
	if len(segments) == 0 {
		segments = append(segments, mathSegment{kind: mathSegmentText, value: value})
	}
	return segments
}

func unescapedDelimiterIndex(value, delimiter string, start int) int {
	for start <= len(value)-len(delimiter) {
		relative := strings.Index(value[start:], delimiter)
		if relative < 0 {
			return -1
		}
		position := start + relative
		if !isEscaped(value, position) {
			return position
		}
		start = position + len(delimiter)
	}
	return -1
}

func isEscaped(value string, index int) bool {
	backslashes := 0
	for index-1-backslashes >= 0 && value[index-1-backslashes] == '\\' {
		backslashes++
	}
	return backslashes%2 == 1
}
