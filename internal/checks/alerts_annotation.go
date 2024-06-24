package checks

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"

	"golang.org/x/exp/slices"
)

const (
	AnnotationCheckName = "alerts/annotation"
)

func NewAnnotationCheck(keyRe, tokenRe, valueRe *TemplatedRegexp, values []string, isRequired bool, comment string, severity Severity) AnnotationCheck {
	return AnnotationCheck{
		keyRe:      keyRe,
		tokenRe:    tokenRe,
		valueRe:    valueRe,
		values:     values,
		isRequired: isRequired,
		comment:    comment,
		severity:   severity,
	}
}

type AnnotationCheck struct {
	keyRe      *TemplatedRegexp
	tokenRe    *TemplatedRegexp
	valueRe    *TemplatedRegexp
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
		IsOnline: false,
	}
}

func (c AnnotationCheck) String() string {
	if c.valueRe != nil {
		return fmt.Sprintf("%s(%s=~%s:%v)", AnnotationCheckName, c.keyRe.original, c.valueRe.anchored, c.isRequired)
	}
	return fmt.Sprintf("%s(%s:%v)", AnnotationCheckName, c.keyRe.original, c.isRequired)
}

func (c AnnotationCheck) Reporter() string {
	return AnnotationCheckName
}

func (c AnnotationCheck) Check(_ context.Context, _ discovery.Path, rule parser.Rule, _ []discovery.Entry) (problems []Problem) {
	if rule.AlertingRule == nil {
		return nil
	}

	if rule.AlertingRule.Annotations == nil || len(rule.AlertingRule.Annotations.Items) == 0 {
		if c.isRequired {
			problems = append(problems, Problem{
				Lines:    rule.Lines,
				Reporter: c.Reporter(),
				Text:     fmt.Sprintf("`%s` annotation is required.", c.keyRe.original),
				Details:  maybeComment(c.comment),
				Severity: c.severity,
			})
		}
		return problems
	}

	annotations := make([]*parser.YamlKeyValue, 0, len(rule.AlertingRule.Annotations.Items))

	for _, annotation := range rule.AlertingRule.Annotations.Items {
		if c.keyRe.MustExpand(rule).MatchString(annotation.Key.Value) {
			annotations = append(annotations, annotation)
		}
	}

	if len(annotations) == 0 && c.isRequired {
		problems = append(problems, Problem{
			Lines:    rule.AlertingRule.Annotations.Lines,
			Reporter: c.Reporter(),
			Text:     fmt.Sprintf("`%s` annotation is required.", c.keyRe.original),
			Details:  maybeComment(c.comment),
			Severity: c.severity,
		})
		return problems
	}

	for _, ann := range annotations {
		if ann.Value.Value == "" && c.isRequired {
			problems = append(problems, Problem{
				Lines:    rule.AlertingRule.Annotations.Lines,
				Reporter: c.Reporter(),
				Text:     fmt.Sprintf("`%s` annotation is required.", c.keyRe.original),
				Details:  maybeComment(c.comment),
				Severity: c.severity,
			})
			return problems
		}
		if c.tokenRe != nil {
			for _, match := range c.tokenRe.MustExpand(rule).FindAllString(ann.Value.Value, -1) {
				problems = append(problems, c.checkValue(rule, match, ann.Value.Lines)...)
			}
		} else {
			problems = append(problems, c.checkValue(rule, ann.Value.Value, ann.Value.Lines)...)
		}
	}

	return problems
}

func (c AnnotationCheck) checkValue(rule parser.Rule, value string, lines parser.LineRange) (problems []Problem) {
	if c.valueRe != nil && !c.valueRe.MustExpand(rule).MatchString(value) {
		problems = append(problems, Problem{
			Lines:    lines,
			Reporter: c.Reporter(),
			Text:     fmt.Sprintf("`%s` annotation value `%s` must match `%s`.", c.keyRe.original, value, c.valueRe.anchored),
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
				Text:     fmt.Sprintf("`%s` annotation value `%s` is not one of valid values.", c.keyRe.original, value),
				Details:  details.String(),
				Severity: c.severity,
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
