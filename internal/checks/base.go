package checks

import (
	"context"
	"errors"
	"fmt"
	"encoding/json"

	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
)

var (
	CheckNames = []string{
		AnnotationCheckName,
		AlertsCheckName,
		AlertForCheckName,
		TemplateCheckName,
		LabelsConflictCheckName,
		AggregationCheckName,
		ComparisonCheckName,
		FragileCheckName,
		RangeQueryCheckName,
		RateCheckName,
		RegexpCheckName,
		SyntaxCheckName,
		VectorMatchingCheckName,
		CostCheckName,
		SeriesCheckName,
		RuleDuplicateCheckName,
		RuleForCheckName,
		LabelCheckName,
		RuleLinkCheckName,
		RejectCheckName,
	}
	OnlineChecks = []string{
		AlertsCheckName,
		LabelsConflictCheckName,
		RangeQueryCheckName,
		RateCheckName,
		VectorMatchingCheckName,
		CostCheckName,
		SeriesCheckName,
		RuleDuplicateCheckName,
		RuleLinkCheckName,
	}
)

// Severity of the problem reported
type Severity int

func (s Severity) String() string {
	switch s {
	case Information:
		return "Information"
	case Warning:
		return "Warning"
	case Bug:
		return "Bug"
	case Fatal:
		return "Fatal"
	}
	return "Unknown"
}

func ParseSeverity(s string) (Severity, error) {
	switch s {
	case "fatal":
		return Fatal, nil
	case "bug":
		return Bug, nil
	case "warning":
		return Warning, nil
	case "info":
		return Information, nil
	default:
		return Fatal, fmt.Errorf("unknown severity: %s", s)
	}
}

func (s Severity) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
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

type SettingsKey string

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

type CheckMeta struct {
	IsOnline bool
}

type RuleChecker interface {
	String() string
	Reporter() string
	Meta() CheckMeta
	Check(_ context.Context, _ string, rule parser.Rule, _ []discovery.Entry) []Problem
}

type exprProblem struct {
	expr     string
	text     string
	severity Severity
}

func textAndSeverityFromError(err error, reporter, prom string, s Severity) (text string, severity Severity) {
	promDesc := fmt.Sprintf("%q", prom)
	var perr *promapi.FailoverGroupError
	perrOk := errors.As(err, &perr)
	if perrOk {
		if uri := perr.URI(); uri != "" {
			promDesc = promText(prom, uri)
		}
	}

	switch {
	case promapi.IsQueryTooExpensive(err):
		text = fmt.Sprintf("couldn't run %q checks on %s because some queries are too expensive: %s", reporter, promDesc, err)
		severity = Warning
	case promapi.IsUnavailableError(err):
		text = fmt.Sprintf("couldn't run %q checks due to %s connection error: %s", reporter, promDesc, err)
		severity = Warning
		if perrOk && perr.IsStrict() {
			severity = Bug
		}
	default:
		text = fmt.Sprintf("%s failed with: %s", promDesc, err)
		severity = s
	}

	return text, severity
}

func promText(name, uri string) string {
	return fmt.Sprintf("prometheus %q at %s", name, uri)
}
