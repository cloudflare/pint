package checks

import (
	"context"
	"fmt"

	"github.com/prometheus/prometheus/model/labels"

	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
)

const (
	RuleDuplicateCheckName = "rule/duplicate"
)

func NewRuleDuplicateCheck(prom *promapi.FailoverGroup) RuleDuplicateCheck {
	return RuleDuplicateCheck{prom: prom}
}

type RuleDuplicateCheck struct {
	prom *promapi.FailoverGroup
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
	return fmt.Sprintf("%s(%s)", RuleDuplicateCheckName, c.prom.Name())
}

func (c RuleDuplicateCheck) Reporter() string {
	return RuleDuplicateCheckName
}

func (c RuleDuplicateCheck) Check(ctx context.Context, entry discovery.Entry, entries []discovery.Entry) (problems []Problem) {
	expr := entry.Rule.Expr()
	if expr.SyntaxError != nil {
		return problems
	}

	for _, other := range entries {
		if other.State == discovery.Removed {
			continue
		}
		if other.PathError != nil {
			continue
		}
		if other.Rule.Error.Err != nil {
			continue
		}
		if other.Rule.Expr().SyntaxError != nil {
			continue
		}
		if other.Rule.RecordingRule == nil {
			continue
		}
		if other.Path.Name == entry.Path.Name && other.Rule.Lines.First == entry.Rule.Lines.First {
			continue
		}
		if !c.prom.IsEnabledForPath(entry.Path.Name) {
			continue
		}
		if !c.prom.IsEnabledForPath(other.Path.Name) {
			continue
		}

		// Look for identical recording rules.
		if other.Rule.RecordingRule != nil && entry.Rule.RecordingRule != nil && other.Rule.RecordingRule.Record.Value == entry.Rule.RecordingRule.Record.Value {
			problems = append(problems, c.compareRules(ctx, entry.Rule.RecordingRule, other, entry.Rule.Lines)...)
		}
	}
	return problems
}

func (c RuleDuplicateCheck) compareRules(_ context.Context, rule *parser.RecordingRule, entry discovery.Entry, lines diags.LineRange) (problems []Problem) {
	ruleALabels := buildRuleLabels(rule.Labels)
	ruleBLabels := buildRuleLabels(entry.Rule.RecordingRule.Labels)

	if ruleALabels.Hash() != ruleBLabels.Hash() {
		return nil
	}

	if rule.Expr.Query.Expr.String() == entry.Rule.RecordingRule.Expr.Query.Expr.String() {
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
					FirstColumn: 1,
					LastColumn:  len(rule.Record.Value),
				},
			},
		})
	}

	return problems
}

func buildRuleLabels(l *parser.YamlMap) labels.Labels {
	if l == nil || len(l.Items) == 0 {
		return labels.EmptyLabels()
	}

	pairs := make([]string, 0, len(l.Items))
	for _, label := range l.Items {
		pairs = append(pairs, label.Key.Value, label.Value.Value)
	}
	return labels.FromStrings(pairs...)
}
