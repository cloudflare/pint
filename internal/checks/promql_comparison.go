package checks

import (
	"github.com/cloudflare/pint/internal/parser"

	promParser "github.com/prometheus/prometheus/promql/parser"
)

const (
	ComparisonCheckName = "promql/comparison"
)

func NewComparisonCheck(severity Severity) ComparisonCheck {
	return ComparisonCheck{severity: severity}
}

type ComparisonCheck struct {
	severity Severity
}

func (c ComparisonCheck) String() string {
	return ComparisonCheckName
}

func (c ComparisonCheck) Check(rule parser.Rule) (problems []Problem) {
	if rule.AlertingRule == nil {
		return
	}

	if rule.AlertingRule.Expr.SyntaxError != nil {
		return
	}

	if hasComparision(rule.Expr().Query) {
		return
	}

	problems = append(problems, Problem{
		Fragment: rule.AlertingRule.Expr.Value.Value,
		Lines:    rule.AlertingRule.Expr.Lines(),
		Reporter: AlertsCheckName,
		Text:     "alert query doesn't have any condition, it will always fire if the metric exists",
		Severity: c.severity,
	})

	return
}

func hasComparision(n *parser.PromQLNode) bool {
	if node, ok := n.Node.(*promParser.BinaryExpr); ok {
		if node.Op.IsComparisonOperator() {
			return true
		}
		if node.Op == promParser.LUNLESS {
			return true
		}
	}

	for _, child := range n.Children {
		if hasComparision(child) {
			return true
		}
	}

	return false
}
