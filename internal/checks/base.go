package checks

import (
	"fmt"

	"github.com/cloudflare/pint/internal/parser"
)

var (
	CheckNames = []string{
		AnnotationCheckName,
		AlertsCheckName,
		AlertForCheckName,
		TemplateCheckName,
		AggregationCheckName,
		ComparisonCheckName,
		RateCheckName,
		SyntaxCheckName,
		VectorMatchingCheckName,
		CostCheckName,
		SeriesCheckName,
		LabelCheckName,
		RejectCheckName,
	}
	OnlineChecks = []string{
		AlertsCheckName,
		RateCheckName,
		VectorMatchingCheckName,
		CostCheckName,
		SeriesCheckName,
	}
)

// Severity of the problem reported
type Severity int

func (s Severity) String() string {
	switch s {
	case Fatal:
		return "Fatal"
	case Information:
		return "Information"
	case Warning:
		return "Warning"
	default:
		return "Bug"
	}
}

func ParseSeverity(s string) (Severity, error) {
	switch s {
	case "fatal":
		return Fatal, nil
	case "bug":
		return Bug, nil
	case "info":
		return Information, nil
	case "warning":
		return Warning, nil
	default:
		return Fatal, fmt.Errorf("unknown severity: %s", s)
	}
}

const (
	// Information doesn't count as a problem, it's a comment
	Information Severity = iota

	// Warning is not consider an error
	Warning

	// Bug is an error that should be corrected
	Bug

	// Fatal is a problem with linting content
	Fatal
)

type Problem struct {
	Fragment string
	Lines    []int
	Reporter string
	Text     string
	Severity Severity
}

func (p Problem) LineRange() (int, int) {
	return p.Lines[0], p.Lines[len(p.Lines)-1]
}

type RuleChecker interface {
	String() string
	Check(rule parser.Rule) []Problem
}

type exprProblem struct {
	expr     string
	text     string
	severity Severity
}
