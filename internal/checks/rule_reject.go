package checks

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/parser"
)

const (
	RejectCheckName = "rule/reject"
)

func NewRejectCheck(l, a bool, k, v *TemplatedRegexp, s Severity) Reject {
	return Reject{checkLabels: l, checkAnnotations: a, keyRe: k, valueRe: v, severity: s}
}

type Reject struct {
	keyRe            *TemplatedRegexp
	valueRe          *TemplatedRegexp
	severity         Severity
	checkLabels      bool
	checkAnnotations bool
}

func (c Reject) Meta() CheckMeta {
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

func (c Reject) String() string {
	r := []string{}
	if c.keyRe != nil {
		r = append(r, fmt.Sprintf("key=~'%s'", c.keyRe.anchored))
	}
	if c.valueRe != nil {
		r = append(r, fmt.Sprintf("val=~'%s'", c.valueRe.anchored))
	}
	return fmt.Sprintf("%s(%s)", RejectCheckName, strings.Join(r, " "))
}

func (c Reject) Reporter() string {
	return RejectCheckName
}

func (c Reject) Check(_ context.Context, _ discovery.Path, rule parser.Rule, _ []discovery.Entry) (problems []Problem) {
	if c.checkLabels && rule.AlertingRule != nil && rule.AlertingRule.Labels != nil {
		for _, label := range rule.AlertingRule.Labels.Items {
			problems = append(problems, c.reject(rule, label, label.Value.Lines)...)
		}
	}
	if c.checkLabels && rule.RecordingRule != nil && rule.RecordingRule.Labels != nil {
		for _, label := range rule.RecordingRule.Labels.Items {
			problems = append(problems, c.reject(rule, label, label.Value.Lines)...)
		}
	}
	if c.checkAnnotations && rule.AlertingRule != nil && rule.AlertingRule.Annotations != nil {
		for _, ann := range rule.AlertingRule.Annotations.Items {
			problems = append(problems, c.reject(rule, ann, ann.Value.Lines)...)
		}
	}
	return problems
}

func (c Reject) reject(rule parser.Rule, label *parser.YamlKeyValue, lines parser.LineRange) (problems []Problem) {
	if c.keyRe != nil && c.keyRe.MustExpand(rule).MatchString(label.Key.Value) {
		problems = append(problems, Problem{
			Lines:    lines,
			Reporter: c.Reporter(),
			Summary:  "key not allowed",
			Diagnostics: []output.Diagnostic{
				{
					Line:        label.Key.Lines.Last,
					FirstColumn: label.Key.Column,
					LastColumn:  nodeLastColumn(label.Key),
					Message:     fmt.Sprintf("key is not allowed to match `%s`.", c.keyRe.anchored),
				},
			},
			Severity: c.severity,
		})
	}
	if c.valueRe != nil && c.valueRe.MustExpand(rule).MatchString(label.Value.Value) {
		problems = append(problems, Problem{
			Lines: parser.LineRange{
				First: label.Key.Lines.First,
				Last:  label.Value.Lines.Last,
			},
			Reporter: c.Reporter(),
			Summary:  "value not allowed",
			Diagnostics: []output.Diagnostic{
				{
					Line:        label.Value.Lines.Last,
					FirstColumn: label.Value.Column,
					LastColumn:  nodeLastColumn(label.Value),
					Message:     fmt.Sprintf("value is not allowed to match `%s`.", c.valueRe.anchored),
				},
			},
			Severity: c.severity,
		})
	}
	return problems
}
