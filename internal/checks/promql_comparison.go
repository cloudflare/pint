package checks

import (
	"context"

	"github.com/cloudflare/pint/internal/parser"

	promParser "github.com/prometheus/prometheus/promql/parser"
)

const (
	ComparisonCheckName = "promql/comparison"
)

func NewComparisonCheck() ComparisonCheck {
	return ComparisonCheck{}
}

type ComparisonCheck struct {
}

func (c ComparisonCheck) String() string {
	return ComparisonCheckName
}

func (c ComparisonCheck) Reporter() string {
	return ComparisonCheckName
}

func (c ComparisonCheck) Check(ctx context.Context, rule parser.Rule) (problems []Problem) {
	if rule.AlertingRule == nil {
		return
	}

	if rule.AlertingRule.Expr.SyntaxError != nil {
		return
	}

	if isAbsent(rule.Expr().Query) {
		return
	}

	if expr := hasComparision(rule.Expr().Query); expr != nil {
		if expr.ReturnBool {
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

	problems = append(problems, Problem{
		Fragment: rule.AlertingRule.Expr.Value.Value,
		Lines:    rule.AlertingRule.Expr.Lines(),
		Reporter: c.Reporter(),
		Text:     "alert query doesn't have any condition, it will always fire if the metric exists",
		Severity: Warning,
	})

	return
}

func hasComparision(n *parser.PromQLNode) *promParser.BinaryExpr {
	if node, ok := n.Node.(*promParser.BinaryExpr); ok {
		if node.Op.IsComparisonOperator() {
			return node
		}
		if node.Op == promParser.LUNLESS {
			return node
		}
	}

	for _, child := range n.Children {
		if node := hasComparision(child); node != nil {
			return node
		}
	}

	return nil
}

func isAbsent(n *parser.PromQLNode) bool {
	if node, ok := n.Node.(*promParser.Call); ok && (node.Func.Name == "absent") {
		return true
	}
	return false
}
