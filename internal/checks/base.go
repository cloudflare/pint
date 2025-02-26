package checks

import (
	"context"
	"errors"
	"fmt"

	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
)

var (
	CheckNames = []string{
		AlertsAbsentCheckName,
		AnnotationCheckName,
		AlertsCheckName,
		AlertsExternalLabelsCheckName,
		AlertForCheckName,
		TemplateCheckName,
		LabelsConflictCheckName,
		AggregationCheckName,
		ComparisonCheckName,
		ImpossibleCheckName,
		FragileCheckName,
		RangeQueryCheckName,
		RateCheckName,
		RegexpCheckName,
		SyntaxCheckName,
		VectorMatchingCheckName,
		CostCheckName,
		CounterCheckName,
		SeriesCheckName,
		RuleDependencyCheckName,
		RuleDuplicateCheckName,
		RuleForCheckName,
		RuleNameCheckName,
		LabelCheckName,
		RuleLinkCheckName,
		RejectCheckName,
		ReportCheckName,
	}
	OnlineChecks = []string{
		AlertsAbsentCheckName,
		AlertsCheckName,
		AlertsExternalLabelsCheckName,
		LabelsConflictCheckName,
		RangeQueryCheckName,
		RateCheckName,
		VectorMatchingCheckName,
		CostCheckName,
		CounterCheckName,
		SeriesCheckName,
		RuleLinkCheckName,
	}
)

// Severity of the problem reported.
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
	// Information doesn't count as a problem, it's a comment.
	Information Severity = iota

	// Warning is not consider an error.
	Warning

	// Bug is an error that should be corrected.
	Bug

	// Fatal is a problem with linting content.
	Fatal
)

type SettingsKey string

type Anchor uint8

const (
	AnchorAfter Anchor = iota
	AnchorBefore
)

type Problem struct {
	Reporter    string
	Summary     string
	Details     string
	Diagnostics []output.Diagnostic
	Lines       parser.LineRange
	Severity    Severity
	Anchor      Anchor
}

type CheckMeta struct {
	States        []discovery.ChangeType
	Online        bool
	AlwaysEnabled bool
}

type RuleChecker interface {
	String() string
	Reporter() string
	Meta() CheckMeta
	Check(_ context.Context, _ discovery.Path, rule parser.Rule, _ []discovery.Entry) []Problem
}

type exprProblem struct {
	summary  string
	details  string
	diags    []output.Diagnostic
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
		text = fmt.Sprintf("Couldn't run `%s` checks on %s because some queries are too expensive: `%s`.", reporter, promDesc, err)
		severity = Warning
	case promapi.IsUnavailableError(err):
		text = fmt.Sprintf("Couldn't run `%s` checks due to %s connection error: `%s`.", reporter, promDesc, err)
		severity = Warning
		if perrOk && perr.IsStrict() {
			severity = Bug
		}
	default:
		text = fmt.Sprintf("%s failed with: `%s`.", promDesc, err)
		severity = s
	}

	return text, severity
}

func promText(name, uri string) string {
	return fmt.Sprintf("`%s` Prometheus server at %s", name, uri)
}

func nodeLastColumn(node *parser.YamlNode) int {
	if node.Value == "" {
		return node.Column
	}
	return node.Column + len(node.Value) - 1
}
