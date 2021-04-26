package checks

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/cloudflare/pint/internal/parser"
)

const (
	RejectCheckName = "rule/reject"
)

func NewRejectCheck(l, a bool, k, v *regexp.Regexp, s Severity) Reject {
	return Reject{checkLabels: l, checkAnnotations: a, keyRe: k, valueRe: v, severity: s}
}

type Reject struct {
	checkLabels      bool
	checkAnnotations bool
	keyRe            *regexp.Regexp
	valueRe          *regexp.Regexp
	severity         Severity
}

func (c Reject) String() string {
	r := []string{}
	if c.keyRe != nil {
		r = append(r, fmt.Sprintf("key=~'%s'", c.keyRe))
	}
	if c.valueRe != nil {
		r = append(r, fmt.Sprintf("val=~'%s'", c.valueRe))
	}
	return fmt.Sprintf("%s(%s)", RejectCheckName, strings.Join(r, " "))
}

func (c Reject) Check(rule parser.Rule) (problems []Problem) {
	if c.checkLabels && rule.AlertingRule != nil && rule.AlertingRule.Labels != nil {
		for _, label := range rule.AlertingRule.Labels.Items {
			problems = append(problems, c.reject(label, "label")...)
		}
	}
	if c.checkLabels && rule.RecordingRule != nil && rule.RecordingRule.Labels != nil {
		for _, label := range rule.RecordingRule.Labels.Items {
			problems = append(problems, c.reject(label, "label")...)
		}
	}
	if c.checkAnnotations && rule.AlertingRule != nil && rule.AlertingRule.Annotations != nil {
		for _, ann := range rule.AlertingRule.Annotations.Items {
			problems = append(problems, c.reject(ann, "annotation")...)
		}
	}
	return
}

func (c Reject) reject(label *parser.YamlKeyValue, kind string) (problems []Problem) {
	if c.keyRe != nil && c.keyRe.MatchString(label.Key.Value) {
		problems = append(problems, Problem{
			Fragment: label.Key.Value,
			Lines:    label.Lines(),
			Reporter: RejectCheckName,
			Text:     fmt.Sprintf("%s key %s is not allowed to match %s", kind, label.Key.Value, c.keyRe),
			Severity: c.severity,
		})
	}
	if c.valueRe != nil && c.valueRe.MatchString(label.Value.Value) {
		problems = append(problems, Problem{
			Fragment: label.Value.Value,
			Lines:    label.Lines(),
			Reporter: RejectCheckName,
			Text:     fmt.Sprintf("%s value %s is not allowed to match %s", kind, label.Value.Value, c.valueRe),
			Severity: c.severity,
		})
	}
	return
}
