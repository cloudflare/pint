package checks

import (
	"fmt"
	"regexp"

	"github.com/cloudflare/pint/internal/parser"
)

const (
	AnnotationCheckName = "alerts/annotation"
)

func NewAnnotationCheck(key string, valueRe *regexp.Regexp, isReguired bool, severity Severity) AnnotationCheck {
	return AnnotationCheck{key: key, valueRe: valueRe, isReguired: isReguired, severity: severity}
}

type AnnotationCheck struct {
	key        string
	valueRe    *regexp.Regexp
	isReguired bool
	severity   Severity
}

func (c AnnotationCheck) String() string {
	if c.valueRe != nil {
		return fmt.Sprintf("%s(%s=~%s:%v)", AnnotationCheckName, c.key, c.valueRe, c.isReguired)
	}
	return fmt.Sprintf("%s(%s:%v)", AnnotationCheckName, c.key, c.isReguired)
}

func (c AnnotationCheck) Check(rule parser.Rule) (problems []Problem) {
	if rule.AlertingRule == nil {
		return nil
	}

	if rule.AlertingRule.Annotations == nil {
		if c.isReguired {
			problems = append(problems, Problem{
				Fragment: fmt.Sprintf("%s: %s", rule.AlertingRule.Alert.Key.Value, rule.AlertingRule.Alert.Value.Value),
				Lines:    rule.Lines(),
				Reporter: AnnotationCheckName,
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
				Reporter: AnnotationCheckName,
				Text:     fmt.Sprintf("%s annotation is required", c.key),
				Severity: c.severity,
			})
		}
		return
	}

	if c.valueRe != nil && !c.valueRe.MatchString(val.Value) {
		problems = append(problems, Problem{
			Fragment: fmt.Sprintf("%s: %s", c.key, val.Value),
			Lines:    val.Position.Lines,
			Reporter: AnnotationCheckName,
			Text:     fmt.Sprintf("%s annotation value must match regex: %s", c.key, c.valueRe.String()),
			Severity: c.severity,
		})
		return
	}

	return nil
}
