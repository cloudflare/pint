package checks

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/cloudflare/pint/internal/diags"
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

func (c LabelCheck) Check(_ context.Context, entry discovery.Entry, _ []discovery.Entry) (problems []Problem) {
	if entry.Rule.RecordingRule != nil {
		problems = append(problems, c.checkRecordingRule(entry)...)
	}

	if entry.Rule.AlertingRule != nil {
		problems = append(problems, c.checkAlertingRule(entry)...)
	}

	return problems
}

func (c LabelCheck) checkRecordingRule(entry discovery.Entry) (problems []Problem) {
	entryLabels := entry.Labels()

	if len(entryLabels.Items) == 0 {
		if c.isRequired {
			problems = append(problems, Problem{
				Anchor:   AnchorAfter,
				Lines:    entry.Rule.Lines,
				Reporter: c.Reporter(),
				Summary:  "required label not set",
				Details:  maybeComment(c.comment),
				Severity: c.severity,
				Diagnostics: []diags.Diagnostic{
					WholeRuleDiag(entry.Rule, fmt.Sprintf("`%s` label is required.", c.keyRe.original)),
				},
			})
		}
		return problems
	}

	val := entryLabels.GetValue(c.keyRe.original)
	if val == nil || val.Value == "" {
		if c.isRequired {
			problems = append(problems, Problem{
				Anchor:   AnchorAfter,
				Lines:    entry.Rule.RecordingRule.Labels.Lines(),
				Reporter: c.Reporter(),
				Summary:  "required label not set",
				Details:  maybeComment(c.comment),
				Severity: c.severity,
				Diagnostics: []diags.Diagnostic{
					{
						Message:     fmt.Sprintf("`%s` label is required.", c.keyRe.original),
						Pos:         entry.Rule.RecordingRule.Labels.Key.Pos,
						FirstColumn: 1,
						LastColumn:  len(entry.Rule.RecordingRule.Labels.Key.Value),
					},
				},
			})
		}
		return problems
	}

	if c.tokenRe != nil {
		for _, match := range c.tokenRe.MustExpand(entry.Rule).FindAllString(val.Value, -1) {
			problems = append(problems, c.checkValue(entry.Rule, match, val)...)
		}
		return problems
	}

	problems = append(problems, c.checkValue(entry.Rule, val.Value, val)...)
	return problems
}

func (c LabelCheck) checkAlertingRule(entry discovery.Entry) (problems []Problem) {
	entryLabels := entry.Labels()

	if len(entryLabels.Items) == 0 {
		if c.isRequired {
			problems = append(problems, Problem{
				Anchor:   AnchorAfter,
				Lines:    entry.Rule.Lines,
				Reporter: c.Reporter(),
				Summary:  "required label not set",
				Details:  maybeComment(c.comment),
				Severity: c.severity,
				Diagnostics: []diags.Diagnostic{
					WholeRuleDiag(entry.Rule, fmt.Sprintf("`%s` label is required.", c.keyRe.original)),
				},
			})
		}
		return problems
	}

	labels := make([]*parser.YamlKeyValue, 0, len(entryLabels.Items))

	for _, lab := range entryLabels.Items {
		if c.keyRe.MustExpand(entry.Rule).MatchString(lab.Key.Value) {
			labels = append(labels, lab)
		}
	}

	if len(labels) == 0 && c.isRequired {
		problems = append(problems, Problem{
			Anchor:   AnchorAfter,
			Lines:    entry.Rule.Lines,
			Reporter: c.Reporter(),
			Summary:  "required label not set",
			Details:  maybeComment(c.comment),
			Severity: c.severity,
			Diagnostics: []diags.Diagnostic{
				WholeRuleDiag(entry.Rule, fmt.Sprintf("`%s` label is required.", c.keyRe.original)),
			},
		})
		return problems
	}

	for _, lab := range labels {
		if lab.Value.Value == "" && c.isRequired {
			problems = append(problems, Problem{
				Anchor:   AnchorAfter,
				Lines:    entry.Rule.Lines,
				Reporter: c.Reporter(),
				Summary:  "required label not set",
				Details:  maybeComment(c.comment),
				Severity: c.severity,
				Diagnostics: []diags.Diagnostic{
					WholeRuleDiag(entry.Rule, fmt.Sprintf("`%s` label is required.", c.keyRe.original)),
				},
			})
			return problems
		}
		if c.tokenRe != nil {
			for _, match := range c.tokenRe.MustExpand(entry.Rule).FindAllString(lab.Value.Value, -1) {
				problems = append(problems, c.checkValue(entry.Rule, match, lab.Value)...)
			}
		} else {
			problems = append(problems, c.checkValue(entry.Rule, lab.Value.Value, lab.Value)...)
		}
	}

	return problems
}

func (c LabelCheck) checkValue(rule parser.Rule, value string, lab *parser.YamlNode) (problems []Problem) {
	if c.valueRe != nil && !c.valueRe.MustExpand(rule).MatchString(value) {
		problems = append(problems, Problem{
			Anchor:   AnchorAfter,
			Lines:    lab.Pos.Lines(),
			Reporter: c.Reporter(),
			Summary:  "invalid label value",
			Details:  maybeComment(c.comment),
			Severity: c.severity,
			Diagnostics: []diags.Diagnostic{
				{
					Message:     fmt.Sprintf("`%s` label value must match `%s`.", c.keyRe.original, c.valueRe.anchored),
					Pos:         lab.Pos,
					FirstColumn: 1,
					LastColumn:  len(lab.Value),
				},
			},
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
				Anchor:   AnchorAfter,
				Lines:    lab.Pos.Lines(),
				Reporter: c.Reporter(),
				Summary:  "invalid label value",
				Details:  details.String(),
				Severity: c.severity,
				Diagnostics: []diags.Diagnostic{
					{
						Message:     fmt.Sprintf("`%s` label value is not one of valid values.", c.keyRe.original),
						Pos:         lab.Pos,
						FirstColumn: 1,
						LastColumn:  len(lab.Value),
					},
				},
			})
		}
	}
	return problems
}
