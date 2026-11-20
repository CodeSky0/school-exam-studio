package document

import (
	"strings"
	"testing"
)

func TestSplitQuestionsRecognizesNumberedItemsAndAnswers(t *testing.T) {
	text := "一、选择题\n1. 下列说法正确的是（ ）\nA. 甲\nB. 乙\nC. 丙\nD. 丁\n答案：A\n解析：乙不符合题意。\n2. 计算 2+2 的值。\n答案：4"
	items := SplitQuestions(text, Defaults{Subject: "数学", Grade: "七年级"})
	if len(items) != 2 {
		t.Fatalf("items = %d", len(items))
	}
	first := items[0]
	if first.QuestionType != "single_choice" {
		t.Fatalf("type = %q", first.QuestionType)
	}
	if first.Answer != "A" {
		t.Fatalf("answer = %q", first.Answer)
	}
	if len(first.Options) != 4 {
		t.Fatalf("options = %d", len(first.Options))
	}
	if !strings.Contains(first.Content, "下列说法正确的是") {
		t.Fatalf("content = %q", first.Content)
	}
	if !strings.Contains(first.Explanation, "乙不符合题意") {
		t.Fatalf("explanation = %q", first.Explanation)
	}
	second := items[1]
	if second.QuestionType != "essay" && second.QuestionType != "fill_blank" {
		t.Fatalf("second type = %q", second.QuestionType)
	}
	if second.Answer != "<p>4</p>" {
		t.Fatalf("second answer = %q", second.Answer)
	}
}

func TestSplitQuestionsRecognizesTrueFalse(t *testing.T) {
	text := "二、判断题\n1. 地球是圆的。\n答案：正确\n2. 1+1=3。\n答案：错误"
	items := SplitQuestions(text, Defaults{})
	if len(items) != 2 {
		t.Fatalf("items = %d", len(items))
	}
	if items[0].QuestionType != "true_false" || items[0].Answer != "true" {
		t.Fatalf("first = %#v", items[0])
	}
	if items[1].Answer != "false" {
		t.Fatalf("second answer = %q", items[1].Answer)
	}
}

func TestSplitQuestionsFallsBackToBlankLines(t *testing.T) {
	text := "第一道没有题号的题目\n答案：甲\n\n第二道没有题号的题目\n答案：乙"
	items := SplitQuestions(text, Defaults{})
	if len(items) != 2 {
		t.Fatalf("items = %d", len(items))
	}
	for _, item := range items {
		if !strings.Contains(strings.Join(item.Warnings, " "), "未识别到题号") {
			t.Fatalf("warnings = %#v", item.Warnings)
		}
	}
}

func TestSplitQuestionsMarksMissingAnswer(t *testing.T) {
	text := "1. 没有答案的题目\n2. 另一道题\n答案：A"
	items := SplitQuestions(text, Defaults{})
	if len(items) != 2 {
		t.Fatalf("items = %d", len(items))
	}
	if !strings.Contains(strings.Join(items[0].Warnings, " "), "未识别到答案") {
		t.Fatalf("warnings = %#v", items[0].Warnings)
	}
}

func TestSplitQuestionsKeepsInlineOptions(t *testing.T) {
	text := "1. 选出正确答案（ ） A. 甲 B. 乙 C. 丙 D. 丁\n答案：B"
	items := SplitQuestions(text, Defaults{})
	if len(items) != 1 {
		t.Fatalf("items = %d", len(items))
	}
	if len(items[0].Options) != 4 {
		t.Fatalf("options = %d", len(items[0].Options))
	}
	if items[0].Answer != "B" {
		t.Fatalf("answer = %q", items[0].Answer)
	}
}

func TestSplitQuestionsParsesOptionsOnOneOptionLine(t *testing.T) {
	text := "1. 题干（ ）\nA．甲 B．乙 C．丙 D．丁\n答案：B"
	items := SplitQuestions(text, Defaults{})
	if len(items) != 1 {
		t.Fatalf("items = %d", len(items))
	}
	if len(items[0].Options) != 4 {
		t.Fatalf("options = %d", len(items[0].Options))
	}
	if items[0].Answer != "B" {
		t.Fatalf("answer = %q", items[0].Answer)
	}
}

func TestSplitQuestionsKeepsSectionTypePerBlock(t *testing.T) {
	text := "一、选择题\n1. 第一题（ ）\nA. 甲\nB. 乙\nC. 丙\nD. 丁\n答案：A\n2. 第二题（ ）\nA. 甲\nB. 乙\nC. 丙\nD. 丁\n答案：B\n二、填空题\n3. 第三题 ____\n答案：12"
	items := SplitQuestions(text, Defaults{})
	if len(items) != 3 {
		t.Fatalf("items = %d", len(items))
	}
	if items[0].QuestionType != "single_choice" || items[1].QuestionType != "single_choice" {
		t.Fatalf("choice types = %q, %q", items[0].QuestionType, items[1].QuestionType)
	}
	if items[2].QuestionType != "fill_blank" {
		t.Fatalf("fill type = %q", items[2].QuestionType)
	}
}

func TestSplitQuestionsReadsAnswerSection(t *testing.T) {
	text := "一、选择题\n1. 第一题（ ）\nA. 甲\nB. 乙\nC. 丙\nD. 丁\n2. 第二题（ ）\nA. 甲\nB. 乙\nC. 丙\nD. 丁\n参考答案\n1. A\n2. B\n解析：乙符合题意。"
	items := SplitQuestions(text, Defaults{})
	if len(items) != 2 {
		t.Fatalf("items = %d", len(items))
	}
	if items[0].Answer != "A" || items[1].Answer != "B" {
		t.Fatalf("answers = %q, %q", items[0].Answer, items[1].Answer)
	}
	if !strings.Contains(items[1].Explanation, "乙符合题意") {
		t.Fatalf("explanation = %q", items[1].Explanation)
	}
	if strings.Contains(strings.Join(items[0].Warnings, " "), "未识别到答案") {
		t.Fatalf("warnings = %#v", items[0].Warnings)
	}
}

func TestSplitQuestionsReadsCompactAnswerRange(t *testing.T) {
	text := "一、选择题\n1. 第一题（ ）\nA. 甲\nB. 乙\nC. 丙\nD. 丁\n2. 第二题（ ）\nA. 甲\nB. 乙\nC. 丙\nD. 丁\n参考答案\n1-2 AB"
	items := SplitQuestions(text, Defaults{})
	if len(items) != 2 {
		t.Fatalf("items = %d", len(items))
	}
	if items[0].Answer != "A" || items[1].Answer != "B" {
		t.Fatalf("answers = %q, %q", items[0].Answer, items[1].Answer)
	}
}

func TestSplitQuestionsSplitsInlineExplanationMarker(t *testing.T) {
	text := "1. 第一题（ ）\nA. 甲\nB. 乙\nC. 丙\nD. 丁\n答案：A【解析】甲符合题意。"
	items := SplitQuestions(text, Defaults{})
	if len(items) != 1 {
		t.Fatalf("items = %d", len(items))
	}
	if items[0].Answer != "A" {
		t.Fatalf("answer = %q", items[0].Answer)
	}
	if strings.Contains(items[0].Explanation, "】") || !strings.Contains(items[0].Explanation, "甲符合题意") {
		t.Fatalf("explanation = %q", items[0].Explanation)
	}
}
