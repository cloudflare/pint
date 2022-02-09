package checks

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/prometheus/common/model"
	promParser "github.com/prometheus/prometheus/promql/parser"

	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
)

const (
	VectorMatchingCheckName = "promql/vector_matching"
)

func NewVectorMatchingCheck(prom *promapi.Prometheus) VectorMatchingCheck {
	return VectorMatchingCheck{prom: prom}
}

type VectorMatchingCheck struct {
	prom *promapi.Prometheus
}

func (c VectorMatchingCheck) String() string {
	return fmt.Sprintf("%s(%s)", VectorMatchingCheckName, c.prom.Name())
}

func (c VectorMatchingCheck) Reporter() string {
	return VectorMatchingCheckName
}

func (c VectorMatchingCheck) Check(ctx context.Context, rule parser.Rule) (problems []Problem) {
	expr := rule.Expr()
	if expr.SyntaxError != nil {
		return nil
	}

	for _, problem := range c.checkNode(ctx, expr.Query) {
		problems = append(problems, Problem{
			Fragment: problem.expr,
			Lines:    expr.Lines(),
			Reporter: c.Reporter(),
			Text:     problem.text,
			Severity: Bug,
		})
	}

	return
}

func (c VectorMatchingCheck) checkNode(ctx context.Context, node *parser.PromQLNode) (problems []exprProblem) {
	if n, ok := removeConditions(node.Node.String()).(*promParser.BinaryExpr); ok &&
		n.VectorMatching != nil &&
		n.Op != promParser.LOR &&
		n.Op != promParser.LUNLESS {

		q := fmt.Sprintf("count(%s)", n.String())
		qr, err := c.prom.Query(ctx, q)
		if err == nil && len(qr.Series) > 0 {
			return
		}

		if _, ok := n.LHS.(*promParser.BinaryExpr); ok {
			goto NEXT
		}
		if _, ok := n.RHS.(*promParser.BinaryExpr); ok {
			goto NEXT
		}

		ignored := []model.LabelName{model.MetricNameLabel}
		if !n.VectorMatching.On {
			for _, name := range n.VectorMatching.MatchingLabels {
				ignored = append(ignored, model.LabelName(name))
			}
		}

		leftLabels, err := c.seriesLabels(ctx, fmt.Sprintf("topk(1, %s)", n.LHS.String()), ignored...)
		if leftLabels == nil || err != nil {
			goto NEXT
		}

		rightLabels, err := c.seriesLabels(ctx, fmt.Sprintf("topk(1, %s)", n.RHS.String()), ignored...)
		if rightLabels == nil || err != nil {
			goto NEXT
		}

		if n.VectorMatching.On {
			for _, name := range n.VectorMatching.MatchingLabels {
				if !stringInSlice(leftLabels, name) && stringInSlice(rightLabels, name) {
					problems = append(problems, exprProblem{
						expr: node.Expr,
						text: fmt.Sprintf("using on(%q) won't produce any results because left hand side of the query doesn't have this label: %q", name, node.Node.(*promParser.BinaryExpr).LHS),
					})
				}
				if stringInSlice(leftLabels, name) && !stringInSlice(rightLabels, name) {
					problems = append(problems, exprProblem{
						expr: node.Expr,
						text: fmt.Sprintf("using on(%q) won't produce any results because right hand side of the query doesn't have this label: %q", name, node.Node.(*promParser.BinaryExpr).RHS),
					})
				}
				if !stringInSlice(leftLabels, name) && !stringInSlice(rightLabels, name) {
					problems = append(problems, exprProblem{
						expr: node.Expr,
						text: fmt.Sprintf("using on(%q) won't produce any results because both sides of the query don't have this label", name),
					})
				}
			}
		} else {
			if !areStringSlicesEqual(leftLabels, rightLabels) {
				if len(n.VectorMatching.MatchingLabels) == 0 {
					problems = append(problems, exprProblem{
						expr: node.Expr,
						text: fmt.Sprintf("both sides of the query have different labels: %s != %s", leftLabels, rightLabels),
					})
				} else {
					problems = append(problems, exprProblem{
						expr: node.Expr,
						text: fmt.Sprintf("using ignoring(%q) won't produce any results because both sides of the query have different labels: %s != %s", strings.Join(n.VectorMatching.MatchingLabels, ","), leftLabels, rightLabels),
					})
				}
			}
		}
	}

NEXT:
	for _, child := range node.Children {
		problems = append(problems, c.checkNode(ctx, child)...)
	}

	return
}

func (c VectorMatchingCheck) seriesLabels(ctx context.Context, query string, ignored ...model.LabelName) ([]string, error) {
	qr, err := c.prom.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	if len(qr.Series) == 0 {
		return nil, nil
	}

	names := []string{}
	for _, s := range qr.Series {
		for k := range s.Metric {
			var isIgnored bool
			for _, i := range ignored {
				if k == i {
					isIgnored = true
				}
			}
			if !isIgnored {
				names = append(names, string(k))
			}
		}
	}
	sort.Strings(names)

	return names, nil
}

func stringInSlice(sl []string, v string) bool {
	for _, s := range sl {
		if s == v {
			return true
		}
	}
	return false
}

func areStringSlicesEqual(sla, slb []string) bool {
	return reflect.DeepEqual(sla, slb)
}

func removeConditions(source string) promParser.Node {
	node, _ := promParser.ParseExpr(source)
	switch n := node.(type) {
	case *promParser.AggregateExpr:
		n.Expr = removeConditions(n.Expr.String()).(promParser.Expr)
		return n
	case *promParser.BinaryExpr:
		var ln, rn bool
		if _, ok := n.LHS.(*promParser.NumberLiteral); ok {
			ln = true
		}
		if _, ok := n.RHS.(*promParser.NumberLiteral); ok {
			rn = true
		}
		if ln && rn {
			return &promParser.ParenExpr{}
		}
		if ln {
			return removeConditions(n.RHS.String())
		}
		if rn {
			return removeConditions(n.LHS.String())
		}
		n.LHS = removeConditions(n.LHS.String()).(promParser.Expr)
		n.RHS = removeConditions(n.RHS.String()).(promParser.Expr)
		return n
	case *promParser.Call:
		ret := promParser.Expressions{}
		for _, e := range n.Args {
			ret = append(ret, removeConditions(e.String()).(promParser.Expr))
		}
		n.Args = ret
		return n
	case *promParser.SubqueryExpr:
		n.Expr = removeConditions(n.Expr.String()).(promParser.Expr)
		return n
	case *promParser.ParenExpr:
		n.Expr = removeConditions(n.Expr.String()).(promParser.Expr)
		return n
	case *promParser.UnaryExpr:
		n.Expr = removeConditions(n.Expr.String()).(promParser.Expr)
		return n
	case *promParser.MatrixSelector:
		return node
	case *promParser.StepInvariantExpr:
		n.Expr = removeConditions(n.Expr.String()).(promParser.Expr)
		return n
	case *promParser.NumberLiteral, *promParser.StringLiteral, *promParser.VectorSelector:
		return n
	default:
		return node
	}
}
