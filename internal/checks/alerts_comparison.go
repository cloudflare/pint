package checks

import (
	"context"

	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/parser/utils"

	promParser "github.com/prometheus/prometheus/promql/parser"
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

func (c ComparisonCheck) Check(_ context.Context, _ discovery.Path, rule parser.Rule, _ []discovery.Entry) (problems []Problem) {
	if rule.AlertingRule == nil {
		return problems
	}

	if rule.AlertingRule.Expr.SyntaxError != nil {
		return problems
	}

	expr := rule.Expr()

	if n := utils.HasOuterBinaryExpr(expr.Query); n != nil && n.Op == promParser.LOR {
		if (hasComparision(n.LHS) == nil || hasComparision(n.RHS) == nil) && !isAbsent(n.LHS) && !isAbsent(n.RHS) {
			problems = append(problems, Problem{
				Anchor:   AnchorAfter,
				Lines:    rule.AlertingRule.Expr.Value.Lines,
				Reporter: c.Reporter(),
				Summary:  "always firing alert",
				Details:  ComparisonCheckDetails,
				Severity: rewriteSeverity(Warning, n.LHS, n.RHS),
				Diagnostics: []output.Diagnostic{
					{
						Message:     "Alert query uses `or` operator with one side of the query that will always return a result, this alert will always fire.",
						Pos:         expr.Value.Pos,
						FirstColumn: int(n.PositionRange().Start) + 1,
						LastColumn:  int(n.PositionRange().End),
					},
				},
			})
		}
	}

	if n := hasComparision(expr.Query.Expr); n != nil {
		if n.ReturnBool && hasComparision(n.LHS) == nil && hasComparision(n.RHS) == nil {
			problems = append(problems, Problem{
				Anchor:   AnchorAfter,
				Lines:    rule.AlertingRule.Expr.Value.Lines,
				Reporter: c.Reporter(),
				Summary:  "always firing alert",
				Details:  ComparisonCheckDetails,
				Severity: Bug,
				Diagnostics: []output.Diagnostic{
					{
						Message:     "Alert query uses `bool` modifier for comparison, this means it will always return a result and the alert will always fire.",
						Pos:         expr.Value.Pos,
						FirstColumn: int(n.PositionRange().Start) + 1,
						LastColumn:  int(n.PositionRange().End),
					},
				},
			})
		}
		return problems
	}

	if hasAbsent(expr.Query) {
		return problems
	}

	problems = append(problems, Problem{
		Anchor:   AnchorAfter,
		Lines:    rule.AlertingRule.Expr.Value.Lines,
		Reporter: c.Reporter(),
		Summary:  "always firing alert",
		Details:  ComparisonCheckDetails,
		Severity: Warning,
		Diagnostics: []output.Diagnostic{
			{
				Message:     "Alert query doesn't have any condition, it will always fire if the metric exists.",
				Pos:         expr.Value.Pos,
				FirstColumn: 1,
				LastColumn:  len(expr.Value.Value),
			},
		},
	})

	return problems
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

func isAbsent(node promParser.Node) bool {
	if node, ok := node.(*promParser.Call); ok {
		if node.Func.Name == "absent" || node.Func.Name == "absent_over_time" {
			return true
		}
	}

	for _, child := range promParser.Children(node) {
		if isAbsent(child) {
			return true
		}
	}

	return false
}

func hasAbsent(n *parser.PromQLNode) bool {
	if isAbsent(n.Expr) {
		return true
	}
	for _, child := range n.Children {
		if hasAbsent(child) {
			return true
		}
	}
	return false
}

func rewriteSeverity(s Severity, nodes ...promParser.Node) Severity {
	for _, node := range nodes {
		if n, ok := node.(*promParser.Call); ok && n.Func.Name == "vector" {
			return Bug
		}
	}
	return s
}
