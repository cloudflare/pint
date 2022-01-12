package checks

import (
	"context"
	"fmt"
	"regexp"

	"github.com/cloudflare/pint/internal/parser"
)

const (
	LabelCheckName = "rule/label"
)

func NewLabelCheck(key string, valueRe *regexp.Regexp, isReguired bool, severity Severity) LabelCheck {
	return LabelCheck{key: key, valueRe: valueRe, isReguired: isReguired, severity: severity}
}

type LabelCheck struct {
	key        string
	valueRe    *regexp.Regexp
	isReguired bool
	severity   Severity
}

func (c LabelCheck) String() string {
	return fmt.Sprintf("%s(%s:%v)", LabelCheckName, c.key, c.isReguired)
}

func (c LabelCheck) Reporter() string {
	return LabelCheckName
}

func (c LabelCheck) Check(ctx context.Context, rule parser.Rule) (problems []Problem) {
	if rule.RecordingRule != nil {
		problems = append(problems, c.checkRecordingRule(rule.RecordingRule)...)
	}

	if rule.AlertingRule != nil {
		problems = append(problems, c.checkAlertingRule(rule.AlertingRule)...)
	}

	return
}

func (c LabelCheck) checkRecordingRule(rule *parser.RecordingRule) (problems []Problem) {
	if rule.Labels == nil {
		if c.isReguired {
			problems = append(problems, Problem{
				Fragment: fmt.Sprintf("%s: %s", rule.Record.Key.Value, rule.Record.Value.Value),
				Lines:    rule.Lines(),
				Reporter: c.Reporter(),
				Text:     fmt.Sprintf("%s label is required", c.key),
				Severity: c.severity,
			})
		}
		return
	}

	val := rule.Labels.GetValue(c.key)
	if val == nil {
		if c.isReguired {
			problems = append(problems, Problem{
				Fragment: fmt.Sprintf("%s:", rule.Labels.Key.Value),
				Lines:    rule.Labels.Lines(),
				Reporter: c.Reporter(),
				Text:     fmt.Sprintf("%s label is required", c.key),
				Severity: c.severity,
			})
		}
		return
	}

	problems = append(problems, c.checkValue(val)...)

	return
}

func (c LabelCheck) checkAlertingRule(rule *parser.AlertingRule) (problems []Problem) {
	if rule.Labels == nil {
		if c.isReguired {
			problems = append(problems, Problem{
				Fragment: fmt.Sprintf("%s: %s", rule.Alert.Key.Value, rule.Alert.Value.Value),
				Lines:    rule.Lines(),
				Reporter: c.Reporter(),
				Text:     fmt.Sprintf("%s label is required", c.key),
				Severity: c.severity,
			})
		}
		return
	}

	val := rule.Labels.GetValue(c.key)
	if val == nil {
		if c.isReguired {
			problems = append(problems, Problem{
				Fragment: fmt.Sprintf("%s:", rule.Labels.Key.Value),
				Lines:    rule.Labels.Lines(),
				Reporter: c.Reporter(),
				Text:     fmt.Sprintf("%s label is required", c.key),
				Severity: c.severity,
			})
		}
		return
	}

	problems = append(problems, c.checkValue(val)...)

	return
}

func (c LabelCheck) checkValue(val *parser.YamlNode) (problems []Problem) {
	if c.valueRe != nil && !c.valueRe.MatchString(val.Value) {
		problems = append(problems, Problem{
			Fragment: fmt.Sprintf("%s: %s", c.key, val.Value),
			Lines:    val.Position.Lines,
			Reporter: c.Reporter(),
			Text:     fmt.Sprintf("%s label value must match regex: %s", c.key, c.valueRe.String()),
			Severity: c.severity,
		})
	}
	return
}
