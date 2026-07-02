package checks

import (
	"context"
	"fmt"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	promParser "github.com/prometheus/prometheus/promql/parser"

	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/parser/source"
	"github.com/cloudflare/pint/internal/promapi"
)

const (
	RuleDuplicateCheckName = "rule/duplicate"
)

func NewRuleDuplicateCheck(prom *promapi.FailoverGroup) RuleDuplicateCheck {
	return RuleDuplicateCheck{
		prom:     prom,
		instance: fmt.Sprintf("%s(%s)", RuleDuplicateCheckName, prom.Name()),
	}
}

type RuleDuplicateCheck struct {
	prom     *promapi.FailoverGroup
	instance string
}

func (c RuleDuplicateCheck) Meta() CheckMeta {
	return CheckMeta{
		States: []discovery.ChangeType{
			discovery.Noop,
			discovery.Added,
			discovery.Modified,
			discovery.Moved,
		},
		Online:        false,
		AlwaysEnabled: false,
	}
}

func (c RuleDuplicateCheck) String() string {
	return c.instance
}

func (c RuleDuplicateCheck) Reporter() string {
	return RuleDuplicateCheckName
}

func (c RuleDuplicateCheck) Check(ctx context.Context, entry *discovery.Entry, entries []*discovery.Entry) (problems []Problem) {
	expr := entry.Rule.Expr()
	if expr.SyntaxError() != nil {
		return problems
	}

	for _, other := range entries {
		if ignoreOtherEntry(entry, other, c.prom) {
			continue
		}

		if entry.Rule.RecordingRule != nil && other.Rule.RecordingRule != nil &&
			other.Rule.RecordingRule.Record.Value == entry.Rule.RecordingRule.Record.Value {
			problems = append(problems, c.compareRules(ctx, entry.Rule.RecordingRule, other, entry.Rule.Lines)...)
		}

		if entry.Rule.AlertingRule != nil && other.Rule.AlertingRule != nil &&
			other.Rule.AlertingRule.Alert.Value == entry.Rule.AlertingRule.Alert.Value {
			problems = append(problems, c.compareAlertingRules(entry.Rule.AlertingRule, other, entry.Rule.Lines)...)
		}
	}
	return problems
}

func (c RuleDuplicateCheck) compareAlertingRules(rule *parser.AlertingRule, entry *discovery.Entry, lines diags.LineRange) (problems []Problem) {
	other := entry.Rule.AlertingRule

	if !rule.Labels.IsIdentical(other.Labels) {
		return nil
	}

	var msg string
	var severity Severity
	switch {
	case rule.Expr.Query().Expr.String() == other.Expr.Query().Expr.String():
		msg = fmt.Sprintf(
			"Duplicated rule, identical rule found in the `%s` alert at `%s:%d`.",
			other.Alert.Value, entry.Path.SymlinkTarget, other.Alert.Pos.Lines().First,
		)
		severity = Bug
	case exprCanOverlap(rule.Expr, other.Expr):
		msg = fmt.Sprintf(
			"This rule uses a different query but it can select the same time series as the `%s` alert at `%s:%d`, which means both rules will create identical alerts that might be deduplicated, with no way to distinguish between them.",
			other.Alert.Value, entry.Path.SymlinkTarget, other.Alert.Pos.Lines().First,
		)
		severity = Warning
	default:
		return nil
	}

	return append(problems, Problem{
		Anchor:   AnchorAfter,
		Lines:    lines,
		Reporter: c.Reporter(),
		Summary:  "duplicated alerting rule",
		Details:  "",
		Severity: severity,
		Diagnostics: []diags.Diagnostic{
			{
				Message:     msg,
				Pos:         rule.Alert.Pos,
				Expr:        nil,
				FirstColumn: 1,
				LastColumn:  len(rule.Alert.Value),
				Kind:        diags.Issue,
			},
		},
	})
}

func exprCanOverlap(a, b parser.PromQLExpr) bool {
	aSelectors := vectorSelectors(a)
	bSelectors := vectorSelectors(b)

	// We can only reason reliably about a single selector on each side.
	if len(aSelectors) != 1 || len(bSelectors) != 1 {
		return false
	}

	aName := metricName(aSelectors[0])
	bName := metricName(bSelectors[0])
	if aName == "" || bName == "" || aName != bName {
		return false
	}

	return !selectorsMismatch(aSelectors[0], bSelectors[0])
}

func vectorSelectors(expr parser.PromQLExpr) (selectors []*promParser.VectorSelector) {
	for _, s := range expr.Source() {
		if vs, ok := source.MostOuterOperation[*promParser.VectorSelector](s); ok {
			selectors = append(selectors, vs)
		}
	}
	return selectors
}

func metricName(vs *promParser.VectorSelector) string {
	if vs.Name != "" {
		return vs.Name
	}
	for _, m := range vs.LabelMatchers {
		if m.Name == model.MetricNameLabel && m.Type == labels.MatchEqual {
			return m.Value
		}
	}
	return ""
}

func selectorsMismatch(a, b *promParser.VectorSelector) bool {
	for _, am := range a.LabelMatchers {
		if am.Name == model.MetricNameLabel {
			continue
		}
		for _, bm := range b.LabelMatchers {
			if bm.Name == model.MetricNameLabel || bm.Name != am.Name {
				continue
			}
			if matchersMismatch(am, bm) {
				return true
			}
		}
	}
	return false
}

func matchersMismatch(a, b *labels.Matcher) bool {
	// We can only prove two matchers never overlap when one pins the label to a
	// single value that the other rejects.
	if a.Type == labels.MatchEqual {
		return !b.Matches(a.Value)
	}
	if b.Type == labels.MatchEqual {
		return !a.Matches(b.Value)
	}
	return false
}

func (c RuleDuplicateCheck) compareRules(_ context.Context, rule *parser.RecordingRule, entry *discovery.Entry, lines diags.LineRange) (problems []Problem) {
	if !rule.Labels.IsIdentical(entry.Rule.RecordingRule.Labels) {
		return nil
	}

	if rule.Expr.Query().Expr.String() == entry.Rule.RecordingRule.Expr.Query().Expr.String() {
		problems = append(problems, Problem{
			Anchor:   AnchorAfter,
			Lines:    lines,
			Reporter: c.Reporter(),
			Summary:  "duplicated recording rule",
			Details:  "",
			Severity: Bug,
			Diagnostics: []diags.Diagnostic{
				{
					Message:     fmt.Sprintf("Duplicated rule, identical rule found at %s:%d.", entry.Path.SymlinkTarget, entry.Rule.RecordingRule.Record.Pos.Lines().First),
					Pos:         rule.Record.Pos,
					Expr:        nil,
					FirstColumn: 1,
					LastColumn:  len(rule.Record.Value),
					Kind:        diags.Issue,
				},
			},
		})
	}

	return problems
}

func ignoreOtherEntry(entry, other *discovery.Entry, prom *promapi.FailoverGroup) bool {
	if other.State == discovery.Removed {
		return true
	}
	if other.PathError != nil {
		return true
	}
	if other.Rule.Error.Err != nil {
		return true
	}
	if other.Rule.Expr().SyntaxError() != nil {
		return true
	}
	if other.Path.Name == entry.Path.Name && other.Rule.Lines.First == entry.Rule.Lines.First {
		return true
	}
	if !prom.IsEnabledForPath(entry.Path.Name) {
		return true
	}
	if !prom.IsEnabledForPath(other.Path.Name) {
		return true
	}
	return false
}
