package checks

import (
	"context"
	"fmt"

	"github.com/prometheus/prometheus/model/labels"

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
		IsOnline: false,
	}
}

func (c RuleDuplicateCheck) String() string {
	return fmt.Sprintf("%s(%s)", RuleDuplicateCheckName, c.prom.Name())
}

func (c RuleDuplicateCheck) Reporter() string {
	return RuleDuplicateCheckName
}

func (c RuleDuplicateCheck) Check(ctx context.Context, path discovery.Path, rule parser.Rule, entries []discovery.Entry) (problems []Problem) {
	expr := rule.Expr()
	if expr.SyntaxError != nil {
		return problems
	}

	for _, entry := range entries {
		if entry.State == discovery.Removed {
			continue
		}
		if entry.PathError != nil {
			continue
		}
		if entry.Rule.Error.Err != nil {
			continue
		}
		if entry.Rule.Expr().SyntaxError != nil {
			continue
		}
		if entry.Rule.RecordingRule == nil {
			continue
		}
		if entry.Path.Name == path.Name && entry.Rule.Lines.First == rule.Lines.First {
			continue
		}
		if !c.prom.IsEnabledForPath(path.Name) {
			continue
		}
		if !c.prom.IsEnabledForPath(entry.Path.Name) {
			continue
		}

		// Look for identical recording rules.
		if entry.Rule.RecordingRule != nil && rule.RecordingRule != nil && entry.Rule.RecordingRule.Record.Value == rule.RecordingRule.Record.Value {
			problems = append(problems, c.compareRules(ctx, rule.RecordingRule, entry, rule.Lines)...)
		}
	}
	return problems
}

func (c RuleDuplicateCheck) compareRules(_ context.Context, rule *parser.RecordingRule, entry discovery.Entry, lines parser.LineRange) (problems []Problem) {
	ruleALabels := buildRuleLabels(rule.Labels)
	ruleBLabels := buildRuleLabels(entry.Rule.RecordingRule.Labels)

	if ruleALabels.Hash() != ruleBLabels.Hash() {
		return nil
	}

	if rule.Expr.Query.Expr.String() == entry.Rule.RecordingRule.Expr.Query.Expr.String() {
		problems = append(problems, Problem{
			Lines:    lines,
			Reporter: c.Reporter(),
			Text:     fmt.Sprintf("Duplicated rule, identical rule found at %s:%d.", entry.Path.SymlinkTarget, entry.Rule.RecordingRule.Record.Lines.First),
			Severity: Bug,
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
