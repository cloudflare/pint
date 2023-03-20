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

func NewAnnotationCheck(key string, valueRe *TemplatedRegexp, isReguired bool, severity Severity) AnnotationCheck {
	return AnnotationCheck{key: key, valueRe: valueRe, isReguired: isReguired, severity: severity}
}

type AnnotationCheck struct {
	key        string
	valueRe    *TemplatedRegexp
	isReguired bool
	severity   Severity
}

func (c AnnotationCheck) Meta() CheckMeta {
	return CheckMeta{IsOnline: false}
}

func (c AnnotationCheck) String() string {
	if c.valueRe != nil {
		return fmt.Sprintf("%s(%s=~%s:%v)", AnnotationCheckName, c.key, c.valueRe.anchored, c.isReguired)
	}
	return fmt.Sprintf("%s(%s:%v)", AnnotationCheckName, c.key, c.isReguired)
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
				Text:     fmt.Sprintf("%s annotation is required", c.key),
				Severity: c.severity,
			})
		}
		return
	}

	val := rule.AlertingRule.Annotations.GetValue(c.key)
	if val == nil {
		if c.isReguired {
			problems = append(problems, Problem{
				Fragment: fmt.Sprintf("%s:", rule.AlertingRule.Annotations.Key.Value),
				Lines:    rule.AlertingRule.Annotations.Lines(),
				Reporter: c.Reporter(),
				Text:     fmt.Sprintf("%s annotation is required", c.key),
				Severity: c.severity,
			})
		}
		return
	}

	if c.valueRe != nil && !c.valueRe.MustExpand(rule).MatchString(val.Value) {
		problems = append(problems, Problem{
			Fragment: fmt.Sprintf("%s: %s", c.key, val.Value),
			Lines:    val.Position.Lines,
			Reporter: c.Reporter(),
			Text:     fmt.Sprintf("%s annotation value must match %q", c.key, c.valueRe.anchored),
			Severity: c.severity,
		})
		return
	}

	return nil
}
