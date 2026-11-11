package paper_test

import (
	"testing"

	"school-exam-studio/backend/internal/paper"
)

func TestSumScoresWithoutFloatingPointError(t *testing.T) {
	first, err := paper.ParseScore("0.10")
	if err != nil {
		t.Fatal(err)
	}
	second, err := paper.ParseScore("0.20")
	if err != nil {
		t.Fatal(err)
	}
	if got := paper.SumScores([]paper.Score{first, second}).String(); got != "0.30" {
		t.Fatalf("expected 0.30, got %s", got)
	}
}

func TestParseScoreNormalizesAndRejectsInvalidPrecision(t *testing.T) {
	score, err := paper.ParseScore("12.3")
	if err != nil || score.String() != "12.30" {
		t.Fatalf("score normalization = %q, err=%v", score.String(), err)
	}
	for _, value := range []string{"", "-1.00", "0.001", "1e2", "01.00", "NaN"} {
		if _, err := paper.ParseScore(value); err == nil {
			t.Fatalf("ParseScore(%q) should fail", value)
		}
	}
}
