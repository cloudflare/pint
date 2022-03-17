package checks

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/prometheus/common/model"
	promParser "github.com/prometheus/prometheus/promql/parser"

	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/parser/utils"
	"github.com/cloudflare/pint/internal/promapi"
)

const (
	VectorMatchingCheckName = "promql/vector_matching"
)

func NewVectorMatchingCheck(prom *promapi.FailoverGroup) VectorMatchingCheck {
	return VectorMatchingCheck{prom: prom}
}

type VectorMatchingCheck struct {
	prom *promapi.FailoverGroup
}

func (c VectorMatchingCheck) String() string {
	return fmt.Sprintf("%s(%s)", VectorMatchingCheckName, c.prom.Name())
}

func (c VectorMatchingCheck) Reporter() string {
	return VectorMatchingCheckName
}

func (c VectorMatchingCheck) Check(ctx context.Context, rule parser.Rule, entries []discovery.Entry) (problems []Problem) {
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
			Severity: problem.severity,
		})
	}

	return
}

func (c VectorMatchingCheck) checkNode(ctx context.Context, node *parser.PromQLNode) (problems []exprProblem) {
	if n, ok := utils.RemoveConditions(node.Node.String()).(*promParser.BinaryExpr); ok &&
		n.VectorMatching != nil &&
		n.Op != promParser.LOR &&
		n.Op != promParser.LUNLESS {

		q := fmt.Sprintf("count(%s)", n.String())
		qr, err := c.prom.Query(ctx, q)
		if err != nil {
			text, severity := textAndSeverityFromError(err, c.Reporter(), c.prom.Name(), Bug)
			problems = append(problems, exprProblem{
				expr:     node.Expr,
				text:     text,
				severity: severity,
			})
			return
		}
		if len(qr.Series) > 0 {
			return
		}

		ignored := []model.LabelName{model.MetricNameLabel}
		if !n.VectorMatching.On {
			for _, name := range n.VectorMatching.MatchingLabels {
				ignored = append(ignored, model.LabelName(name))
			}
		}

		leftLabels, err := c.seriesLabels(ctx, fmt.Sprintf("topk(1, %s)", n.LHS.String()), ignored...)
		if err != nil {
			text, severity := textAndSeverityFromError(err, c.Reporter(), c.prom.Name(), Bug)
			problems = append(problems, exprProblem{
				expr:     node.Expr,
				text:     text,
				severity: severity,
			})
			return
		}
		if leftLabels == nil {
			goto NEXT
		}

		rightLabels, err := c.seriesLabels(ctx, fmt.Sprintf("topk(1, %s)", n.RHS.String()), ignored...)
		if err != nil {
			text, severity := textAndSeverityFromError(err, c.Reporter(), c.prom.Name(), Bug)
			problems = append(problems, exprProblem{
				expr:     node.Expr,
				text:     text,
				severity: severity,
			})
			return
		}
		if rightLabels == nil {
			goto NEXT
		}

		if n.VectorMatching.On {
			for _, name := range n.VectorMatching.MatchingLabels {
				if !stringInSlice(leftLabels, name) && stringInSlice(rightLabels, name) {
					problems = append(problems, exprProblem{
						expr:     node.Expr,
						text:     fmt.Sprintf("using on(%q) won't produce any results because left hand side of the query doesn't have this label: %q", name, node.Node.(*promParser.BinaryExpr).LHS),
						severity: Bug,
					})
				}
				if stringInSlice(leftLabels, name) && !stringInSlice(rightLabels, name) {
					problems = append(problems, exprProblem{
						expr:     node.Expr,
						text:     fmt.Sprintf("using on(%q) won't produce any results because right hand side of the query doesn't have this label: %q", name, node.Node.(*promParser.BinaryExpr).RHS),
						severity: Bug,
					})
				}
				if !stringInSlice(leftLabels, name) && !stringInSlice(rightLabels, name) {
					problems = append(problems, exprProblem{
						expr:     node.Expr,
						text:     fmt.Sprintf("using on(%q) won't produce any results because both sides of the query don't have this label", name),
						severity: Bug,
					})
				}
			}
		} else {
			if !areStringSlicesEqual(leftLabels, rightLabels) {
				if len(n.VectorMatching.MatchingLabels) == 0 {
					problems = append(problems, exprProblem{
						expr:     node.Expr,
						text:     fmt.Sprintf("both sides of the query have different labels: %s != %s", leftLabels, rightLabels),
						severity: Bug,
					})
				} else {
					problems = append(problems, exprProblem{
						expr:     node.Expr,
						text:     fmt.Sprintf("using ignoring(%q) won't produce any results because both sides of the query have different labels: %s != %s", strings.Join(n.VectorMatching.MatchingLabels, ","), leftLabels, rightLabels),
						severity: Bug,
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
