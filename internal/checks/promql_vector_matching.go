package checks

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	promParser "github.com/prometheus/prometheus/promql/parser"
	"golang.org/x/exp/slices"

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

func (c VectorMatchingCheck) Meta() CheckMeta {
	return CheckMeta{IsOnline: true}
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

		var lhsVec, rhsVec bool
		lhsMatchers := map[string]string{}
		if lhs, ok := n.LHS.(*promParser.VectorSelector); ok {
			lhsVec = true
			for _, lm := range lhs.LabelMatchers {
				if lm.Name != labels.MetricName && lm.Type == labels.MatchEqual {
					if n.VectorMatching.On != slices.Contains(n.VectorMatching.MatchingLabels, lm.Name) {
						continue
					}
					lhsMatchers[lm.Name] = lm.Value
				}
			}
		}
		rhsMatchers := map[string]string{}
		if rhs, ok := n.RHS.(*promParser.VectorSelector); ok {
			rhsVec = true
			for _, lm := range rhs.LabelMatchers {
				if lm.Name != labels.MetricName && lm.Type == labels.MatchEqual {
					rhsMatchers[lm.Name] = lm.Value
				}
			}
		}
		if lhsVec && rhsVec {
			for k, lv := range lhsMatchers {
				if rv, ok := rhsMatchers[k]; ok && rv != lv {
					problems = append(problems, exprProblem{
						expr:     node.Expr,
						text:     fmt.Sprintf("left hand side uses {%s=%q} while right hand side uses {%s=%q}, this will never match", k, lv, k, rv),
						severity: Bug,
					})
					return
				}
			}
		}

		leftLabels, err := c.seriesLabels(ctx, n.LHS.String(), ignored...)
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

		rightLabels, err := c.seriesLabels(ctx, n.RHS.String(), ignored...)
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
				if !leftLabels.hasName(name) && rightLabels.hasName(name) {
					problems = append(problems, exprProblem{
						expr:     node.Expr,
						text:     fmt.Sprintf("using on(%q) won't produce any results because left hand side of the query doesn't have this label: %q", name, node.Node.(*promParser.BinaryExpr).LHS),
						severity: Bug,
					})
				}
				if leftLabels.hasName(name) && !rightLabels.hasName(name) {
					problems = append(problems, exprProblem{
						expr:     node.Expr,
						text:     fmt.Sprintf("using on(%q) won't produce any results because right hand side of the query doesn't have this label: %q", name, node.Node.(*promParser.BinaryExpr).RHS),
						severity: Bug,
					})
				}
				if !leftLabels.hasName(name) && !rightLabels.hasName(name) {
					problems = append(problems, exprProblem{
						expr:     node.Expr,
						text:     fmt.Sprintf("using on(%q) won't produce any results because both sides of the query don't have this label", name),
						severity: Bug,
					})
				}
			}
		} else if !leftLabels.overlaps(rightLabels) {
			l, r := leftLabels.getFirstNonOverlap(rightLabels)
			if len(n.VectorMatching.MatchingLabels) == 0 {
				problems = append(problems, exprProblem{
					expr:     node.Expr,
					text:     fmt.Sprintf("both sides of the query have different labels: %s != %s", l, r),
					severity: Bug,
				})
			} else {
				problems = append(problems, exprProblem{
					expr:     node.Expr,
					text:     fmt.Sprintf("using ignoring(%q) won't produce any results because both sides of the query have different labels: %s != %s", strings.Join(n.VectorMatching.MatchingLabels, ","), l, r),
					severity: Bug,
				})
			}
		}
	}

NEXT:
	for _, child := range node.Children {
		problems = append(problems, c.checkNode(ctx, child)...)
	}

	return
}

func (c VectorMatchingCheck) seriesLabels(ctx context.Context, query string, ignored ...model.LabelName) (labelSets, error) {
	qr, err := c.prom.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	if len(qr.Series) == 0 {
		return nil, nil
	}

	var lsets labelSets
	for _, s := range qr.Series {
		var ls labelSet
		for k := range s.Metric {
			var isIgnored bool
			for _, i := range ignored {
				if k == i {
					isIgnored = true
				}
			}
			if !isIgnored {
				ls.add(string(k))
			}
		}
		if len(ls.names) > 1 {
			sort.Strings(ls.names)
		}
		lsets = append(lsets, ls)
	}

	return lsets, nil
}

type labelSet struct {
	names []string
}

func (ls labelSet) String() string {
	return fmt.Sprintf("[%s]", strings.Join(ls.names, ", "))
}

func (ls *labelSet) add(n string) {
	if ls.hasName(n) {
		return
	}
	ls.names = append(ls.names, n)
}

func (ls labelSet) hasName(n string) bool {
	for _, l := range ls.names {
		if l == n {
			return true
		}
	}
	return false
}

func (ls labelSet) isEqual(b labelSet) bool {
	for _, n := range ls.names {
		if !b.hasName(n) {
			return false
		}
	}
	for _, n := range b.names {
		if !ls.hasName(n) {
			return false
		}
	}
	return true
}

type labelSets []labelSet

func (ls labelSets) hasName(n string) bool {
	for _, l := range ls {
		if l.hasName(n) {
			return true
		}
	}
	return false
}

func (ls labelSets) overlaps(bs labelSets) bool {
	for _, a := range ls {
		for _, b := range bs {
			if a.isEqual(b) {
				return true
			}
		}
	}
	return false
}

func (ls labelSets) getFirstNonOverlap(bs labelSets) (labelSet, labelSet) {
	for _, a := range ls {
		for _, b := range bs {
			if !a.isEqual(b) {
				return a, b
			}
		}
	}
	return labelSet{}, labelSet{}
}
