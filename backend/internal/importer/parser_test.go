package importer_test

import (
	"bytes"
	"encoding/csv"
	"errors"
	"strings"
	"testing"

	"school-exam-studio/backend/internal/importer"
	"school-exam-studio/backend/internal/question"
)

func TestParseCSVBOMTagsAllTypesMathAndXSS(t *testing.T) {
	records := [][]string{validCSVRecord("单选题")}
	records[0][4] = "代数；方程; 代数"
	records[0][5] = `<strong>求 $x^2$</strong><script>alert(1)</script><img src=x onerror=alert(2)>`
	records[0][6] = `$a<b$`
	multiple := validCSVRecord("多选题")
	multiple[10] = "A, C"
	records = append(records, multiple)
	judgement := validCSVRecord("判断题")
	judgement[6], judgement[7], judgement[8], judgement[9] = "", "", "", ""
	judgement[10] = "正确"
	records = append(records, judgement)
	fill := validCSVRecord("填空题")
	fill[6], fill[7], fill[8], fill[9] = "", "", "", ""
	fill[10] = `$x+1$`
	records = append(records, fill)
	essay := validCSVRecord("解答题")
	essay[6], essay[7], essay[8], essay[9] = "", "", "", ""
	essay[10] = "写出完整过程"
	essay[11] = `使用 $$x=\frac{-b\pm\sqrt{b^2-4ac}}{2a}$$`
	records = append(records, essay)

	rows, rowErrors, err := importer.ParseCSV(bytes.NewReader(csvBytes(t, records, true)))
	if err != nil || len(rowErrors) != 0 || len(rows) != 5 {
		t.Fatalf("parse failed: rows=%d errors=%+v err=%v", len(rows), rowErrors, err)
	}
	if len(rows[0].Tags) != 2 || rows[0].Tags[0] != "代数" || rows[0].Tags[1] != "方程" {
		t.Fatalf("tags were not normalized: %#v", rows[0].Tags)
	}
	if !strings.Contains(rows[0].Content, `data-type="inlineMath"`) || !strings.Contains(rows[0].Content, `data-latex="x^2"`) || !strings.Contains(rows[0].Content, "<strong>") {
		t.Fatalf("inline formula or safe markup lost: %s", rows[0].Content)
	}
	for _, forbidden := range []string{"<script", "alert(", "<img", "onerror"} {
		if strings.Contains(rows[0].Content, forbidden) {
			t.Fatalf("unsafe content %q survived: %s", forbidden, rows[0].Content)
		}
	}
	if !strings.Contains(rows[0].Options[0].Content, "inlineMath") || !strings.Contains(rows[0].Options[0].Content, "a&lt;b") {
		t.Fatalf("option formula was not safely converted: %s", rows[0].Options[0].Content)
	}
	if rows[1].Type != question.TypeMultipleChoice || rows[1].Answer != "A,C" || rows[2].Answer != "true" {
		t.Fatalf("answers were not normalized: %+v %+v", rows[1], rows[2])
	}
	if !strings.Contains(rows[3].Answer, "inlineMath") || !strings.Contains(rows[4].Explanation, `data-type="blockMath"`) {
		t.Fatalf("text answer or block formula lost: %+v %+v", rows[3], rows[4])
	}
}

func TestParseCSVTrueFalseAliasesAndInvalidRows(t *testing.T) {
	for input, expected := range map[string]string{"true": "true", "false": "false", "正确": "true", "错误": "false"} {
		record := validCSVRecord("判断题")
		record[6], record[7], record[8], record[9] = "", "", "", ""
		record[10] = input
		rows, rowErrors, err := importer.ParseCSV(bytes.NewReader(csvBytes(t, [][]string{record}, false)))
		if err != nil || len(rowErrors) != 0 || rows[0].Answer != expected {
			t.Fatalf("true/false alias %q: rows=%+v errors=%+v err=%v", input, rows, rowErrors, err)
		}
	}
	tests := []struct {
		name   string
		mutate func([]string)
		field  string
	}{
		{"difficulty", func(record []string) { record[3] = "6" }, "difficulty"},
		{"single answer", func(record []string) { record[10] = "E" }, "answer"},
		{"multiple answer", func(record []string) { record[0] = "多选题"; record[10] = "A,A" }, "answer"},
		{"missing option", func(record []string) { record[7], record[8], record[9], record[10] = "", "", "", "A" }, "options"},
		{"empty fill answer", func(record []string) {
			record[0] = "填空题"
			record[6], record[7], record[8], record[9], record[10] = "", "", "", "", ""
		}, "answer"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record := validCSVRecord("单选题")
			test.mutate(record)
			rows, rowErrors, err := importer.ParseCSV(bytes.NewReader(csvBytes(t, [][]string{record}, false)))
			if err != nil || len(rows) != 0 || len(rowErrors) != 1 || rowErrors[0].Line != 2 || rowErrors[0].Field != test.field {
				t.Fatalf("unexpected validation: rows=%+v errors=%+v err=%v", rows, rowErrors, err)
			}
		})
	}
}

func TestParseCSVStrictHeaderRowAndSizeLimits(t *testing.T) {
	badHeader := strings.Join(expectedTestHeader[:len(expectedTestHeader)-1], ",") + ",错误表头\n"
	_, rowErrors, err := importer.ParseCSV(strings.NewReader(badHeader))
	if err != nil || len(rowErrors) != 1 || rowErrors[0].Line != 1 || rowErrors[0].Field != "header" {
		t.Fatalf("strict header not enforced: %+v %v", rowErrors, err)
	}
	records := make([][]string, 1001)
	for index := range records {
		records[index] = validCSVRecord("单选题")
	}
	_, rowErrors, err = importer.ParseCSV(bytes.NewReader(csvBytes(t, records, false)))
	if err != nil || len(rowErrors) != 1 || rowErrors[0].Line != 1002 {
		t.Fatalf("1000-row limit not enforced: %+v %v", rowErrors, err)
	}
	if _, _, err := importer.ParseCSV(bytes.NewReader(make([]byte, importer.MaxFileSize+1))); !errors.Is(err, importer.ErrFileTooLarge) {
		t.Fatalf("5 MiB limit not enforced: %v", err)
	}
}

var expectedTestHeader = []string{"题型", "年级", "学科", "难度", "知识点标签", "题干", "选项A", "选项B", "选项C", "选项D", "答案", "解析"}

func validCSVRecord(questionType string) []string {
	return []string{questionType, "七年级", "数学", "2", "代数", "1+1=?", "1", "2", "3", "4", "B", "基础加法"}
}

func csvBytes(t *testing.T, records [][]string, bom bool) []byte {
	t.Helper()
	var output bytes.Buffer
	if bom {
		output.Write([]byte{0xef, 0xbb, 0xbf})
	}
	writer := csv.NewWriter(&output)
	if err := writer.Write(expectedTestHeader); err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		if err := writer.Write(record); err != nil {
			t.Fatal(err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}
