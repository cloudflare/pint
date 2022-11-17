package checks

import (
	"context"

	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"

	promParser "github.com/prometheus/prometheus/promql/parser"
)

const (
	ComparisonCheckName = "alerts/comparison"
)

func NewComparisonCheck() ComparisonCheck {
	return ComparisonCheck{}
}

type ComparisonCheck struct{}

func (c ComparisonCheck) Meta() CheckMeta {
	return CheckMeta{IsOnline: false}
}

func (c ComparisonCheck) String() string {
	return ComparisonCheckName
}

func (c ComparisonCheck) Reporter() string {
	return ComparisonCheckName
}

func (c ComparisonCheck) Check(ctx context.Context, path string, rule parser.Rule, entries []discovery.Entry) (problems []Problem) {
	if rule.AlertingRule == nil {
		return
	}

	if rule.AlertingRule.Expr.SyntaxError != nil {
		return
	}

	expr := rule.Expr().Query

	if n := hasComparision(expr.Node); n != nil {
		if n.ReturnBool && hasComparision(n.LHS) == nil && hasComparision(n.RHS) == nil {
			problems = append(problems, Problem{
				Fragment: rule.AlertingRule.Expr.Value.Value,
				Lines:    rule.AlertingRule.Expr.Lines(),
				Reporter: c.Reporter(),
				Text:     "alert query uses bool modifier for comparison, this means it will always return a result and the alert will always fire",
				Severity: Bug,
			})
		}
		return
	}

	if hasAbsent(expr) {
		return
	}

	problems = append(problems, Problem{
		Fragment: rule.AlertingRule.Expr.Value.Value,
		Lines:    rule.AlertingRule.Expr.Lines(),
		Reporter: c.Reporter(),
		Text:     "alert query doesn't have any condition, it will always fire if the metric exists",
		Severity: Warning,
	})

	return
}

func hasComparision(n promParser.Node) *promParser.BinaryExpr {
	if node, ok := n.(*promParser.BinaryExpr); ok {
		if node.Op.IsComparisonOperator() {
			return node
		}
		if node.Op == promParser.LUNLESS {
			return node
		}
	}

	for _, child := range promParser.Children(n) {
		if node := hasComparision(child); node != nil {
			return node
		}
	}

	return nil
}

func hasAbsent(n *parser.PromQLNode) bool {
	if node, ok := n.Node.(*promParser.Call); ok && (node.Func.Name == "absent") {
		return true
	}
	for _, child := range n.Children {
		if hasAbsent(child) {
			return true
		}
	}
	return false
}
