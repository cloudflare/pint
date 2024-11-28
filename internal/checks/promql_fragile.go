package checks

import (
	"context"
	"fmt"
	"slices"

	promParser "github.com/prometheus/prometheus/promql/parser"

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
		IsOnline: false,
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

	for _, problem := range c.checkAggregation(expr.Value.Value, expr.Query) {
		problems = append(problems, Problem{
			Lines:    expr.Value.Lines,
			Reporter: c.Reporter(),
			Text:     problem.text,
			Details:  problem.details,
			Severity: problem.severity,
		})
	}

	if rule.AlertingRule != nil {
		for _, problem := range c.checkSampling(expr.Value.Value, expr.Query.Expr) {
			problems = append(problems, Problem{
				Lines:    expr.Value.Lines,
				Reporter: c.Reporter(),
				Text:     problem.text,
				Details:  problem.details,
				Severity: problem.severity,
			})
		}
	}

	return problems
}

func (c FragileCheck) checkAggregation(query string, node *parser.PromQLNode) (problems []exprProblem) {
	if n := utils.HasOuterBinaryExpr(node); n != nil && n.Op != promParser.LOR && n.Op != promParser.LUNLESS {
		if n.VectorMatching != nil && n.VectorMatching.On {
			goto NEXT
		}
		if _, ok := n.LHS.(*promParser.NumberLiteral); ok {
			goto NEXT
		}
		if _, ok := n.RHS.(*promParser.NumberLiteral); ok {
			goto NEXT
		}
		var isFragile bool
		for _, child := range node.Children {
			for _, src := range utils.LabelsSource(query, child.Expr) {
				if src.Type == utils.AggregateSource && !src.FixedLabels {
					isFragile = true
				}
			}
		}
		if !isFragile {
			goto NEXT
		}

		// don't report any issues if query uses same metric for both sides
		series := map[string]struct{}{}
		for _, ac := range node.Children {
			for _, vs := range utils.HasVectorSelector(ac) {
				series[vs.Name] = struct{}{}
			}
		}
		if len(series) >= 2 {
			p := exprProblem{
				text:     "Aggregation using `without()` can be fragile when used inside binary expression because both sides must have identical sets of labels to produce any results, adding or removing labels to metrics used here can easily break the query, consider aggregating using `by()` to ensure consistent labels.",
				severity: Warning,
			}
			problems = append(problems, p)
			return problems
		}
	}

NEXT:
	for _, child := range node.Children {
		problems = append(problems, c.checkAggregation(query, child)...)
	}

	return problems
}

func (c FragileCheck) checkSampling(expr string, node promParser.Node) (problems []exprProblem) {
	for _, src := range utils.LabelsSource(expr, node) {
		if src.Type != utils.AggregateSource {
			continue
		}
		if src.FixedLabels && len(src.IncludedLabels) == 0 {
			continue
		}
		if !slices.Contains([]string{"topk", "bottomk", "limit", "limit_ratio"}, src.Operation) {
			continue
		}
		problems = append(problems, exprProblem{
			text:     fmt.Sprintf("Using `%s` to select time series might return different set of time series on every query, which would cause flapping alerts.", src.Operation),
			details:  FragileCheckSamplingDetails,
			severity: Warning,
		})
	}
	return problems
}
