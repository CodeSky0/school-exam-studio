package paper

import (
	"errors"
	"math"
	"regexp"
	"strconv"
	"strings"
)

const (
	MaximumScore       = "999999.99"
	maximumScoreCents  = int64(99999999)
)

var (
	ErrInvalidScore = errors.New("invalid score")
	scorePattern    = regexp.MustCompile(`^(0|[1-9][0-9]*)(\.[0-9]{1,2})?$`)
)

// Score stores an exact number of cents and never uses binary floating point.
type Score struct {
	cents int64
}

func ParseScore(value string) (Score, error) {
	if !scorePattern.MatchString(value) {
		return Score{}, ErrInvalidScore
	}
	parts := strings.SplitN(value, ".", 2)
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return Score{}, ErrInvalidScore
	}
	var fraction int64
	if len(parts) == 2 {
		fractionText := parts[1]
		if len(fractionText) == 1 {
			fractionText += "0"
		}
		fraction, err = strconv.ParseInt(fractionText, 10, 64)
		if err != nil {
			return Score{}, ErrInvalidScore
		}
	}
	if whole > (math.MaxInt64-fraction)/100 {
		return Score{}, ErrInvalidScore
	}
	return Score{cents: whole*100 + fraction}, nil
}

func SumScores(scores []Score) Score {
	var total int64
	for _, score := range scores {
		if score.cents > 0 && total > math.MaxInt64-score.cents {
			return Score{cents: math.MaxInt64}
		}
		total += score.cents
	}
	return Score{cents: total}
}

func (s Score) String() string {
	return strconv.FormatInt(s.cents/100, 10) + "." + leftPadTwo(s.cents%100)
}

func (s Score) isPositive() bool {
	return s.cents > 0
}

func (s Score) exceedsMaximum() bool {
	return s.cents > maximumScoreCents
}

func leftPadTwo(value int64) string {
	if value < 10 {
		return "0" + strconv.FormatInt(value, 10)
	}
	return strconv.FormatInt(value, 10)
}
