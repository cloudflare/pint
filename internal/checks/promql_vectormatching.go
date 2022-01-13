package checks

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
	"github.com/prometheus/common/model"
	promParser "github.com/prometheus/prometheus/promql/parser"
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

func (c VectorMatchingCheck) Check(rule parser.Rule) (problems []Problem) {
	expr := rule.Expr()
	if expr.SyntaxError != nil {
		return nil
	}

	for _, problem := range c.checkNode(expr.Query) {
		problems = append(problems, Problem{
			Fragment: problem.expr,
			Lines:    expr.Lines(),
			Reporter: VectorMatchingCheckName,
			Text:     problem.text,
			Severity: Bug,
		})
	}

	return
}

func (c VectorMatchingCheck) checkNode(node *parser.PromQLNode) (problems []exprProblem) {
	if n, ok := node.Node.(*promParser.BinaryExpr); ok && n.VectorMatching != nil && n.Op != promParser.LOR && n.Op != promParser.LUNLESS {
		q := fmt.Sprintf("count(%s)", node.Expr)
		qr, err := c.prom.Query(q)
		if err != nil || len(qr.Series) != 0 {
			goto NEXT
		}

		ignored := []model.LabelName{model.MetricNameLabel}
		if !n.VectorMatching.On {
			for _, name := range n.VectorMatching.MatchingLabels {
				ignored = append(ignored, model.LabelName(name))
			}
		}

		leftLabels, err := c.seriesLabels(fmt.Sprintf("topk(1, %s)", n.LHS.String()), ignored...)
		if leftLabels == nil || err != nil {
			goto NEXT
		}

		rightLabels, err := c.seriesLabels(fmt.Sprintf("topk(1, %s)", n.RHS.String()), ignored...)
		if rightLabels == nil || err != nil {
			goto NEXT
		}

		if n.VectorMatching.On {
			for _, name := range n.VectorMatching.MatchingLabels {
				if !stringInSlice(leftLabels, name) && stringInSlice(rightLabels, name) {
					problems = append(problems, exprProblem{
						expr: node.Expr,
						text: fmt.Sprintf("using on(%q) won't produce any results because left hand side of the query doesn't have this label: %q", name, n.LHS),
					})
				}
				if stringInSlice(leftLabels, name) && !stringInSlice(rightLabels, name) {
					problems = append(problems, exprProblem{
						expr: node.Expr,
						text: fmt.Sprintf("using on(%q) won't produce any results because right hand side of the query doesn't have this label: %q", name, n.RHS),
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
		problems = append(problems, c.checkNode(child)...)
	}

	return
}

func (c VectorMatchingCheck) seriesLabels(query string, ignored ...model.LabelName) ([]string, error) {
	qr, err := c.prom.Query(query)
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
