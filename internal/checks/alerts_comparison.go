package checks

import (
	"context"

	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser/utils"
)

const (
	ComparisonCheckName    = "alerts/comparison"
	ComparisonCheckDetails = `Prometheus alerting rules will trigger an alert for each query that returns *any* result.
Unless you do want an alert to always fire you should write your query in a way that returns results only when some condition is met.
In most cases this can be achieved by having some condition in the query expression.
For example ` + "`" + `up == 0` + "`" + " or " + "`" + "rate(error_total[2m]) > 0" + "`." + `
Be careful as some PromQL operations will cause the query to always return the results, for example using the [bool modifier](https://prometheus.io/docs/prometheus/latest/querying/operators/#comparison-binary-operators).`
)

func NewComparisonCheck() ComparisonCheck {
	return ComparisonCheck{}
}

type ComparisonCheck struct{}

func (c ComparisonCheck) Meta() CheckMeta {
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

func (c ComparisonCheck) String() string {
	return ComparisonCheckName
}

func (c ComparisonCheck) Reporter() string {
	return ComparisonCheckName
}

func (c ComparisonCheck) Check(_ context.Context, entry discovery.Entry, _ []discovery.Entry) (problems []Problem) {
	if entry.Rule.AlertingRule == nil {
		return problems
	}

	if entry.Rule.AlertingRule.Expr.SyntaxError != nil {
		return problems
	}

	expr := entry.Rule.Expr()
	srcs := utils.LabelsSource(expr.Value.Value, expr.Query.Expr)
	var msg string
	for _, src := range srcs {
		if src.DeadInfo != nil {
			continue
		}
		if len(src.Unless) > 0 {
			continue
		}
		for _, s := range src.Joins {
			if s.Src.DeadInfo == nil && s.Src.IsConditional {
				goto NEXT
			}
		}
		if src.Operation == "absent" || src.Operation == "absent_over_time" {
			continue
		}

		switch {
		case src.ReturnInfo.AlwaysReturns && !src.IsConditional:
			if len(srcs) == 1 {
				msg = "This query will always return a result and so this alert will always fire."
			} else {
				msg = "If other parts of this query don't return anything then this part will always return a result and so this alert will fire."
			}
		case src.ReturnInfo.IsReturnBool:
			msg = "Results of this query are using the `bool` modifier, which means it will always return a result and the alert will always fire."
		case !src.IsConditional:
			msg = "This query doesn't have any condition and so this alert will always fire if it matches anything."
		default:
			msg = ""
		}

		if msg != "" {
			problems = append(problems, Problem{
				Anchor:   AnchorAfter,
				Lines:    entry.Rule.AlertingRule.Expr.Value.Pos.Lines(),
				Reporter: c.Reporter(),
				Summary:  "always firing alert",
				Details:  ComparisonCheckDetails,
				Severity: Warning,
				Diagnostics: []diags.Diagnostic{
					{
						Message:     msg,
						Pos:         entry.Rule.AlertingRule.Expr.Value.Pos,
						FirstColumn: int(src.Position.Start) + 1,
						LastColumn:  int(src.Position.End),
						Kind:        diags.Issue,
					},
				},
			})
		}
	NEXT:
	}

	return problems
}
