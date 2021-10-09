package checks

import (
	"fmt"
	"strings"

	"github.com/cloudflare/pint/internal/parser"
)

const (
	ValueCheckName = "alerts/value"
)

func NewValueCheck(severity Severity) ValueCheck {
	return ValueCheck{severity: severity}
}

type ValueCheck struct {
	severity Severity
}

func (c ValueCheck) String() string {
	return ValueCheckName
}

func (c ValueCheck) Check(rule parser.Rule) (problems []Problem) {
	if rule.AlertingRule == nil {
		return
	}

	if rule.AlertingRule.Labels != nil {
		for _, label := range rule.AlertingRule.Labels.Items {
			if token, ok := hasValue(label.Key.Value); ok {
				problems = append(problems, Problem{
					Fragment: label.Key.Value,
					Lines:    label.Lines(),
					Reporter: ValueCheckName,
					Text:     fmt.Sprintf("using %s in labels will generate a new alert on every value change, move it to annotations", token),
					Severity: c.severity,
				})
			}
			if token, ok := hasValue(label.Value.Value); ok {
				problems = append(problems, Problem{
					Fragment: label.Value.Value,
					Lines:    label.Lines(),
					Reporter: ValueCheckName,
					Text:     fmt.Sprintf("using %s in labels will generate a new alert on every value change, move it to annotations", token),
					Severity: c.severity,
				})
			}
		}
	}

	return
}

func hasValue(s string) (string, bool) {
	trimmed := strings.Join(strings.Fields(s), "")
	if strings.Contains(trimmed, "{{$value}}") {
		return "$value", true
	}
	if strings.Contains(trimmed, "{{$value|") {
		return "$value", true
	}
	if strings.Contains(trimmed, "{{.Value}}") {
		return ".Value", true
	}
	if strings.Contains(trimmed, "{{.Value|") {
		return ".Value", true
	}
	return "", false
}
