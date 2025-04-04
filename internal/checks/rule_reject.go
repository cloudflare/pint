package checks

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
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

func (c Reject) Check(_ context.Context, entry discovery.Entry, _ []discovery.Entry) (problems []Problem) {
	if c.checkLabels && entry.Rule.AlertingRule != nil && entry.Rule.AlertingRule.Labels != nil {
		for _, label := range entry.Rule.AlertingRule.Labels.Items {
			problems = append(problems, c.reject(entry.Rule, label)...)
		}
	}
	if c.checkLabels && entry.Rule.RecordingRule != nil && entry.Rule.RecordingRule.Labels != nil {
		for _, label := range entry.Rule.RecordingRule.Labels.Items {
			problems = append(problems, c.reject(entry.Rule, label)...)
		}
	}
	if c.checkAnnotations && entry.Rule.AlertingRule != nil && entry.Rule.AlertingRule.Annotations != nil {
		for _, ann := range entry.Rule.AlertingRule.Annotations.Items {
			problems = append(problems, c.reject(entry.Rule, ann)...)
		}
	}
	return problems
}

func (c Reject) reject(rule parser.Rule, label *parser.YamlKeyValue) (problems []Problem) {
	if c.keyRe != nil && c.keyRe.MustExpand(rule).MatchString(label.Key.Value) {
		problems = append(problems, Problem{
			Anchor:   AnchorAfter,
			Lines:    label.Value.Pos.Lines(),
			Reporter: c.Reporter(),
			Summary:  "key not allowed",
			Details:  "",
			Diagnostics: []diags.Diagnostic{
				{
					Message:     fmt.Sprintf("key is not allowed to match `%s`.", c.keyRe.anchored),
					Pos:         label.Key.Pos,
					FirstColumn: 1,
					LastColumn:  len(label.Key.Value) - 1,
				},
			},
			Severity: c.severity,
		})
	}
	if c.valueRe != nil && c.valueRe.MustExpand(rule).MatchString(label.Value.Value) {
		problems = append(problems, Problem{
			Anchor: AnchorAfter,
			Lines: diags.LineRange{
				First: label.Key.Pos.Lines().First,
				Last:  label.Value.Pos.Lines().Last,
			},
			Reporter: c.Reporter(),
			Summary:  "value not allowed",
			Details:  "",
			Diagnostics: []diags.Diagnostic{
				{
					Message:     fmt.Sprintf("value is not allowed to match `%s`.", c.valueRe.anchored),
					Pos:         label.Value.Pos,
					FirstColumn: 1,
					LastColumn:  len(label.Value.Value) - 1,
				},
			},
			Severity: c.severity,
		})
	}
	return problems
}
