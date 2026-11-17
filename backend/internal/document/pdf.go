package document

import (
	"bytes"
	"compress/flate"
	"compress/zlib"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

const (
	maxPDFStreamBytes      = 16 << 20
	maxPDFTotalStreamBytes = 64 << 20
	maxPDFObjects          = 5000
)

var (
	pdfObjectPattern = regexp.MustCompile(`(?s)(\d+)\s+(\d+)\s+obj\b`)
	pdfPagePattern   = regexp.MustCompile(`/Type\s*/Page\b`)
	pdfBFCharPair    = regexp.MustCompile(`<([0-9A-Fa-f]+)>\s*<([0-9A-Fa-f]+)>`)
	pdfCMapToken     = regexp.MustCompile(`\[|\]|<([0-9A-Fa-f]+)>`)
)

type pdfObject struct {
	number int
	body   []byte
}

type pdfCMap struct {
	codes   map[string]string
	twoByte bool
}

func extractPDF(data []byte) (Result, error) {
	objects := scanPDFObjects(data)
	if len(objects) == 0 {
		return Result{}, fmt.Errorf("PDF 结构无法识别")
	}
	sort.Slice(objects, func(i, j int) bool { return objects[i].number < objects[j].number })

	cmap := &pdfCMap{codes: map[string]string{}}
	decodedStreams := make(map[int][]byte, len(objects))
	warnings := []string{}
	totalBytes := 0
	for _, object := range objects {
		stream := pdfStreamData(object.body)
		if len(stream) == 0 {
			continue
		}
		if totalBytes+len(stream) > maxPDFTotalStreamBytes {
			warnings = append(warnings, "PDF 解压后内容过大，仅解析了前面的页面")
			break
		}
		totalBytes += len(stream)
		decodedStreams[object.number] = stream
		if bytes.Contains(stream, []byte("beginbfchar")) || bytes.Contains(stream, []byte("beginbfrange")) {
			cmap.merge(parsePDFCMap(stream))
		}
	}

	var text strings.Builder
	for _, object := range objects {
		stream, ok := decodedStreams[object.number]
		if !ok {
			continue
		}
		if bytes.Contains(stream, []byte("beginbfchar")) || bytes.Contains(stream, []byte("beginbfrange")) {
			continue
		}
		if !bytes.Contains(stream, []byte("BT")) || !containsAnyPDFOperator(stream) {
			continue
		}
		text.WriteString(extractPDFContentText(string(stream), cmap))
		text.WriteString("\n")
	}

	pages := len(pdfPagePattern.FindAll(data, -1))
	result := Result{
		Text:     text.String(),
		Kind:     "pdf",
		Pages:    pages,
		Warnings: warnings,
	}
	if utf8.RuneCountInString(strings.TrimSpace(result.Text)) < 20 {
		result.Warnings = append(result.Warnings, "未提取到足够的可读文本，可能是扫描版 PDF，请将页面转为图片后使用图片 OCR")
	}
	return result, nil
}

func containsAnyPDFOperator(stream []byte) bool {
	for _, operator := range [][]byte{[]byte("Tj"), []byte("TJ"), []byte("'"), []byte(`"`)} {
		if bytes.Contains(stream, operator) {
			return true
		}
	}
	return false
}

func scanPDFObjects(data []byte) []pdfObject {
	matches := pdfObjectPattern.FindAllSubmatchIndex(data, -1)
	objects := make([]pdfObject, 0, min(len(matches), maxPDFObjects))
	for _, match := range matches {
		if len(objects) >= maxPDFObjects {
			break
		}
		number, err := strconv.Atoi(string(data[match[2]:match[3]]))
		if err != nil {
			continue
		}
		start := match[1]
		end := bytes.Index(data[start:], []byte("endobj"))
		if end < 0 {
			continue
		}
		body := data[start : start+end]
		objects = append(objects, pdfObject{number: number, body: body})
	}
	return objects
}

func pdfStreamData(body []byte) []byte {
	index := bytes.Index(body, []byte("stream"))
	if index < 0 {
		return nil
	}
	dictionary := body[:index]
	start := index + len("stream")
	if start < len(body) && body[start] == '\r' {
		start++
	}
	if start < len(body) && body[start] == '\n' {
		start++
	}
	end := bytes.LastIndex(body, []byte("endstream"))
	if end <= start {
		return nil
	}
	raw := bytes.TrimRight(body[start:end], "\r\n")
	if len(raw) == 0 {
		return nil
	}
	if bytes.Contains(dictionary, []byte("/FlateDecode")) {
		return inflatePDFStream(raw)
	}
	if bytes.Contains(dictionary, []byte("/Filter")) {
		return nil
	}
	return raw
}

func inflatePDFStream(raw []byte) []byte {
	readers := []func(io.Reader) (io.ReadCloser, error){
		zlib.NewReader,
		func(reader io.Reader) (io.ReadCloser, error) { return flate.NewReader(reader), nil },
	}
	for _, open := range readers {
		stream, err := open(bytes.NewReader(raw))
		if err != nil {
			continue
		}
		decoded, err := io.ReadAll(io.LimitReader(stream, maxPDFStreamBytes))
		_ = stream.Close()
		if err == nil && len(decoded) > 0 {
			return decoded
		}
	}
	return nil
}

func parsePDFCMap(stream []byte) pdfCMap {
	text := string(stream)
	result := pdfCMap{codes: map[string]string{}}
	if index := strings.Index(text, "beginbfchar"); index >= 0 {
		if end := strings.Index(text[index:], "endbfchar"); end >= 0 {
			for _, pair := range pdfBFCharPair.FindAllStringSubmatch(text[index:index+end], -1) {
				key := strings.ToLower(pair[1])
				if value := utf16HexToString(pair[2]); value != "" {
					result.codes[key] = value
					if len(key) > 2 {
						result.twoByte = true
					}
				}
			}
		}
	}
	for offset := 0; offset < len(text); {
		index := strings.Index(text[offset:], "beginbfrange")
		if index < 0 {
			break
		}
		index += offset
		end := strings.Index(text[index:], "endbfrange")
		if end < 0 {
			break
		}
		parsePDFBFRange(text[index:index+end], &result)
		offset = index + end + len("endbfrange")
	}
	if len(result.codes) > 0 && result.twoByte == false {
		for key := range result.codes {
			if len(key) > 2 {
				result.twoByte = true
				break
			}
		}
	}
	return result
}

func parsePDFBFRange(block string, target *pdfCMap) {
	tokens := pdfCMapToken.FindAllStringSubmatch(block, -1)
	index := 0
	for index < len(tokens) {
		first := tokens[index]
		if first[0] == "[" || first[0] == "]" || first[1] == "" {
			index++
			continue
		}
		if index+1 >= len(tokens) || tokens[index+1][1] == "" {
			index++
			continue
		}
		startKey := strings.ToLower(first[1])
		endKey := strings.ToLower(tokens[index+1][1])
		index += 2
		startValue, startErr := strconv.ParseUint(startKey, 16, 64)
		endValue, endErr := strconv.ParseUint(endKey, 16, 64)
		if startErr != nil || endErr != nil || endValue < startValue {
			continue
		}
		var destinations []string
		if index < len(tokens) && tokens[index][0] == "[" {
			index++
			for index < len(tokens) && tokens[index][0] != "]" {
				if tokens[index][1] != "" {
					destinations = append(destinations, tokens[index][1])
				}
				index++
			}
			index++
		} else if index < len(tokens) && tokens[index][1] != "" {
			destinations = []string{tokens[index][1]}
			index++
		}
		if len(destinations) == 0 {
			continue
		}
		width := len(startKey)
		for code := startValue; code <= endValue; code++ {
			offset := code - startValue
			var destination string
			if offset < uint64(len(destinations)) {
				destination = destinations[offset]
			} else {
				destination = incrementHex(destinations[len(destinations)-1], int(offset)-len(destinations)+1)
			}
			value := utf16HexToString(destination)
			if value != "" {
				key := fmt.Sprintf("%0*x", width, code)
				target.codes[key] = value
				if width > 2 {
					target.twoByte = true
				}
			}
			if endValue-startValue > 65535 {
				break
			}
		}
	}
}

func (c *pdfCMap) merge(other pdfCMap) {
	for key, value := range other.codes {
		c.codes[key] = value
	}
	if other.twoByte {
		c.twoByte = true
	}
}

func utf16HexToString(value string) string {
	if len(value)%2 == 1 {
		value = "0" + value
	}
	raw, err := hex.DecodeString(value)
	if err != nil || len(raw) == 0 {
		return ""
	}
	if len(raw)%2 == 1 {
		raw = append([]byte{0}, raw...)
	}
	units := make([]uint16, 0, len(raw)/2)
	for index := 0; index+1 < len(raw); index += 2 {
		units = append(units, binary.BigEndian.Uint16(raw[index:index+2]))
	}
	return string(utf16.Decode(units))
}

func incrementHex(value string, delta int) string {
	width := len(value)
	number, err := strconv.ParseUint(value, 16, 64)
	if err != nil {
		return value
	}
	number += uint64(delta)
	return fmt.Sprintf("%0*x", width, number)
}

func extractPDFContentText(content string, cmap *pdfCMap) string {
	var out strings.Builder
	var operands []pdfToken
	var array []pdfToken
	inArray := false
	index := 0
	for index < len(content) {
		char := content[index]
		if isPDFWhitespace(char) {
			index++
			continue
		}
		if char == '%' {
			for index < len(content) && content[index] != '\n' {
				index++
			}
			continue
		}
		switch char {
		case '(':
			value, next := readPDFLiteralString(content, index)
			index = next
			token := pdfToken{kind: "string", text: value}
			if inArray {
				array = append(array, token)
			} else {
				operands = append(operands, token)
			}
		case '<':
			if index+1 < len(content) && content[index+1] == '<' {
				index = skipPDFDictionary(content, index)
				operands = append(operands, pdfToken{kind: "dict"})
				continue
			}
			value, next := readPDFHexString(content, index)
			index = next
			token := pdfToken{kind: "string", text: value}
			if inArray {
				array = append(array, token)
			} else {
				operands = append(operands, token)
			}
		case '[':
			inArray = true
			array = array[:0]
			index++
		case ']':
			operands = append(operands, pdfToken{kind: "array", items: append([]pdfToken(nil), array...)})
			inArray = false
			index++
		case '/':
			start := index + 1
			index++
			for index < len(content) && !isPDFWhitespace(content[index]) && !isPDFDelimiter(content[index]) {
				index++
			}
			operands = append(operands, pdfToken{kind: "name", text: content[start:index]})
		case '>':
			index++
		default:
			start := index
			for index < len(content) && !isPDFWhitespace(content[index]) && !isPDFDelimiter(content[index]) {
				index++
			}
			if start == index {
				index++
				continue
			}
			word := content[start:index]
			if _, err := strconv.ParseFloat(word, 64); err == nil {
				operands = append(operands, pdfToken{kind: "number", text: word})
				continue
			}
			applyPDFOperator(word, operands, cmap, &out)
			operands = operands[:0]
		}
	}
	return out.String()
}

type pdfToken struct {
	kind  string
	text  string
	items []pdfToken
}

func applyPDFOperator(operator string, operands []pdfToken, cmap *pdfCMap, out *strings.Builder) {
	last := func() (pdfToken, bool) {
		if len(operands) == 0 {
			return pdfToken{}, false
		}
		return operands[len(operands)-1], true
	}
	switch operator {
	case "Tj":
		if token, ok := last(); ok && token.kind == "string" {
			out.WriteString(decodePDFString(token.text, cmap))
		}
	case "TJ":
		if token, ok := last(); ok && token.kind == "array" {
			for _, item := range token.items {
				if item.kind == "string" {
					out.WriteString(decodePDFString(item.text, cmap))
				}
			}
		}
	case "'":
		out.WriteString("\n")
		if token, ok := last(); ok && token.kind == "string" {
			out.WriteString(decodePDFString(token.text, cmap))
		}
	case `"`:
		out.WriteString("\n")
		if token, ok := last(); ok && token.kind == "string" {
			out.WriteString(decodePDFString(token.text, cmap))
		}
	case "Td", "TD", "T*", "Tm", "ET":
		out.WriteString("\n")
	}
}

func decodePDFString(raw string, cmap *pdfCMap) string {
	if len(raw) == 0 {
		return ""
	}
	bytesValue := []byte(raw)
	if len(bytesValue) >= 2 && bytesValue[0] == 0xFE && bytesValue[1] == 0xFF {
		return decodeUTF16BE(bytesValue[2:])
	}
	if cmap != nil && len(cmap.codes) > 0 {
		if cmap.twoByte && len(bytesValue)%2 == 0 {
			var builder strings.Builder
			matched := true
			for index := 0; index < len(bytesValue); index += 2 {
				key := fmt.Sprintf("%02x%02x", bytesValue[index], bytesValue[index+1])
				value, ok := cmap.codes[key]
				if !ok {
					matched = false
					break
				}
				builder.WriteString(value)
			}
			if matched && builder.Len() > 0 {
				return builder.String()
			}
		}
		var builder strings.Builder
		matched := true
		for index := 0; index < len(bytesValue); index++ {
			key := fmt.Sprintf("%02x", bytesValue[index])
			value, ok := cmap.codes[key]
			if !ok {
				matched = false
				break
			}
			builder.WriteString(value)
		}
		if matched && builder.Len() > 0 {
			return builder.String()
		}
	}
	if len(bytesValue) >= 2 && looksLikeUTF16BE(bytesValue) {
		return decodeUTF16BE(bytesValue)
	}
	runes := make([]rune, 0, len(bytesValue))
	for _, value := range bytesValue {
		runes = append(runes, rune(value))
	}
	return string(runes)
}

func decodeUTF16BE(raw []byte) string {
	if len(raw)%2 == 1 {
		raw = append([]byte{0}, raw...)
	}
	units := make([]uint16, 0, len(raw)/2)
	for index := 0; index+1 < len(raw); index += 2 {
		units = append(units, binary.BigEndian.Uint16(raw[index:index+2]))
	}
	return string(utf16.Decode(units))
}

func looksLikeUTF16BE(raw []byte) bool {
	if len(raw)%2 != 0 {
		return false
	}
	zeroes := 0
	for index := 0; index < len(raw); index += 2 {
		if raw[index] == 0 {
			zeroes++
		}
	}
	return zeroes*2 >= len(raw)/2
}

func readPDFLiteralString(content string, start int) (string, int) {
	var builder strings.Builder
	depth := 1
	index := start + 1
	for index < len(content) {
		char := content[index]
		index++
		if char == '\\' {
			if index >= len(content) {
				break
			}
			escaped := content[index]
			index++
			switch escaped {
			case 'n':
				builder.WriteByte('\n')
			case 'r':
				builder.WriteByte('\r')
			case 't':
				builder.WriteByte('\t')
			case 'b':
				builder.WriteByte('\b')
			case 'f':
				builder.WriteByte('\f')
			case '\n':
			case '\r':
				if index < len(content) && content[index] == '\n' {
					index++
				}
			case '(', ')', '\\':
				builder.WriteByte(escaped)
			default:
				if escaped >= '0' && escaped <= '7' {
					octal := string(escaped)
					for count := 0; count < 2 && index < len(content) && content[index] >= '0' && content[index] <= '7'; count++ {
						octal += string(content[index])
						index++
					}
					if value, err := strconv.ParseInt(octal, 8, 32); err == nil {
						builder.WriteByte(byte(value))
					}
				} else {
					builder.WriteByte(escaped)
				}
			}
			continue
		}
		if char == '(' {
			depth++
			builder.WriteByte(char)
			continue
		}
		if char == ')' {
			depth--
			if depth == 0 {
				break
			}
			builder.WriteByte(char)
			continue
		}
		builder.WriteByte(char)
	}
	return builder.String(), index
}

func readPDFHexString(content string, start int) (string, int) {
	var builder strings.Builder
	index := start + 1
	for index < len(content) && content[index] != '>' {
		if !isPDFWhitespace(content[index]) {
			builder.WriteByte(content[index])
		}
		index++
	}
	if index < len(content) {
		index++
	}
	value := builder.String()
	if len(value)%2 == 1 {
		value += "0"
	}
	raw, err := hex.DecodeString(value)
	if err != nil {
		return "", index
	}
	return string(raw), index
}

func skipPDFDictionary(content string, start int) int {
	depth := 0
	index := start
	for index < len(content) {
		switch content[index] {
		case '<':
			if index+1 < len(content) && content[index+1] == '<' {
				depth++
				index += 2
				continue
			}
		case '>':
			if index+1 < len(content) && content[index+1] == '>' {
				depth--
				index += 2
				if depth == 0 {
					return index
				}
				continue
			}
		}
		index++
	}
	return index
}

func isPDFWhitespace(value byte) bool {
	return value == ' ' || value == '\t' || value == '\r' || value == '\n' || value == '\f' || value == 0
}

func isPDFDelimiter(value byte) bool {
	switch value {
	case '(', ')', '<', '>', '[', ']', '{', '}', '/', '%':
		return true
	default:
		return false
	}
}
