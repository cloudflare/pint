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
	AnnotationCheckName = "alerts/annotation"
)

func NewAnnotationCheck(keyRe, tokenRe, valueRe *TemplatedRegexp, values []string, isRequired bool, comment string, severity Severity) AnnotationCheck {
	var instance string
	if valueRe != nil {
		instance = fmt.Sprintf("%s(%s=~%s:%v)", AnnotationCheckName, keyRe.original, valueRe.anchored, isRequired)
	} else {
		instance = fmt.Sprintf("%s(%s:%v)", AnnotationCheckName, keyRe.original, isRequired)
	}
	return AnnotationCheck{
		keyRe:      keyRe,
		tokenRe:    tokenRe,
		valueRe:    valueRe,
		values:     values,
		isRequired: isRequired,
		comment:    comment,
		severity:   severity,
		instance:   instance,
	}
}

type AnnotationCheck struct {
	keyRe      *TemplatedRegexp
	tokenRe    *TemplatedRegexp
	valueRe    *TemplatedRegexp
	instance   string
	comment    string
	values     []string
	severity   Severity
	isRequired bool
}

func (c AnnotationCheck) Meta() CheckMeta {
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

func (c AnnotationCheck) String() string {
	return c.instance
}

func (c AnnotationCheck) Reporter() string {
	return AnnotationCheckName
}

func (c AnnotationCheck) Check(_ context.Context, entry *discovery.Entry, _ []*discovery.Entry) (problems []Problem) {
	if entry.Rule.AlertingRule == nil {
		return nil
	}

	if entry.Rule.AlertingRule.Annotations == nil || len(entry.Rule.AlertingRule.Annotations.Items) == 0 {
		if c.isRequired {
			problems = append(problems, Problem{
				Anchor:   AnchorAfter,
				Lines:    entry.Rule.Lines,
				Reporter: c.Reporter(),
				Summary:  "required annotation not set",
				Details:  maybeComment(c.comment),
				Severity: c.severity,
				Diagnostics: []diags.Diagnostic{
					WholeRuleDiag(entry.Rule, fmt.Sprintf("`%s` annotation is required.", c.keyRe.original)),
				},
			})
		}
		return problems
	}

	annotations := make([]*parser.YamlKeyValue, 0, len(entry.Rule.AlertingRule.Annotations.Items))

	for _, annotation := range entry.Rule.AlertingRule.Annotations.Items {
		if c.keyRe.MustExpand(entry.Rule).MatchString(annotation.Key.Value) {
			annotations = append(annotations, annotation)
		}
	}

	if len(annotations) == 0 && c.isRequired {
		problems = append(problems, Problem{
			Anchor:   AnchorAfter,
			Lines:    entry.Rule.Lines,
			Reporter: c.Reporter(),
			Summary:  "required annotation not set",
			Details:  maybeComment(c.comment),
			Severity: c.severity,
			Diagnostics: []diags.Diagnostic{
				WholeRuleDiag(entry.Rule, fmt.Sprintf("`%s` annotation is required.", c.keyRe.original)),
			},
		})
		return problems
	}

	for _, ann := range annotations {
		if ann.Value.Value == "" && c.isRequired {
			problems = append(problems, Problem{
				Anchor:   AnchorAfter,
				Lines:    entry.Rule.Lines,
				Reporter: c.Reporter(),
				Summary:  "required annotation not set",
				Details:  maybeComment(c.comment),
				Severity: c.severity,
				Diagnostics: []diags.Diagnostic{
					WholeRuleDiag(entry.Rule, fmt.Sprintf("`%s` annotation is required.", c.keyRe.original)),
				},
			})
			return problems
		}
		if c.tokenRe != nil {
			for _, match := range c.tokenRe.MustExpand(entry.Rule).FindAllString(ann.Value.Value, -1) {
				problems = append(problems, c.checkValue(entry.Rule, match, ann.Value)...)
			}
		} else {
			problems = append(problems, c.checkValue(entry.Rule, ann.Value.Value, ann.Value)...)
		}
	}

	return problems
}

func (c AnnotationCheck) checkValue(rule parser.Rule, value string, ann *parser.YamlNode) (problems []Problem) {
	if c.valueRe != nil && !c.valueRe.MustExpand(rule).MatchString(value) {
		problems = append(problems, Problem{
			Anchor:   AnchorAfter,
			Lines:    ann.Pos.Lines(),
			Reporter: c.Reporter(),
			Summary:  "invalid annotation value",
			Details:  maybeComment(c.comment),
			Severity: c.severity,
			Diagnostics: []diags.Diagnostic{
				{
					Message:     fmt.Sprintf("`%s` annotation value must match `%s`.", c.keyRe.original, c.valueRe.anchored),
					Pos:         ann.Pos,
					FirstColumn: 1,
					LastColumn:  len(ann.Value),
					Kind:        diags.Issue,
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
				Lines:    ann.Pos.Lines(),
				Reporter: c.Reporter(),
				Summary:  "invalid annotation value",
				Details:  details.String(),
				Severity: c.severity,
				Diagnostics: []diags.Diagnostic{
					{
						Message:     fmt.Sprintf("`%s` annotation value is not one of valid values.", c.keyRe.original),
						Pos:         ann.Pos,
						FirstColumn: 1,
						LastColumn:  len(ann.Value),
						Kind:        diags.Issue,
					},
				},
			})
		}
	}
	return problems
}

func maybeComment(c string) string {
	if c != "" {
		return "Rule comment: " + c
	}
	return ""
}
