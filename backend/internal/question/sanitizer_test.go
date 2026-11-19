package question_test

import (
	"strings"
	"testing"

	"school-exam-studio/backend/internal/question"
)

func TestSanitizerAllowsMathAndTablesButRemovesActiveContent(t *testing.T) {
	sanitizer := question.NewSanitizer()
	input := `<p onclick="alert(1)">已知<script>alert(1)</script><img src=x onerror=alert(1)><iframe src="https://evil.example"></iframe>` +
		`<span data-type="inlineMath" data-latex="x^2" onmouseover="alert(1)"></span>` +
		`<table><tbody><tr><td colspan="2">答案</td></tr></tbody></table></p>`
	got := sanitizer.Sanitize(input)
	for _, allowed := range []string{"<p>", "已知", `data-type="inlineMath"`, `data-latex="x^2"`, "<table>", `colspan="2"`} {
		if !strings.Contains(got, allowed) {
			t.Fatalf("sanitized HTML lost %q: %s", allowed, got)
		}
	}
	for _, forbidden := range []string{"<script", "<img", "<iframe", "onclick", "onerror", "onmouseover", "evil.example"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("sanitized HTML contains %q: %s", forbidden, got)
		}
	}
}

func TestQuestionInputValidation(t *testing.T) {
	sanitizer := question.NewSanitizer()
	tests := []struct {
		name   string
		mutate func(*question.Input)
	}{
		{"empty content", func(input *question.Input) { input.Content = `<script>alert(1)</script>` }},
		{"markup-only content", func(input *question.Input) { input.Content = `<p> <br> </p>` }},
		{"difficulty", func(input *question.Input) { input.Difficulty = 6 }},
		{"single choice options", func(input *question.Input) { input.Options = input.Options[:1] }},
		{"single choice answer", func(input *question.Input) { input.Answer = "C" }},
		{"multiple duplicate answer", func(input *question.Input) {
			input.QuestionType = question.TypeMultipleChoice
			input.Answer = "A,A"
		}},
		{"multiple illegal answer", func(input *question.Input) {
			input.QuestionType = question.TypeMultipleChoice
			input.Answer = "A,C"
		}},
		{"true false answer", func(input *question.Input) {
			input.QuestionType = question.TypeTrueFalse
			input.Options = nil
			input.Answer = "yes"
		}},
		{"true false options", func(input *question.Input) {
			input.QuestionType = question.TypeTrueFalse
			input.Answer = "true"
		}},
		{"empty tag", func(input *question.Input) { input.Tags = []string{"代数", "  "} }},
		{"too many tags", func(input *question.Input) {
			input.Tags = []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11"}
		}},
		{"duplicate option", func(input *question.Input) { input.Options[1].Key = "A" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validQuestionInput()
			test.mutate(&input)
			if _, err := sanitizer.Normalize(input); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
	input := validQuestionInput()
	input.Tags = []string{" 代数 ", "代数", "方程"}
	normalized, err := sanitizer.Normalize(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(normalized.Tags) != 2 || normalized.Tags[0] != "代数" || normalized.Tags[1] != "方程" {
		t.Fatalf("tags were not trimmed and deduplicated: %#v", normalized.Tags)
	}
	formulaOnly := validQuestionInput()
	formulaOnly.Content = `<span data-type="blockMath" data-latex="x^2"></span>`
	if _, err := sanitizer.Normalize(formulaOnly); err != nil {
		t.Fatalf("formula-only content should be accepted: %v", err)
	}
	kebabFormula := validQuestionInput()
	kebabFormula.Content = `<span data-type="inline-math" data-latex="x^2"></span>`
	kebabNormalized, err := sanitizer.Normalize(kebabFormula)
	if err != nil {
		t.Fatalf("kebab-case formula-only content should be accepted: %v", err)
	}
	if !strings.Contains(kebabNormalized.Content, `data-type="inline-math"`) {
		t.Fatalf("kebab-case formula data-type was lost: %s", kebabNormalized.Content)
	}
}

func validQuestionInput() question.Input {
	return question.Input{
		Subject:      "数学",
		Grade:        "七年级",
		QuestionType: question.TypeSingleChoice,
		Difficulty:   2,
		Tags:         []string{"代数"},
		Content:      "<p>1+1=?</p>",
		Options:      []question.Option{{Key: "A", Content: "1"}, {Key: "B", Content: "2"}},
		Answer:       "B",
		Explanation:  "<p>基础加法</p>",
	}
}

func TestSanitizerNormalizesTextMathIntoEditorNodes(t *testing.T) {
	sanitizer := question.NewSanitizer()
	input := validQuestionInput()
	input.Content = `<p>已知 $x^2$，求 \[y+1\] 的值</p>`
	input.Explanation = `<p>由 $$a^2+b^2=c^2$$ 可得 $c$</p>`
	normalized, err := sanitizer.Normalize(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		`data-type="inlineMath" data-latex="x^2"`,
		`data-type="blockMath" data-latex="y+1"`,
		`data-type="blockMath" data-latex="a^2+b^2=c^2"`,
		`data-type="inlineMath" data-latex="c"`,
	} {
		if !strings.Contains(normalized.Content+normalized.Explanation, expected) {
			t.Fatalf("math markup %q missing: %s | %s", expected, normalized.Content, normalized.Explanation)
		}
	}

	codeInput := validQuestionInput()
	codeInput.Content = `<p><code>$x^2$</code> 保持原样</p>`
	codeNormalized, err := sanitizer.Normalize(codeInput)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(codeNormalized.Content, "$x^2$") || strings.Contains(codeNormalized.Content, "data-latex") {
		t.Fatalf("code block formula should stay literal: %s", codeNormalized.Content)
	}

	unclosed := validQuestionInput()
	unclosed.Content = `<p>价格 $5 未闭合</p>`
	unclosedNormalized, err := sanitizer.Normalize(unclosed)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(unclosedNormalized.Content, "data-latex") {
		t.Fatalf("unclosed delimiter should stay literal: %s", unclosedNormalized.Content)
	}
}

func TestSanitizerDropsTagsDuplicatingCategoryAndKnowledgePoints(t *testing.T) {
	sanitizer := question.NewSanitizer()
	input := validQuestionInput()
	input.Category = "数与运算"
	input.KnowledgePoints = []string{"有理数加法"}
	input.Tags = []string{"数与运算", " 有理数加法 ", "加法", "加法", "ADD", "add"}
	normalized, err := sanitizer.Normalize(input)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"加法", "ADD"}
	if len(normalized.Tags) != len(want) {
		t.Fatalf("tags = %#v, want %#v", normalized.Tags, want)
	}
	for index := range want {
		if normalized.Tags[index] != want[index] {
			t.Fatalf("tags = %#v, want %#v", normalized.Tags, want)
		}
	}
}
