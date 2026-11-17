package document

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

const maxDocxXMLBytes = 32 << 20

func hasZipEntry(data []byte, name string) bool {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return false
	}
	for _, file := range reader.File {
		if strings.EqualFold(file.Name, name) {
			return true
		}
	}
	return false
}

func extractDOCX(data []byte) (Result, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return Result{}, fmt.Errorf("无法读取 DOCX 压缩包：%w", err)
	}
	var documentFile *zip.File
	for _, file := range reader.File {
		if strings.EqualFold(file.Name, "word/document.xml") {
			documentFile = file
			break
		}
	}
	if documentFile == nil {
		return Result{}, fmt.Errorf("DOCX 缺少 word/document.xml")
	}
	stream, err := documentFile.Open()
	if err != nil {
		return Result{}, fmt.Errorf("无法读取 DOCX 正文：%w", err)
	}
	defer stream.Close()
	payload, err := io.ReadAll(io.LimitReader(stream, maxDocxXMLBytes+1))
	if err != nil {
		return Result{}, fmt.Errorf("无法读取 DOCX 正文：%w", err)
	}
	if len(payload) > maxDocxXMLBytes {
		return Result{}, fmt.Errorf("DOCX 正文超过 %d 字节限制", maxDocxXMLBytes)
	}
	text, err := docxPlainText(payload)
	if err != nil {
		return Result{}, err
	}
	return Result{
		Text:     text,
		Kind:     "docx",
		Pages:    0,
		Warnings: []string{},
	}, nil
}

type docxTextState struct {
	builder   strings.Builder
	inText    bool
	skipText  int
	cellDepth int
}

func docxPlainText(payload []byte) (string, error) {
	decoder := xml.NewDecoder(bytes.NewReader(payload))
	state := &docxTextState{}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("DOCX 正文 XML 解析失败：%w", err)
		}
		switch element := token.(type) {
		case xml.StartElement:
			switch element.Name.Local {
			case "t":
				if state.skipText == 0 {
					state.inText = true
				}
			case "instrText", "delText":
				state.skipText++
			case "tab":
				state.builder.WriteString("\t")
			case "br", "cr":
				state.builder.WriteString("\n")
			case "tabl", "tbl":
				state.cellDepth++
			}
		case xml.EndElement:
			switch element.Name.Local {
			case "t":
				state.inText = false
			case "instrText", "delText":
				if state.skipText > 0 {
					state.skipText--
				}
			case "p":
				state.builder.WriteString("\n")
			case "tc":
				state.builder.WriteString("\t")
			case "tr":
				state.builder.WriteString("\n")
			case "tabl", "tbl":
				if state.cellDepth > 0 {
					state.cellDepth--
				}
			}
		case xml.CharData:
			if state.inText {
				state.builder.Write([]byte(element))
			}
		}
	}
	return state.builder.String(), nil
}
