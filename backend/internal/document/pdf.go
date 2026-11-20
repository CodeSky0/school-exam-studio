package document

import (
	"bytes"
	"compress/flate"
	"compress/zlib"
	"encoding/ascii85"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
)

const (
	maxPDFStreamBytes      = 16 << 20
	maxPDFTotalStreamBytes = 64 << 20
	maxPDFObjects          = 20000
	maxPDFPageDepth        = 64
)

var (
	pdfObjectPattern = regexp.MustCompile(`(?s)(\d+)\s+(\d+)\s+obj\b`)
	pdfPagePattern   = regexp.MustCompile(`/Type\s*/Page\b`)
	pdfBFCharPair    = regexp.MustCompile(`<([0-9A-Fa-f]+)>\s*<([0-9A-Fa-f]+)>`)
	pdfCMapToken     = regexp.MustCompile(`\[|\]|<([0-9A-Fa-f]+)>`)
	pdfFilterPattern = regexp.MustCompile(`/Filter\s*(\[[^\]]*\]|/[A-Za-z0-9]+)`)
	pdfNamePattern   = regexp.MustCompile(`/[A-Za-z0-9]+`)
	pdfIntegerToken  = regexp.MustCompile(`[-+]?\d+`)
	pdfRefPattern    = regexp.MustCompile(`(\d+)\s+(\d+)\s+R\b`)
)

type pdfObject struct {
	number    int
	body      []byte
	stream    []byte
	hasStream bool
}

// pdfDocument 汇总 PDF 的间接对象，并展开对象流、字体与页面信息。
type pdfDocument struct {
	objects  map[int]*pdfObject
	order    []int
	warnings []string
}

type pdfCMap struct {
	codes   map[string]string
	twoByte bool
}

type pdfFont struct {
	cmap     *pdfCMap
	twoByte  bool
	encoding string
}

type pdfPage struct {
	number   int
	contents []int
	fonts    map[string]*pdfFont
}

func extractPDF(data []byte) (Result, error) {
	doc := buildPDFDocument(data)
	if len(doc.objects) == 0 {
		return Result{}, fmt.Errorf("PDF 结构无法识别")
	}
	fonts := doc.loadFonts()
	pages := doc.pages(fonts)
	globalFonts := doc.globalFonts(fonts)

	var text strings.Builder
	unmappedFonts := 0
	usedStreams := 0
	for _, page := range pages {
		layout := &pdfTextLayout{}
		pageFonts := page.fonts
		if len(pageFonts) == 0 {
			pageFonts = globalFonts
		}
		extractor := &pdfTextExtractor{layout: layout, fonts: pageFonts, fallbackFonts: globalFonts}
		for _, number := range page.contents {
			object := doc.objects[number]
			if object == nil || !object.hasStream || len(object.stream) == 0 {
				continue
			}
			extractor.extract(string(object.stream))
			usedStreams++
		}
		pageText := strings.TrimSpace(layout.Text())
		if pageText != "" {
			text.WriteString(pageText)
			text.WriteString("\n")
		}
		unmappedFonts += extractor.unmappedFonts
	}

	if usedStreams == 0 {
		// 页面与资源无法关联时，退化为扫描所有文本内容流。
		for _, number := range doc.order {
			object := doc.objects[number]
			if object == nil || !object.hasStream || len(object.stream) == 0 {
				continue
			}
			if bytes.Contains(object.stream, []byte("beginbfchar")) || bytes.Contains(object.stream, []byte("beginbfrange")) {
				continue
			}
			if !bytes.Contains(object.stream, []byte("BT")) || !containsAnyPDFOperator(object.stream) {
				continue
			}
			layout := &pdfTextLayout{}
			extractor := &pdfTextExtractor{layout: layout, fonts: globalFonts, fallbackFonts: globalFonts}
			extractor.extract(string(object.stream))
			unmappedFonts += extractor.unmappedFonts
			pageText := strings.TrimSpace(layout.Text())
			if pageText != "" {
				text.WriteString(pageText)
				text.WriteString("\n")
			}
		}
	}

	pagesCount := len(pages)
	if pagesCount == 0 {
		pagesCount = len(pdfPagePattern.FindAll(data, -1))
	}
	result := Result{
		Text:     cleanPDFText(text.String()),
		Kind:     "pdf",
		Pages:    pagesCount,
		Warnings: uniqueStrings(doc.warnings),
	}
	if unmappedFonts > 0 {
		result.Warnings = append(result.Warnings, "PDF 内嵌字体缺少 Unicode 映射，部分文字可能按字形编码解析，请核对导入内容")
	}
	if utf8.RuneCountInString(strings.TrimSpace(result.Text)) < 20 {
		result.Warnings = append(result.Warnings, "未提取到足够的可读文本，可能是扫描版 PDF，请将页面转为图片后交给 AI 直接阅读")
	}
	if !looksReadablePDFText(result.Text) {
		result.Warnings = append(result.Warnings, "PDF 文字编码异常，已跳过无法识别的字符；建议上传页面图片交由视觉模型阅读")
	}
	result.Warnings = uniqueStrings(result.Warnings)
	return result, nil
}

func buildPDFDocument(data []byte) *pdfDocument {
	doc := &pdfDocument{objects: make(map[int]*pdfObject)}
	for _, object := range scanPDFObjects(data) {
		clone := object
		doc.objects[object.number] = &clone
	}
	doc.decodeStreams()
	doc.expandObjectStreams()
	doc.rebuildOrder()
	return doc
}

func (d *pdfDocument) rebuildOrder() {
	d.order = d.order[:0]
	for number := range d.objects {
		d.order = append(d.order, number)
	}
	sort.Ints(d.order)
}

func (d *pdfDocument) decodeStreams() {
	total := 0
	seen := make(map[string]bool)
	for _, number := range d.sortedNumbers() {
		object := d.objects[number]
		dictionary, raw, ok := splitPDFStream(object.body)
		if !ok {
			continue
		}
		if total+len(raw) > maxPDFTotalStreamBytes {
			d.warnings = append(d.warnings, "PDF 解压后内容过大，仅解析了前面的页面")
			break
		}
		total += len(raw)
		decoded, warnings := decodePDFStream(dictionary, raw)
		for _, warning := range warnings {
			if !seen[warning] {
				seen[warning] = true
				d.warnings = append(d.warnings, warning)
			}
		}
		if decoded == nil {
			continue
		}
		object.stream = decoded
		object.hasStream = true
	}
}

func (d *pdfDocument) sortedNumbers() []int {
	numbers := make([]int, 0, len(d.objects))
	for number := range d.objects {
		numbers = append(numbers, number)
	}
	sort.Ints(numbers)
	return numbers
}

// expandObjectStreams 解压 /Type /ObjStm，把其中压缩保存的字体、页面字典等对象展开。
func (d *pdfDocument) expandObjectStreams() {
	for _, number := range d.sortedNumbers() {
		object := d.objects[number]
		if object == nil || !object.hasStream || len(object.stream) == 0 {
			continue
		}
		dictionary, _, ok := splitPDFStream(object.body)
		if !ok {
			continue
		}
		parsed := parsePDFDict(dictionary)
		if pdfDictName(parsed, "Type") != "ObjStm" {
			continue
		}
		count := pdfDictInt(parsed, "N", 0)
		first := pdfDictInt(parsed, "First", 0)
		if count <= 0 || first <= 0 || first > len(object.stream) {
			continue
		}
		values := pdfIntegerToken.FindAllString(string(object.stream[:first]), -1)
		if len(values) < count*2 {
			count = len(values) / 2
		}
		for index := 0; index < count; index++ {
			childNumber, numberErr := strconv.Atoi(values[index*2])
			offset, offsetErr := strconv.Atoi(values[index*2+1])
			if numberErr != nil || offsetErr != nil {
				continue
			}
			start := first + offset
			if start < 0 || start > len(object.stream) {
				continue
			}
			end := len(object.stream)
			if index+1 < count {
				if next, err := strconv.Atoi(values[index*2+3]); err == nil {
					if nextStart := first + next; nextStart > start && nextStart < end {
						end = nextStart
					}
				}
			}
			if end <= start {
				continue
			}
			if _, exists := d.objects[childNumber]; exists {
				continue
			}
			body := append([]byte(nil), object.stream[start:end]...)
			d.objects[childNumber] = &pdfObject{number: childNumber, body: bytes.TrimSpace(body)}
			if len(d.objects) >= maxPDFObjects {
				return
			}
		}
	}
	d.rebuildOrder()
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

func splitPDFStream(body []byte) (dictionary, raw []byte, ok bool) {
	index := bytes.Index(body, []byte("stream"))
	if index < 0 {
		return nil, nil, false
	}
	dictionary = bytes.TrimSpace(body[:index])
	if !bytes.Contains(dictionary, []byte("<<")) {
		return nil, nil, false
	}
	start := index + len("stream")
	if start < len(body) && body[start] == '\r' {
		start++
	}
	if start < len(body) && body[start] == '\n' {
		start++
	}
	end := bytes.LastIndex(body, []byte("endstream"))
	if end <= start {
		return nil, nil, false
	}
	raw = bytes.TrimRight(body[start:end], "\r\n")
	if len(raw) == 0 {
		return nil, nil, false
	}
	return dictionary, raw, true
}

func decodePDFStream(dictionary, raw []byte) ([]byte, []string) {
	filters := pdfFilterNames(dictionary)
	if len(filters) == 0 {
		return raw, nil
	}
	if pdfDictName(parsePDFDict(dictionary), "Subtype") == "Image" {
		return nil, nil
	}
	data := raw
	for _, filter := range filters {
		switch filter {
		case "FlateDecode", "Fl":
			data = inflatePDFStream(data)
			if data == nil {
				return nil, []string{"PDF 内容解压失败，部分文字可能缺失"}
			}
		case "ASCIIHexDecode", "AHx":
			decoded, err := decodePDFASCIIHex(data)
			if err != nil {
				return nil, []string{"PDF 十六进制内容解析失败，部分文字可能缺失"}
			}
			data = decoded
		case "ASCII85Decode", "A85":
			decoded, err := decodePDFASCII85(data)
			if err != nil {
				return nil, []string{"PDF ASCII85 内容解析失败，部分文字可能缺失"}
			}
			data = decoded
		case "RunLengthDecode", "RL":
			data = decodePDFRunLength(data)
			if len(data) == 0 {
				return nil, []string{"PDF RunLength 内容解析失败，部分文字可能缺失"}
			}
		case "DCTDecode", "DCT", "JPXDecode", "CCITTFaxDecode", "JBIG2Decode", "Crypt":
			return nil, nil
		default:
			return nil, []string{fmt.Sprintf("PDF 使用了暂不支持的压缩格式 %s，部分内容可能缺失", filter)}
		}
	}
	return data, nil
}

func pdfFilterNames(dictionary []byte) []string {
	match := pdfFilterPattern.FindSubmatch(dictionary)
	if len(match) < 2 {
		return nil
	}
	value := bytes.TrimSpace(match[1])
	if len(value) == 0 {
		return nil
	}
	names := make([]string, 0, 2)
	for _, item := range pdfNamePattern.FindAll(value, -1) {
		names = append(names, strings.TrimPrefix(string(item), "/"))
	}
	return names
}

func decodePDFASCIIHex(raw []byte) ([]byte, error) {
	digits := make([]byte, 0, len(raw))
	for _, value := range raw {
		if value == '>' {
			break
		}
		if isPDFWhitespace(value) {
			continue
		}
		digits = append(digits, value)
	}
	if len(digits)%2 == 1 {
		digits = append(digits, '0')
	}
	return hex.DecodeString(string(digits))
}

func decodePDFASCII85(raw []byte) ([]byte, error) {
	source := bytes.TrimSpace(raw)
	source = bytes.TrimPrefix(source, []byte("<~"))
	if index := bytes.Index(source, []byte("~>")); index >= 0 {
		source = source[:index]
	}
	target := make([]byte, len(source)*4/5+8)
	written, _, err := ascii85.Decode(target, source, true)
	if err != nil {
		return nil, err
	}
	return target[:written], nil
}

func decodePDFRunLength(raw []byte) []byte {
	output := make([]byte, 0, len(raw))
	for index := 0; index < len(raw); {
		length := int(raw[index])
		index++
		if length == 128 {
			break
		}
		if length < 128 {
			count := length + 1
			if index+count > len(raw) {
				count = len(raw) - index
			}
			output = append(output, raw[index:index+count]...)
			index += count
			continue
		}
		if index >= len(raw) {
			break
		}
		count := 257 - length
		value := raw[index]
		index++
		for repeat := 0; repeat < count; repeat++ {
			output = append(output, value)
		}
	}
	return output
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

func (d *pdfDocument) loadFonts() map[int]*pdfFont {
	fonts := make(map[int]*pdfFont)
	for _, number := range d.order {
		object := d.objects[number]
		dictionary := d.objectDict(object)
		if pdfDictName(dictionary, "Type") != "Font" {
			subtype := pdfDictName(dictionary, "Subtype")
			if subtype == "" || pdfDictName(dictionary, "BaseFont") == "" {
				continue
			}
		}
		font := &pdfFont{}
		if reference, ok := pdfDictRef(dictionary, "ToUnicode"); ok {
			if cmapObject := d.objects[reference]; cmapObject != nil && cmapObject.hasStream {
				parsed := parsePDFCMap(cmapObject.stream)
				if len(parsed.codes) > 0 {
					font.cmap = &parsed
				}
			}
		}
		encoding := pdfDictName(dictionary, "Encoding")
		if encoding == "" {
			if encodingDict, ok := d.dictValue(dictionary, "Encoding"); ok {
				encoding = pdfDictName(encodingDict, "CMapName")
			}
		}
		font.encoding = encoding
		subtype := pdfDictName(dictionary, "Subtype")
		if subtype == "Type0" || strings.Contains(strings.ToUpper(encoding), "IDENTITY") {
			font.twoByte = true
		}
		fonts[number] = font
	}
	return fonts
}

func (d *pdfDocument) pages(fonts map[int]*pdfFont) []pdfPage {
	var catalog map[string][]byte
	for _, number := range d.order {
		dictionary := d.objectDict(d.objects[number])
		if pdfDictName(dictionary, "Type") == "Catalog" {
			catalog = dictionary
			break
		}
	}
	pages := make([]pdfPage, 0, 8)
	visited := make(map[int]bool)
	if catalog != nil {
		if root, ok := pdfDictRef(catalog, "Pages"); ok {
			d.walkPageTree(root, nil, &pages, visited, fonts, 0)
		}
	}
	if len(pages) == 0 {
		for _, number := range d.order {
			dictionary := d.objectDict(d.objects[number])
			if pdfDictName(dictionary, "Type") != "Page" {
				continue
			}
			pages = append(pages, d.makePage(number, dictionary, nil, fonts))
		}
	}
	return pages
}

func (d *pdfDocument) walkPageTree(reference int, inherited map[string][]byte, pages *[]pdfPage, visited map[int]bool, fonts map[int]*pdfFont, depth int) {
	if depth > maxPDFPageDepth || visited[reference] {
		return
	}
	visited[reference] = true
	object := d.objects[reference]
	if object == nil {
		return
	}
	dictionary := d.objectDict(object)
	resources := inherited
	if own, ok := d.dictValue(dictionary, "Resources"); ok {
		resources = own
	}
	kids := parsePDFRefs(dictionary["Kids"])
	nodeType := pdfDictName(dictionary, "Type")
	if nodeType == "Pages" || (nodeType == "" && len(kids) > 0) {
		for _, kid := range kids {
			d.walkPageTree(kid, resources, pages, visited, fonts, depth+1)
		}
		return
	}
	*pages = append(*pages, d.makePage(reference, dictionary, resources, fonts))
}

func (d *pdfDocument) makePage(number int, dictionary, resources map[string][]byte, fonts map[int]*pdfFont) pdfPage {
	page := pdfPage{number: number, fonts: make(map[string]*pdfFont)}
	if resources != nil {
		if fontDict, ok := d.dictValue(resources, "Font"); ok {
			for name, value := range fontDict {
				reference, ok := parsePDFRefValue(value)
				if !ok {
					continue
				}
				if font := fonts[reference]; font != nil {
					page.fonts[name] = font
				}
			}
		}
	}
	page.contents = parsePDFRefs(dictionary["Contents"])
	return page
}

func (d *pdfDocument) globalFonts(fonts map[int]*pdfFont) map[string]*pdfFont {
	result := make(map[string]*pdfFont)
	add := func(fontDict map[string][]byte) {
		for name, value := range fontDict {
			reference, ok := parsePDFRefValue(value)
			if !ok {
				continue
			}
			font := fonts[reference]
			if font == nil {
				continue
			}
			if _, exists := result[name]; !exists {
				result[name] = font
			}
		}
	}
	for _, number := range d.order {
		dictionary := d.objectDict(d.objects[number])
		if fontDict, ok := d.dictValue(dictionary, "Font"); ok {
			add(fontDict)
		}
		if resources, ok := d.dictValue(dictionary, "Resources"); ok {
			if fontDict, ok := d.dictValue(resources, "Font"); ok {
				add(fontDict)
			}
		}
	}
	return result
}

func (d *pdfDocument) objectDict(object *pdfObject) map[string][]byte {
	if object == nil {
		return nil
	}
	body := object.body
	if index := bytes.Index(body, []byte("stream")); index >= 0 {
		body = body[:index]
	}
	return parsePDFDict(body)
}

func (d *pdfDocument) dictValue(dictionary map[string][]byte, key string) (map[string][]byte, bool) {
	if dictionary == nil {
		return nil, false
	}
	value, ok := dictionary[key]
	if !ok {
		return nil, false
	}
	return d.resolveDict(value)
}

func (d *pdfDocument) resolveDict(value []byte) (map[string][]byte, bool) {
	value = bytes.TrimSpace(value)
	if len(value) == 0 {
		return nil, false
	}
	if bytes.HasPrefix(value, []byte("<<")) {
		return parsePDFDict(value), true
	}
	if reference, ok := parsePDFRefValue(value); ok {
		object := d.objects[reference]
		if object == nil {
			return nil, false
		}
		return d.objectDict(object), true
	}
	return nil, false
}

func parsePDFDict(data []byte) map[string][]byte {
	result := make(map[string][]byte)
	index := 0
	for index < len(data) {
		for index < len(data) && isPDFWhitespace(data[index]) {
			index++
		}
		if index < len(data) && data[index] == '%' {
			for index < len(data) && data[index] != '\n' {
				index++
			}
			continue
		}
		if index >= len(data) {
			break
		}
		if data[index] != '/' {
			index++
			continue
		}
		start := index + 1
		index++
		for index < len(data) && !isPDFWhitespace(data[index]) && !isPDFDelimiter(data[index]) {
			index++
		}
		key := string(data[start:index])
		for index < len(data) && isPDFWhitespace(data[index]) {
			index++
		}
		if index >= len(data) {
			result[key] = nil
			break
		}
		valueStart := index
		index = scanPDFValueEnd(data, index)
		value := bytes.TrimSpace(data[valueStart:index])
		if isPDFUnsignedInteger(value) {
			cursor := index
			for cursor < len(data) && isPDFWhitespace(data[cursor]) {
				cursor++
			}
			numberStart := cursor
			for cursor < len(data) && data[cursor] >= '0' && data[cursor] <= '9' {
				cursor++
			}
			if cursor > numberStart {
				end := cursor
				for end < len(data) && isPDFWhitespace(data[end]) {
					end++
				}
				if end < len(data) && data[end] == 'R' {
					index = end + 1
					value = bytes.TrimSpace(data[valueStart:index])
				}
			}
		}
		if key != "" {
			result[key] = value
		}
	}
	return result
}

func scanPDFValueEnd(data []byte, start int) int {
	if start >= len(data) {
		return len(data)
	}
	switch data[start] {
	case '<':
		if start+1 < len(data) && data[start+1] == '<' {
			return scanPDFNested(data, start, "<<", ">>")
		}
		if end := bytes.IndexByte(data[start:], '>'); end >= 0 {
			return start + end + 1
		}
		return len(data)
	case '[':
		return scanPDFNested(data, start, "[", "]")
	case '(':
		return scanPDFStringEnd(data, start)
	case '/':
		index := start + 1
		for index < len(data) && !isPDFWhitespace(data[index]) && !isPDFDelimiter(data[index]) {
			index++
		}
		return index
	default:
		index := start
		for index < len(data) && !isPDFWhitespace(data[index]) && !isPDFDelimiter(data[index]) {
			index++
		}
		if index == start {
			index++
		}
		return index
	}
}

func scanPDFNested(data []byte, start int, opening, closing string) int {
	depth := 0
	index := start
	for index < len(data) {
		if bytes.HasPrefix(data[index:], []byte(opening)) {
			depth++
			index += len(opening)
			continue
		}
		if bytes.HasPrefix(data[index:], []byte(closing)) {
			depth--
			index += len(closing)
			if depth == 0 {
				return index
			}
			continue
		}
		switch data[index] {
		case '(':
			index = scanPDFStringEnd(data, index)
		case '<':
			if bytes.HasPrefix(data[index:], []byte("<<")) {
				index = scanPDFNested(data, index, "<<", ">>")
				continue
			}
			if end := bytes.IndexByte(data[index:], '>'); end >= 0 {
				index += end + 1
				continue
			}
			index++
		case '[':
			if opening == "<<" {
				index = scanPDFNested(data, index, "[", "]")
				continue
			}
			index++
		default:
			index++
		}
	}
	return len(data)
}

func scanPDFStringEnd(data []byte, start int) int {
	depth := 1
	index := start + 1
	for index < len(data) {
		switch data[index] {
		case '\\':
			index += 2
			continue
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return index + 1
			}
		}
		index++
	}
	return len(data)
}

func pdfDictName(dictionary map[string][]byte, key string) string {
	value := bytes.TrimSpace(dictionary[key])
	if len(value) == 0 || value[0] != '/' {
		return ""
	}
	name := string(value[1:])
	for index := 0; index < len(name); index++ {
		if name[index] == '/' || name[index] <= ' ' || name[index] == '[' || name[index] == ']' || name[index] == '<' || name[index] == '>' {
			return name[:index]
		}
	}
	return name
}

func pdfDictInt(dictionary map[string][]byte, key string, fallback int) int {
	value := strings.TrimSpace(string(dictionary[key]))
	number, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return number
}

func pdfDictRef(dictionary map[string][]byte, key string) (int, bool) {
	return parsePDFRefValue(dictionary[key])
}

func parsePDFRefValue(value []byte) (int, bool) {
	fields := strings.Fields(string(bytes.TrimSpace(value)))
	if len(fields) != 3 || fields[2] != "R" {
		return 0, false
	}
	number, err := strconv.Atoi(fields[0])
	if err != nil || number <= 0 {
		return 0, false
	}
	return number, true
}

func parsePDFRefs(value []byte) []int {
	value = bytes.TrimSpace(value)
	if len(value) == 0 {
		return nil
	}
	matches := pdfRefPattern.FindAllSubmatch(value, -1)
	refs := make([]int, 0, len(matches))
	for _, match := range matches {
		number, err := strconv.Atoi(string(match[1]))
		if err != nil || number <= 0 {
			continue
		}
		refs = append(refs, number)
	}
	return refs
}

func isPDFUnsignedInteger(value []byte) bool {
	if len(value) == 0 {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
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
	if len(result.codes) > 0 && !result.twoByte {
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

type pdfTextLayout struct {
	builder  strings.Builder
	lastChar byte
	placed   bool
	lineY    float64
	lastX    float64
}

func (l *pdfTextLayout) Text() string {
	return l.builder.String()
}

func (l *pdfTextLayout) Write(text string) {
	if text == "" {
		return
	}
	if l.builder.Len() > 0 && l.lastChar == '\n' {
		text = strings.TrimLeft(text, " \t")
	}
	if text == "" {
		return
	}
	l.builder.WriteString(text)
	l.lastChar = text[len(text)-1]
}

func (l *pdfTextLayout) NewLine() {
	if l.builder.Len() == 0 || l.lastChar == '\n' {
		return
	}
	l.builder.WriteByte('\n')
	l.lastChar = '\n'
}

func (l *pdfTextLayout) Space() {
	if l.builder.Len() == 0 || l.lastChar == ' ' || l.lastChar == '\n' {
		return
	}
	l.builder.WriteByte(' ')
	l.lastChar = ' '
}

func (l *pdfTextLayout) MoveRelative(x, y, fontSize float64) {
	threshold := math.Max(0.6, fontSize*0.4)
	if !l.placed {
		l.placed = true
		l.lineY = y
		l.lastX = x
		return
	}
	if math.Abs(y) > threshold {
		l.NewLine()
		l.lineY += y
		l.lastX += x
		return
	}
	if x > math.Max(2, fontSize*1.8) {
		l.Space()
	}
	l.lastX += x
}

func (l *pdfTextLayout) MoveAbsolute(x, y, fontSize float64) {
	threshold := math.Max(0.6, fontSize*0.4)
	if !l.placed {
		l.placed = true
		l.lineY = y
		l.lastX = x
		return
	}
	if math.Abs(y-l.lineY) > threshold {
		l.NewLine()
		l.lineY = y
		l.lastX = x
		return
	}
	if x-l.lastX > math.Max(2, fontSize*1.8) {
		l.Space()
	}
	l.lastX = x
}

type pdfTextExtractor struct {
	layout        *pdfTextLayout
	fonts         map[string]*pdfFont
	fallbackFonts map[string]*pdfFont
	current       *pdfFont
	fontSize      float64
	unmappedFonts int
}

func (e *pdfTextExtractor) lookupFont(name string) *pdfFont {
	if font := e.fonts[name]; font != nil {
		return font
	}
	return e.fallbackFonts[name]
}

func (e *pdfTextExtractor) writeString(raw string) {
	e.layout.Write(decodePDFString(raw, e.current))
}

func (e *pdfTextExtractor) extract(content string) {
	e.current = nil
	e.fontSize = 12
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
			if word == "BI" {
				index = skipPDFInlineImage(content, index)
				operands = operands[:0]
				continue
			}
			e.applyPDFOperator(word, operands)
			operands = operands[:0]
		}
	}
}

func (e *pdfTextExtractor) applyPDFOperator(operator string, operands []pdfToken) {
	switch operator {
	case "Tf":
		name := ""
		for index := len(operands) - 1; index >= 0; index-- {
			if operands[index].kind == "name" {
				name = operands[index].text
				break
			}
		}
		if size, ok := lastPDFNumber(operands); ok {
			e.fontSize = size
		}
		e.current = e.lookupFont(name)
		if e.current == nil && name != "" {
			e.current = &pdfFont{}
		}
		if fontNeedsUnicodeFallback(e.current) {
			e.unmappedFonts++
		}
	case "Tj":
		if value, ok := lastPDFString(operands); ok {
			e.writeString(value)
		}
	case "TJ":
		if len(operands) == 0 {
			return
		}
		array := operands[len(operands)-1]
		if array.kind != "array" {
			return
		}
		for _, item := range array.items {
			if item.kind == "string" {
				e.writeString(item.text)
			}
		}
	case "'", `"`:
		e.layout.NewLine()
		if value, ok := lastPDFString(operands); ok {
			e.writeString(value)
		}
	case "Td", "TD":
		values, ok := lastPDFNumbers(operands, 2)
		if !ok {
			return
		}
		e.layout.MoveRelative(values[0], values[1], e.fontSize)
	case "T*":
		e.layout.NewLine()
	case "Tm":
		values, ok := lastPDFNumbers(operands, 6)
		if !ok {
			return
		}
		e.layout.MoveAbsolute(values[4], values[5], e.fontSize)
	}
}

func lastPDFString(operands []pdfToken) (string, bool) {
	for index := len(operands) - 1; index >= 0; index-- {
		if operands[index].kind == "string" {
			return operands[index].text, true
		}
	}
	return "", false
}

func lastPDFNumber(operands []pdfToken) (float64, bool) {
	values, ok := lastPDFNumbers(operands, 1)
	if !ok {
		return 0, false
	}
	return values[0], true
}

func lastPDFNumbers(operands []pdfToken, count int) ([]float64, bool) {
	values := make([]float64, 0, count)
	for index := len(operands) - 1; index >= 0 && len(values) < count; index-- {
		if operands[index].kind != "number" {
			continue
		}
		value, err := strconv.ParseFloat(operands[index].text, 64)
		if err != nil {
			continue
		}
		values = append(values, value)
	}
	if len(values) != count {
		return nil, false
	}
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
	return values, true
}

func skipPDFInlineImage(content string, start int) int {
	index := strings.Index(content[start:], "ID")
	if index < 0 {
		return len(content)
	}
	index += start + 2
	for index < len(content)-1 {
		if content[index] == 'E' && content[index+1] == 'I' {
			before := index == 0 || isPDFWhitespace(content[index-1])
			after := index+2 >= len(content) || isPDFWhitespace(content[index+2])
			if before && after {
				return index + 2
			}
		}
		index++
	}
	return len(content)
}

func containsAnyPDFOperator(stream []byte) bool {
	for _, operator := range [][]byte{[]byte("Tj"), []byte("TJ"), []byte("'"), []byte(`"`)} {
		if bytes.Contains(stream, operator) {
			return true
		}
	}
	return false
}

type pdfToken struct {
	kind  string
	text  string
	items []pdfToken
}

func fontNeedsUnicodeFallback(font *pdfFont) bool {
	if font == nil || font.cmap != nil || !font.twoByte {
		return false
	}
	encoding := strings.ToUpper(font.encoding)
	return encoding == "" || strings.Contains(encoding, "IDENTITY")
}

func decodePDFString(raw string, font *pdfFont) string {
	bytesValue := []byte(raw)
	if len(bytesValue) == 0 {
		return ""
	}
	if len(bytesValue) >= 2 && bytesValue[0] == 0xFE && bytesValue[1] == 0xFF {
		return decodeUTF16BE(bytesValue[2:])
	}
	if len(bytesValue) >= 2 && bytesValue[0] == 0xFF && bytesValue[1] == 0xFE {
		return decodeUTF16LE(bytesValue[2:])
	}
	if font != nil {
		if font.cmap != nil && len(font.cmap.codes) > 0 {
			if text, ok := decodeWithCMap(bytesValue, font.cmap, font.twoByte || font.cmap.twoByte); ok {
				return text
			}
		}
		encoding := strings.ToUpper(font.encoding)
		switch {
		case strings.Contains(encoding, "UCS2"), strings.Contains(encoding, "UTF16"):
			if text := decodeUTF16BE(bytesValue); text != "" {
				return text
			}
		case strings.Contains(encoding, "GB"):
			if text := decodeWithEncoding(bytesValue, simplifiedchinese.GBK); text != "" {
				return text
			}
		case strings.Contains(encoding, "BIG5"), strings.Contains(encoding, "ETEN"):
			if text := decodeWithEncoding(bytesValue, traditionalchinese.Big5); text != "" {
				return text
			}
		case strings.Contains(encoding, "RKSJ"), strings.Contains(encoding, "SHIFT"):
			if text := decodeWithEncoding(bytesValue, japanese.ShiftJIS); text != "" {
				return text
			}
		}
		if font.twoByte {
			if text, ok := decodeCIDAsUnicode(bytesValue); ok {
				return text
			}
		}
	}
	if utf8.Valid(bytesValue) && hasMultiByteRune(bytesValue) {
		return string(bytesValue)
	}
	return decodeSimplePDFBytes(bytesValue)
}

func decodeWithCMap(raw []byte, cmap *pdfCMap, twoByte bool) (string, bool) {
	if twoByte && len(raw)%2 == 0 {
		if text, ok := decodeCMapWidth(raw, cmap, 2); ok {
			return text, true
		}
	}
	if text, ok := decodeCMapWidth(raw, cmap, 1); ok {
		return text, true
	}
	if !twoByte && len(raw)%2 == 0 {
		if text, ok := decodeCMapWidth(raw, cmap, 2); ok {
			return text, true
		}
	}
	return "", false
}

func decodeCMapWidth(raw []byte, cmap *pdfCMap, width int) (string, bool) {
	var builder strings.Builder
	total := 0
	mapped := 0
	for index := 0; index+width <= len(raw); index += width {
		total++
		var key string
		if width == 1 {
			key = fmt.Sprintf("%02x", raw[index])
		} else {
			key = fmt.Sprintf("%02x%02x", raw[index], raw[index+1])
		}
		if value, ok := cmap.codes[key]; ok {
			builder.WriteString(value)
			mapped++
			continue
		}
		if width == 1 && raw[index] >= 0x20 && raw[index] < 0x7F {
			builder.WriteByte(raw[index])
			mapped++
			continue
		}
		if width == 2 && raw[index] == 0 && raw[index+1] >= 0x20 && raw[index+1] < 0x7F {
			builder.WriteByte(raw[index+1])
			mapped++
		}
	}
	if total == 0 || mapped == 0 || mapped*2 < total {
		return "", false
	}
	return builder.String(), true
}

func decodeCIDAsUnicode(raw []byte) (string, bool) {
	if len(raw) == 0 || len(raw)%2 != 0 {
		return "", false
	}
	units := make([]uint16, 0, len(raw)/2)
	readable := 0
	cjk := 0
	for index := 0; index+1 < len(raw); index += 2 {
		unit := binary.BigEndian.Uint16(raw[index : index+2])
		units = append(units, unit)
		r := rune(unit)
		if isReadableRune(r) {
			readable++
		}
		if isCJKRune(r) {
			cjk++
		}
	}
	if len(units) == 0 || readable*4 < len(units)*3 {
		return "", false
	}
	if cjk == 0 && readable < len(units) {
		return "", false
	}
	return string(utf16.Decode(units)), true
}

func decodeWithEncoding(raw []byte, codec encoding.Encoding) string {
	decoded, err := codec.NewDecoder().Bytes(raw)
	if len(decoded) == 0 {
		return ""
	}
	if err != nil && !looksReadablePDFText(string(decoded)) {
		return ""
	}
	return string(decoded)
}

func decodeSimplePDFBytes(raw []byte) string {
	decoded, err := charmap.Windows1252.NewDecoder().Bytes(raw)
	if err != nil {
		return string(decoded)
	}
	return string(decoded)
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

func decodeUTF16LE(raw []byte) string {
	if len(raw)%2 == 1 {
		raw = append(raw, 0)
	}
	units := make([]uint16, 0, len(raw)/2)
	for index := 0; index+1 < len(raw); index += 2 {
		units = append(units, binary.LittleEndian.Uint16(raw[index:index+2]))
	}
	return string(utf16.Decode(units))
}

func hasMultiByteRune(value []byte) bool {
	for len(value) > 0 {
		r, size := utf8.DecodeRune(value)
		if r > 0x7F {
			return true
		}
		value = value[size:]
	}
	return false
}

func isReadableRune(r rune) bool {
	if r == '\n' || r == '\t' || r == ' ' {
		return true
	}
	if r == utf8.RuneError || unicode.IsControl(r) || isPrivateUseRune(r) {
		return false
	}
	return unicode.IsPrint(r)
}

func isCJKRune(r rune) bool {
	switch {
	case r >= 0x3000 && r <= 0x303F:
		return true
	case r >= 0x3400 && r <= 0x4DBF:
		return true
	case r >= 0x4E00 && r <= 0x9FFF:
		return true
	case r >= 0xF900 && r <= 0xFAFF:
		return true
	case r >= 0xFF00 && r <= 0xFFEF:
		return true
	default:
		return false
	}
}

func isPrivateUseRune(r rune) bool {
	return (r >= 0xE000 && r <= 0xF8FF) || (r >= 0xF0000 && r <= 0xFFFFD) || (r >= 0x100000 && r <= 0x10FFFD)
}

func cleanPDFText(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	lines := strings.Split(value, "\n")
	output := make([]string, 0, len(lines))
	blank := 0
	for _, line := range lines {
		builder := strings.Builder{}
		for _, r := range line {
			switch {
			case r == '\t':
				builder.WriteRune(' ')
			case r == utf8.RuneError, r == 0xFFFD, unicode.IsControl(r), isPrivateUseRune(r):
				continue
			default:
				builder.WriteRune(r)
			}
		}
		cleaned := collapsePDFSpaces(builder.String())
		if cleaned == "" {
			blank++
			if blank > 1 {
				continue
			}
			output = append(output, "")
			continue
		}
		blank = 0
		output = append(output, cleaned)
	}
	return strings.TrimSpace(strings.Join(output, "\n"))
}

func collapsePDFSpaces(value string) string {
	value = strings.TrimSpace(value)
	if !strings.Contains(value, "  ") {
		return value
	}
	builder := strings.Builder{}
	previousSpace := false
	for _, r := range value {
		space := r == ' '
		if space && previousSpace {
			continue
		}
		previousSpace = space
		builder.WriteRune(r)
	}
	return builder.String()
}

func uniqueStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func looksReadablePDFText(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return true
	}
	total := 0
	readable := 0
	for _, r := range value {
		total++
		if isReadableRune(r) {
			readable++
		}
	}
	return total == 0 || readable*10 >= total*6
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
