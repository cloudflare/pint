package checks

import (
	"context"
	"fmt"

	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
)

const (
	AnnotationCheckName = "alerts/annotation"
)

func NewAnnotationCheck(keyRe, valueRe *TemplatedRegexp, isReguired bool, severity Severity) AnnotationCheck {
	return AnnotationCheck{keyRe: keyRe, valueRe: valueRe, isReguired: isReguired, severity: severity}
}

type AnnotationCheck struct {
	keyRe      *TemplatedRegexp
	valueRe    *TemplatedRegexp
	isReguired bool
	severity   Severity
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
		return fmt.Sprintf("%s(%s=~%s:%v)", AnnotationCheckName, c.keyRe.original, c.valueRe.anchored, c.isReguired)
	}
	return fmt.Sprintf("%s(%s:%v)", AnnotationCheckName, c.keyRe.original, c.isReguired)
}

func (c AnnotationCheck) Reporter() string {
	return AnnotationCheckName
}

func (c AnnotationCheck) Check(_ context.Context, _ string, rule parser.Rule, _ []discovery.Entry) (problems []Problem) {
	if rule.AlertingRule == nil {
		return nil
	}

	if rule.AlertingRule.Annotations == nil {
		if c.isReguired {
			problems = append(problems, Problem{
				Fragment: fmt.Sprintf("%s: %s", rule.AlertingRule.Alert.Key.Value, rule.AlertingRule.Alert.Value.Value),
				Lines:    rule.Lines(),
				Reporter: c.Reporter(),
				Text:     fmt.Sprintf("`%s` annotation is required.", c.keyRe.original),
				Severity: c.severity,
			})
		}
		return problems
	}

	var foundAnnotation bool

	for _, annotation := range rule.AlertingRule.Annotations.Items {
		if c.keyRe.MustExpand(rule).MatchString(annotation.Key.Value) {
			foundAnnotation = true
			if c.valueRe != nil && !c.valueRe.MustExpand(rule).MatchString(annotation.Value.Value) {
				problems = append(problems, Problem{
					Fragment: fmt.Sprintf("%s: %s", annotation.Key.Value, annotation.Value.Value),
					Lines:    annotation.Value.Position.Lines,
					Reporter: c.Reporter(),
					Text:     fmt.Sprintf("`%s` annotation value must match `%s`.", c.keyRe.original, c.valueRe.anchored),
					Severity: c.severity,
				})
				return problems
			}
		}
	}

	if !foundAnnotation && c.isReguired {
		problems = append(problems, Problem{
			Fragment: fmt.Sprintf("%s:", rule.AlertingRule.Annotations.Key.Value),
			Lines:    rule.AlertingRule.Annotations.Lines(),
			Reporter: c.Reporter(),
			Text:     fmt.Sprintf("`%s` annotation is required.", c.keyRe.original),
			Severity: c.severity,
		})
		return problems
	}

	return nil
}
