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

func (c RuleDuplicateCheck) Check(ctx context.Context, path string, rule parser.Rule, entries []discovery.Entry) (problems []Problem) {
	if rule.RecordingRule == nil || rule.RecordingRule.Expr.SyntaxError != nil {
		return nil
	}

	for _, entry := range entries {
		entry := entry
		if entry.Rule.Error.Err != nil {
			continue
		}
		if entry.Rule.RecordingRule == nil {
			continue
		}
		if entry.SourcePath == path && entry.Rule.LineRange()[0] == rule.LineRange()[0] {
			continue
		}
		if !c.prom.IsEnabledForPath(entry.SourcePath) {
			continue
		}
		if entry.Rule.RecordingRule.Record.Value.Value != rule.RecordingRule.Record.Value.Value {
			continue
		}
		problems = append(problems, c.compareRules(ctx, rule.RecordingRule, entry)...)
	}
	return problems
}

func (c RuleDuplicateCheck) compareRules(_ context.Context, rule *parser.RecordingRule, entry discovery.Entry) (problems []Problem) {
	ruleALabels := buildRuleLabels(rule)
	ruleBLabels := buildRuleLabels(entry.Rule.RecordingRule)

	if ruleALabels.Hash() != ruleBLabels.Hash() {
		return nil
	}

	if rule.Expr.Value.Value == entry.Rule.RecordingRule.Expr.Value.Value {
		problems = append(problems, Problem{
			Lines:    rule.Lines(),
			Reporter: c.Reporter(),
			Text:     fmt.Sprintf("Duplicated rule, identical rule found at %s:%d.", entry.ReportedPath, entry.Rule.RecordingRule.Record.Key.Position.FirstLine()),
			Severity: Bug,
		})
	}

	return problems
}

func buildRuleLabels(rule *parser.RecordingRule) labels.Labels {
	if rule.Labels == nil || len(rule.Labels.Items) == 0 {
		return labels.EmptyLabels()
	}

	ls := make(labels.Labels, 0, len(rule.Labels.Items))
	for _, l := range rule.Labels.Items {
		ls = append(ls, labels.FromStrings(l.Key.Value, l.Value.Value)...)
	}
	return ls
}
