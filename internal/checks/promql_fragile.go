package checks

import (
	"context"
	"fmt"
	"slices"

	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/parser/utils"
)

const (
	FragileCheckName = "promql/fragile"

	FragileCheckSamplingDetails = `Alerts are identied by labels, two alerts with identical sets of labels are identical.
If two alerts have the same name but the rest of labels isn't 100% identical then they are two different alerts.
If the same alert query returns results that over time have different labels on them then previous alert instances will resolve and new alerts will be fired.
This can happen when using one of the aggregation operation like topk or bottomk as they can return a different time series each time they are evaluated.`
)

func NewFragileCheck() FragileCheck {
	return FragileCheck{}
}

type FragileCheck struct{}

func (c FragileCheck) Meta() CheckMeta {
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

func (c FragileCheck) String() string {
	return FragileCheckName
}

func (c FragileCheck) Reporter() string {
	return FragileCheckName
}

func (c FragileCheck) Check(_ context.Context, _ discovery.Path, rule parser.Rule, _ []discovery.Entry) (problems []Problem) {
	expr := rule.Expr()
	if expr.SyntaxError != nil {
		return nil
	}

	if rule.AlertingRule != nil {
		for _, src := range utils.LabelsSource(expr.Value.Value, expr.Query.Expr) {
			if src.Type != utils.AggregateSource {
				continue
			}
			if src.FixedLabels && len(src.IncludedLabels) == 0 {
				continue
			}
			if !slices.Contains([]string{"topk", "bottomk", "limit", "limit_ratio"}, src.Operation) {
				continue
			}
			problems = append(problems, Problem{
				Anchor:   AnchorAfter,
				Lines:    expr.Value.Pos.Lines(),
				Reporter: c.Reporter(),
				Summary:  "fragile query",
				Details:  FragileCheckSamplingDetails,
				Severity: Warning,
				Diagnostics: []diags.Diagnostic{
					{
						Message:     fmt.Sprintf("Using `%s` to select time series might return different set of time series on every query, which would cause flapping alerts.", src.Operation),
						Pos:         expr.Value.Pos,
						FirstColumn: int(src.GetSmallestPosition().Start) + 1,
						LastColumn:  int(src.GetSmallestPosition().End),
					},
				},
			})
		}
	}

	return problems
}
