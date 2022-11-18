package checks

import (
	"context"
	"fmt"

	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
)

const (
	LabelCheckName = "rule/label"
)

func NewLabelCheck(key string, valueRe *TemplatedRegexp, isReguired bool, severity Severity) LabelCheck {
	return LabelCheck{key: key, valueRe: valueRe, isReguired: isReguired, severity: severity}
}

type LabelCheck struct {
	key        string
	valueRe    *TemplatedRegexp
	isReguired bool
	severity   Severity
}

func (c LabelCheck) Meta() CheckMeta {
	return CheckMeta{IsOnline: false}
}

func (c LabelCheck) String() string {
	return fmt.Sprintf("%s(%s:%v)", LabelCheckName, c.key, c.isReguired)
}

func (c LabelCheck) Reporter() string {
	return LabelCheckName
}

func (c LabelCheck) Check(ctx context.Context, path string, rule parser.Rule, entries []discovery.Entry) (problems []Problem) {
	if rule.RecordingRule != nil {
		problems = append(problems, c.checkRecordingRule(rule)...)
	}

	if rule.AlertingRule != nil {
		problems = append(problems, c.checkAlertingRule(rule)...)
	}

	return problems
}

func (c LabelCheck) checkRecordingRule(rule parser.Rule) (problems []Problem) {
	if rule.RecordingRule.Labels == nil {
		if c.isReguired {
			problems = append(problems, Problem{
				Fragment: fmt.Sprintf("%s: %s", rule.RecordingRule.Record.Key.Value, rule.RecordingRule.Record.Value.Value),
				Lines:    rule.Lines(),
				Reporter: c.Reporter(),
				Text:     fmt.Sprintf("%s label is required", c.key),
				Severity: c.severity,
			})
		}
		return problems
	}

	val := rule.RecordingRule.Labels.GetValue(c.key)
	if val == nil {
		if c.isReguired {
			problems = append(problems, Problem{
				Fragment: fmt.Sprintf("%s:", rule.RecordingRule.Labels.Key.Value),
				Lines:    rule.RecordingRule.Labels.Lines(),
				Reporter: c.Reporter(),
				Text:     fmt.Sprintf("%s label is required", c.key),
				Severity: c.severity,
			})
		}
		return problems
	}

	problems = append(problems, c.checkValue(rule, val)...)

	return problems
}

func (c LabelCheck) checkAlertingRule(rule parser.Rule) (problems []Problem) {
	if rule.AlertingRule.Labels == nil {
		if c.isReguired {
			problems = append(problems, Problem{
				Fragment: fmt.Sprintf("%s: %s", rule.AlertingRule.Alert.Key.Value, rule.AlertingRule.Alert.Value.Value),
				Lines:    rule.Lines(),
				Reporter: c.Reporter(),
				Text:     fmt.Sprintf("%s label is required", c.key),
				Severity: c.severity,
			})
		}
		return problems
	}

	val := rule.AlertingRule.Labels.GetValue(c.key)
	if val == nil {
		if c.isReguired {
			problems = append(problems, Problem{
				Fragment: fmt.Sprintf("%s:", rule.AlertingRule.Labels.Key.Value),
				Lines:    rule.AlertingRule.Labels.Lines(),
				Reporter: c.Reporter(),
				Text:     fmt.Sprintf("%s label is required", c.key),
				Severity: c.severity,
			})
		}
		return problems
	}

	problems = append(problems, c.checkValue(rule, val)...)

	return problems
}

func (c LabelCheck) checkValue(rule parser.Rule, val *parser.YamlNode) (problems []Problem) {
	if c.valueRe != nil && !c.valueRe.MustExpand(rule).MatchString(val.Value) {
		problems = append(problems, Problem{
			Fragment: fmt.Sprintf("%s: %s", c.key, val.Value),
			Lines:    val.Position.Lines,
			Reporter: c.Reporter(),
			Text:     fmt.Sprintf("%s label value must match %q", c.key, c.valueRe.anchored),
			Severity: c.severity,
		})
	}
	return problems
}
