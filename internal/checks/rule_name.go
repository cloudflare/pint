package checks

import (
	"context"
	"fmt"

	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
)

const (
	RuleNameCheckName = "rule/name"
)

func NewRuleNameCheck(re *TemplatedRegexp, comment string, severity Severity) RuleNameCheck {
	return RuleNameCheck{
		re:       re,
		comment:  comment,
		severity: severity,
	}
}

type RuleNameCheck struct {
	re       *TemplatedRegexp
	comment  string
	severity Severity
}

func (c RuleNameCheck) Meta() CheckMeta {
	return CheckMeta{
		States: []discovery.ChangeType{
			discovery.Noop,
			discovery.Added,
			discovery.Modified,
			discovery.Moved,
		},
		IsOnline: false,
	}
}

func (c RuleNameCheck) String() string {
	return fmt.Sprintf("%s(%s)", RuleNameCheckName, c.re.anchored)
}

func (c RuleNameCheck) Reporter() string {
	return RuleNameCheckName
}

func (c RuleNameCheck) Check(_ context.Context, _ discovery.Path, rule parser.Rule, _ []discovery.Entry) (problems []Problem) {
	if rule.AlertingRule != nil && !c.re.MustExpand(rule).MatchString(rule.AlertingRule.Alert.Value) {
		problems = append(problems, Problem{
			Lines:    rule.AlertingRule.Alert.Lines,
			Reporter: c.Reporter(),
			Text:     fmt.Sprintf("alerting rule name must match `%s`.", c.re.anchored),
			Details:  maybeComment(c.comment),
			Severity: c.severity,
		})
	}
	if rule.RecordingRule != nil && !c.re.MustExpand(rule).MatchString(rule.RecordingRule.Record.Value) {
		problems = append(problems, Problem{
			Lines:    rule.RecordingRule.Record.Lines,
			Reporter: c.Reporter(),
			Text:     fmt.Sprintf("recording rule name must match `%s`.", c.re.anchored),
			Details:  maybeComment(c.comment),
			Severity: c.severity,
		})
	}
	return problems
}
