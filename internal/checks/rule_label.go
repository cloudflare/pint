package checks

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
)

const (
	LabelCheckName = "rule/label"
)

func NewLabelCheck(keyRe, tokenRe, valueRe *TemplatedRegexp, values []string, isReguired bool, severity Severity) LabelCheck {
	return LabelCheck{
		keyRe:      keyRe,
		tokenRe:    tokenRe,
		valueRe:    valueRe,
		values:     values,
		isReguired: isReguired,
		severity:   severity,
	}
}

type LabelCheck struct {
	keyRe      *TemplatedRegexp
	tokenRe    *TemplatedRegexp
	valueRe    *TemplatedRegexp
	values     []string
	isReguired bool
	severity   Severity
}

func (c LabelCheck) Meta() CheckMeta {
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

func (c LabelCheck) String() string {
	if c.valueRe != nil {
		return fmt.Sprintf("%s(%s=~%s:%v)", LabelCheckName, c.keyRe.original, c.valueRe.anchored, c.isReguired)
	}
	return fmt.Sprintf("%s(%s:%v)", LabelCheckName, c.keyRe.original, c.isReguired)
}

func (c LabelCheck) Reporter() string {
	return LabelCheckName
}

func (c LabelCheck) Check(_ context.Context, _ string, rule parser.Rule, _ []discovery.Entry) (problems []Problem) {
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
				Lines:    rule.Lines,
				Reporter: c.Reporter(),
				Text:     fmt.Sprintf("`%s` label is required.", c.keyRe.original),
				Severity: c.severity,
			})
		}
		return problems
	}

	val := rule.RecordingRule.Labels.GetValue(c.keyRe.original)
	if val == nil {
		if c.isReguired {
			problems = append(problems, Problem{
				Lines:    rule.RecordingRule.Labels.Lines,
				Reporter: c.Reporter(),
				Text:     fmt.Sprintf("`%s` label is required.", c.keyRe.original),
				Severity: c.severity,
			})
		}
		return problems
	}

	if c.tokenRe != nil {
		for _, match := range c.tokenRe.MustExpand(rule).FindAllString(val.Value, -1) {
			problems = append(problems, c.checkValue(rule, match, val.Lines)...)
		}
		return problems
	}

	problems = append(problems, c.checkValue(rule, val.Value, val.Lines)...)
	return problems
}

func (c LabelCheck) checkAlertingRule(rule parser.Rule) (problems []Problem) {
	if rule.AlertingRule.Labels == nil {
		if c.isReguired {
			problems = append(problems, Problem{
				Lines:    rule.Lines,
				Reporter: c.Reporter(),
				Text:     fmt.Sprintf("`%s` label is required.", c.keyRe.original),
				Severity: c.severity,
			})
		}
		return problems
	}

	labels := make([]*parser.YamlKeyValue, 0, len(rule.AlertingRule.Labels.Items))

	for _, lab := range rule.AlertingRule.Labels.Items {
		if c.keyRe.MustExpand(rule).MatchString(lab.Key.Value) {
			labels = append(labels, lab)
		}
	}

	if len(labels) == 0 && c.isReguired {
		problems = append(problems, Problem{
			Lines:    rule.AlertingRule.Labels.Lines,
			Reporter: c.Reporter(),
			Text:     fmt.Sprintf("`%s` label is required.", c.keyRe.original),
			Severity: c.severity,
		})
		return problems
	}

	for _, lab := range labels {
		if c.tokenRe != nil {
			for _, match := range c.tokenRe.MustExpand(rule).FindAllString(lab.Value.Value, -1) {
				problems = append(problems, c.checkValue(rule, match, lab.Value.Lines)...)
			}
		} else {
			problems = append(problems, c.checkValue(rule, lab.Value.Value, lab.Value.Lines)...)
		}
	}

	return problems
}

func (c LabelCheck) checkValue(rule parser.Rule, value string, lines parser.LineRange) (problems []Problem) {
	if c.valueRe != nil && !c.valueRe.MustExpand(rule).MatchString(value) {
		problems = append(problems, Problem{
			Lines:    lines,
			Reporter: c.Reporter(),
			Text:     fmt.Sprintf("`%s` label value `%s` must match `%s`.", c.keyRe.original, value, c.valueRe.anchored),
			Severity: c.severity,
		})
	}
	if len(c.values) > 0 {
		if !slices.Contains(c.values, value) {
			var details strings.Builder
			details.WriteString("List of allowed values:\n\n")
			for i, allowed := range c.values {
				details.WriteString("- `")
				details.WriteString(allowed)
				details.WriteString("`\n")
				if i >= 5 && len(c.values) > 8 {
					details.WriteString("\nAnd ")
					details.WriteString(strconv.Itoa(len(c.values) - i - 1))
					details.WriteString(" other value(s).")
					break
				}
			}
			problems = append(problems, Problem{
				Lines:    lines,
				Reporter: c.Reporter(),
				Text:     fmt.Sprintf("`%s` label value `%s` is not one of valid values.", c.keyRe.original, value),
				Details:  details.String(),
				Severity: c.severity,
			})
		}
	}
	return problems
}
