package checks

import (
	"context"
	"errors"
	"fmt"

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
	if promapi.IsUnavailableError(err) {
		text = fmt.Sprintf("couldn't run %q checks due to %q connection error: %s", reporter, prom, err)
		var perr *promapi.FailoverGroupError
		if errors.As(err, &perr) {
			if uri := perr.URI(); uri != "" {
				text = fmt.Sprintf("couldn't run %q checks due to %s connection error: %s", reporter, promText(prom, uri), err)
			}
			if perr.IsStrict() {
				severity = Bug
			} else {
				severity = Warning
			}
		} else {
			severity = Warning
		}
	} else {
		text = fmt.Sprintf("%q failed with: %s", prom, err)
		var perr *promapi.FailoverGroupError
		if errors.As(err, &perr) {
			text = fmt.Sprintf("%s failed with: %s", promText(prom, perr.URI()), err)
		}
		severity = s
	}
	return text, severity
}

func promText(name, uri string) string {
	return fmt.Sprintf("prometheus %q at %s", name, uri)
}
