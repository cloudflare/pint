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

func NewLabelCheck(keyRe, tokenRe, valueRe *TemplatedRegexp, values []string, isRequired bool, comment string, severity Severity) LabelCheck {
	return LabelCheck{
		keyRe:      keyRe,
		tokenRe:    tokenRe,
		valueRe:    valueRe,
		values:     values,
		isRequired: isRequired,
		comment:    comment,
		severity:   severity,
	}
}

type LabelCheck struct {
	keyRe      *TemplatedRegexp
	tokenRe    *TemplatedRegexp
	valueRe    *TemplatedRegexp
	comment    string
	values     []string
	severity   Severity
	isRequired bool
}

func (c LabelCheck) Meta() CheckMeta {
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

func (c LabelCheck) String() string {
	if c.valueRe != nil {
		return fmt.Sprintf("%s(%s=~%s:%v)", LabelCheckName, c.keyRe.original, c.valueRe.anchored, c.isRequired)
	}
	return fmt.Sprintf("%s(%s:%v)", LabelCheckName, c.keyRe.original, c.isRequired)
}

func (c LabelCheck) Reporter() string {
	return LabelCheckName
}

func (c LabelCheck) Check(_ context.Context, _ discovery.Path, rule parser.Rule, _ []discovery.Entry) (problems []Problem) {
	if rule.RecordingRule != nil {
		problems = append(problems, c.checkRecordingRule(rule)...)
	}

	if rule.AlertingRule != nil {
		problems = append(problems, c.checkAlertingRule(rule)...)
	}

	return problems
}

func (c LabelCheck) checkRecordingRule(rule parser.Rule) (problems []Problem) {
	if rule.RecordingRule.Labels == nil || len(rule.RecordingRule.Labels.Items) == 0 {
		if c.isRequired {
			problems = append(problems, Problem{
				Lines:    rule.Lines,
				Reporter: c.Reporter(),
				Summary:  fmt.Sprintf("`%s` label is required.", c.keyRe.original),
				Details:  maybeComment(c.comment),
				Severity: c.severity,
			})
		}
		return problems
	}

	val := rule.RecordingRule.Labels.GetValue(c.keyRe.original)
	if val == nil || val.Value == "" {
		if c.isRequired {
			problems = append(problems, Problem{
				Lines:    rule.RecordingRule.Labels.Lines,
				Reporter: c.Reporter(),
				Summary:  fmt.Sprintf("`%s` label is required.", c.keyRe.original),
				Details:  maybeComment(c.comment),
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
	if rule.AlertingRule.Labels == nil || len(rule.AlertingRule.Labels.Items) == 0 {
		if c.isRequired {
			problems = append(problems, Problem{
				Lines:    rule.Lines,
				Reporter: c.Reporter(),
				Summary:  fmt.Sprintf("`%s` label is required.", c.keyRe.original),
				Details:  maybeComment(c.comment),
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

	if len(labels) == 0 && c.isRequired {
		problems = append(problems, Problem{
			Lines:    rule.AlertingRule.Labels.Lines,
			Reporter: c.Reporter(),
			Summary:  fmt.Sprintf("`%s` label is required.", c.keyRe.original),
			Details:  maybeComment(c.comment),
			Severity: c.severity,
		})
		return problems
	}

	for _, lab := range labels {
		if lab.Value.Value == "" && c.isRequired {
			problems = append(problems, Problem{
				Lines:    rule.AlertingRule.Labels.Lines,
				Reporter: c.Reporter(),
				Summary:  fmt.Sprintf("`%s` label is required.", c.keyRe.original),
				Details:  maybeComment(c.comment),
				Severity: c.severity,
			})
			return problems
		}
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
			Summary:  fmt.Sprintf("`%s` label value `%s` must match `%s`.", c.keyRe.original, value, c.valueRe.anchored),
			Details:  maybeComment(c.comment),
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
			if c.comment != "" {
				details.WriteRune('\n')
				details.WriteString("Rule comment: ")
				details.WriteString(c.comment)
			}
			problems = append(problems, Problem{
				Lines:    lines,
				Reporter: c.Reporter(),
				Summary:  fmt.Sprintf("`%s` label value `%s` is not one of valid values.", c.keyRe.original, value),
				Details:  details.String(),
				Severity: c.severity,
			})
		}
	}
	return problems
}
