package checks

import (
	"context"

	promParser "github.com/prometheus/prometheus/promql/parser"

	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/parser/utils"
)

const (
	FragileCheckName = "promql/fragile"
)

func NewFragileCheck() FragileCheck {
	return FragileCheck{}
}

type FragileCheck struct{}

func (c FragileCheck) Meta() CheckMeta {
	return CheckMeta{IsOnline: false}
}

func (c FragileCheck) String() string {
	return FragileCheckName
}

func (c FragileCheck) Reporter() string {
	return FragileCheckName
}

func (c FragileCheck) Check(_ context.Context, _ string, rule parser.Rule, _ []discovery.Entry) (problems []Problem) {
	expr := rule.Expr()
	if expr.SyntaxError != nil {
		return nil
	}

	for _, problem := range c.checkNode(expr.Query) {
		problems = append(problems, Problem{
			Fragment: problem.expr,
			Lines:    expr.Lines(),
			Reporter: c.Reporter(),
			Text:     problem.text,
			Severity: Warning,
		})
	}
	return problems
}

func (c FragileCheck) checkNode(node *parser.PromQLNode) (problems []exprProblem) {
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
			for _, agg := range utils.HasOuterAggregation(child) {
				if agg.Without {
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
				expr:     node.Expr,
				text:     "aggregation using without() can be fragile when used inside binary expression because both sides must have identical sets of labels to produce any results, adding or removing labels to metrics used here can easily break the query, consider aggregating using by() to ensure consistent labels",
				severity: Warning,
			}
			problems = append(problems, p)
			return problems
		}
	}

NEXT:
	for _, child := range node.Children {
		problems = append(problems, c.checkNode(child)...)
	}

	return problems
}
