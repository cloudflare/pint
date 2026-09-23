package checks

import (
	"context"
	"fmt"

	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
)

const (
	RuleNameCheckName = "rule/name"
)

func NewRuleNameCheck(re *TemplatedRegexp, comment string, severity Severity) RuleNameCheck {
	return RuleNameCheck{
		re:       re,
		comment:  comment,
		severity: severity,
		instance: fmt.Sprintf("%s(%s)", RuleNameCheckName, re.anchored),
	}
}

type RuleNameCheck struct {
	re       *TemplatedRegexp
	comment  string
	instance string
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
		AlwaysEnabled: false,
	}
}

func (c RuleNameCheck) String() string {
	return c.instance
}

func (c RuleNameCheck) Reporter() string {
	return RuleNameCheckName
}

func (c RuleNameCheck) Check(_ context.Context, entry *discovery.Entry, _ []*discovery.Entry) (problems []Problem) {
	if entry.Rule.AlertingRule != nil && !c.re.MustExpand(entry.Rule).MatchString(entry.Rule.AlertingRule.Alert.Value) {
		problems = append(problems, Problem{
			Anchor:   AnchorAfter,
			Lines:    entry.Rule.AlertingRule.Alert.Pos.Lines(),
			Reporter: c.Reporter(),
			Summary:  "name not allowed",
			Details:  maybeComment(c.comment),
			Diagnostics: []diags.Diagnostic{
				{
					Message:     fmt.Sprintf("alerting rule name must match `%s`.", c.re.anchored),
					Pos:         entry.Rule.AlertingRule.Alert.Pos,
					Expr:        nil,
					FirstColumn: 1,
					LastColumn:  len(entry.Rule.AlertingRule.Alert.Value),
					Kind:        diags.Issue,
				},
			},
			Severity: c.severity,
		})
	}
	if entry.Rule.RecordingRule != nil && !c.re.MustExpand(entry.Rule).MatchString(entry.Rule.RecordingRule.Record.Value) {
		problems = append(problems, Problem{
			Anchor:   AnchorAfter,
			Lines:    entry.Rule.RecordingRule.Record.Pos.Lines(),
			Reporter: c.Reporter(),
			Summary:  "name not allowed",
			Details:  maybeComment(c.comment),
			Diagnostics: []diags.Diagnostic{
				{
					Message:     fmt.Sprintf("recording rule name must match `%s`.", c.re.anchored),
					Pos:         entry.Rule.RecordingRule.Record.Pos,
					Expr:        nil,
					FirstColumn: 1,
					LastColumn:  len(entry.Rule.RecordingRule.Record.Value),
					Kind:        diags.Issue,
				},
			},
			Severity: c.severity,
		})
	}
	return problems
}
