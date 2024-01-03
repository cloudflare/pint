package checks

import (
	"context"
	"fmt"

	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
)

const (
	RuleLabelValueTypeName = "rule/label/value_type"
)

func NewRuleLabelValueTypeCheck() RuleLabelValueTypeCheck {
	return RuleLabelValueTypeCheck{}
}

type RuleLabelValueTypeCheck struct{}

func (c RuleLabelValueTypeCheck) Meta() CheckMeta {
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

func (c RuleLabelValueTypeCheck) String() string {
	return RuleLabelValueTypeName
}

func (c RuleLabelValueTypeCheck) Reporter() string {
	return RuleLabelValueTypeName
}

func (c RuleLabelValueTypeCheck) Check(ctx context.Context, _ string, rule parser.Rule, _ []discovery.Entry) (problems []Problem) {
	if rule.RecordingRule != nil {
		problems = append(problems, c.checkRecordingRule(rule)...)
	}

	if rule.AlertingRule != nil {
		problems = append(problems, c.checkAlertingRule(rule)...)
	}

	return problems
}

func (c RuleLabelValueTypeCheck) checkRecordingRule(rule parser.Rule) (problems []Problem) {
	if rule.RecordingRule.Labels == nil {
		return problems
	}
	problems = append(problems, c.checkRuleLabelsValueType(rule.Name(), "recording", rule.RecordingRule.Labels, problems)...)
	return problems
}

func (c RuleLabelValueTypeCheck) checkAlertingRule(rule parser.Rule) (problems []Problem) {
	if rule.AlertingRule.Labels == nil {
		return problems
	}
	problems = append(problems, c.checkRuleLabelsValueType(rule.Name(), "alerting", rule.AlertingRule.Labels, problems)...)
	return problems
}

func (c RuleLabelValueTypeCheck) checkRuleLabelsValueType(ruleName, ruleType string, labels *parser.YamlMap, problems []Problem) []Problem {
	for _, label := range labels.Items {
		if label.Value.Tag != "!!str" {
			problems = append(problems, Problem{
				Lines:    label.Value.Lines,
				Reporter: c.Reporter(),
				Text:     fmt.Sprintf("%s rule `%s` has label `%s` with non-string value, got `%s`.", ruleType, ruleName, label.Key.Value, label.Value.Tag),
				Severity: Bug,
			})
		}

	}
	return problems
}
