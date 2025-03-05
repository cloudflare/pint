package checks

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	promParser "github.com/prometheus/prometheus/promql/parser"

	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/parser/utils"
	"github.com/cloudflare/pint/internal/promapi"
)

const (
	VectorMatchingCheckName    = "promql/vector_matching"
	VectorMatchingCheckDetails = `Trying to match two different time series together will only work if both have the exact same set of labels.
You can match time series with different labels by using special keywords and follow the rules set by PromQL.
[Click here](https://prometheus.io/docs/prometheus/latest/querying/operators/#vector-matching) to read PromQL documentation that explains it.`
)

func NewVectorMatchingCheck(prom *promapi.FailoverGroup) VectorMatchingCheck {
	return VectorMatchingCheck{prom: prom}
}

type VectorMatchingCheck struct {
	prom *promapi.FailoverGroup
}

func (c VectorMatchingCheck) Meta() CheckMeta {
	return CheckMeta{
		States: []discovery.ChangeType{
			discovery.Noop,
			discovery.Added,
			discovery.Modified,
			discovery.Moved,
		},
		Online:        true,
		AlwaysEnabled: false,
	}
}

func (c VectorMatchingCheck) String() string {
	return fmt.Sprintf("%s(%s)", VectorMatchingCheckName, c.prom.Name())
}

func (c VectorMatchingCheck) Reporter() string {
	return VectorMatchingCheckName
}

func (c VectorMatchingCheck) Check(ctx context.Context, _ discovery.Path, rule parser.Rule, _ []discovery.Entry) (problems []Problem) {
	expr := rule.Expr()
	if expr.SyntaxError != nil {
		return nil
	}

	for _, problem := range c.checkNode(ctx, expr.Query) {
		problems = append(problems, Problem{
			Lines:    expr.Value.Lines,
			Reporter: c.Reporter(),
			Summary:  problem.summary,
			Details:  problem.details,
			Severity: problem.severity,
		})
	}

	return problems
}

func (c VectorMatchingCheck) checkNode(ctx context.Context, node *parser.PromQLNode) (problems []exprProblem) {
	if n, ok := utils.RemoveConditions(node.Expr.String()).(*promParser.BinaryExpr); ok &&
		n.VectorMatching != nil &&
		n.Op != promParser.LOR &&
		n.Op != promParser.LUNLESS {

		q := fmt.Sprintf("count(%s)", n.String())
		qr, err := c.prom.Query(ctx, q)
		if err != nil {
			text, severity := textAndSeverityFromError(err, c.Reporter(), c.prom.Name(), Bug)
			problems = append(problems, exprProblem{
				summary:  text,
				severity: severity,
			})
			return problems
		}
		if len(qr.Series) > 0 {
			return problems
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
						summary:  fmt.Sprintf("The left hand side uses `{%s=%q}` while the right hand side uses `{%s=%q}`, this will never match.", k, lv, k, rv),
						details:  VectorMatchingCheckDetails,
						severity: Bug,
					})
					return problems
				}
			}
		}

		leftLabels, err := c.seriesLabels(ctx, n.LHS.String(), ignored...)
		if err != nil {
			text, severity := textAndSeverityFromError(err, c.Reporter(), c.prom.Name(), Bug)
			problems = append(problems, exprProblem{
				summary:  text,
				severity: severity,
			})
			return problems
		}
		if leftLabels == nil {
			goto NEXT
		}

		rightLabels, err := c.seriesLabels(ctx, n.RHS.String(), ignored...)
		if err != nil {
			text, severity := textAndSeverityFromError(err, c.Reporter(), c.prom.Name(), Bug)
			problems = append(problems, exprProblem{
				summary:  text,
				details:  "",
				severity: severity,
			})
			return problems
		}
		if rightLabels == nil {
			goto NEXT
		}

		if n.VectorMatching.On {
			for _, name := range n.VectorMatching.MatchingLabels {
				if !leftLabels.hasName(name) && rightLabels.hasName(name) {
					problems = append(problems, exprProblem{
						summary: fmt.Sprintf(
							"Using `on(%s)` won't produce any results on %s because results from the left hand side of the query don't have this label: `%s`.",
							name, promText(c.prom.Name(), qr.URI), node.Expr.(*promParser.BinaryExpr).LHS),
						details:  VectorMatchingCheckDetails,
						severity: Bug,
					})
				}
				if leftLabels.hasName(name) && !rightLabels.hasName(name) {
					problems = append(problems, exprProblem{
						summary: fmt.Sprintf("Using `on(%s)` won't produce any results on %s because results from the right hand side of the query don't have this label: `%s`.",
							name, promText(c.prom.Name(), qr.URI), node.Expr.(*promParser.BinaryExpr).RHS),
						details:  VectorMatchingCheckDetails,
						severity: Bug,
					})
				}
				if !leftLabels.hasName(name) && !rightLabels.hasName(name) {
					problems = append(problems, exprProblem{
						summary: fmt.Sprintf("Using `on(%s)` won't produce any results on %s because results from both sides of the query don't have this label: `%s`.",
							name, promText(c.prom.Name(), qr.URI), node.Expr),
						details:  VectorMatchingCheckDetails,
						severity: Bug,
					})
				}
			}
		} else if !leftLabels.overlaps(rightLabels) {
			l, r := leftLabels.getFirstNonOverlap(rightLabels)
			if len(n.VectorMatching.MatchingLabels) == 0 {
				problems = append(problems, exprProblem{
					summary: fmt.Sprintf("This query will never return anything on %s because results from the right and the left hand side have different labels: `%s` != `%s`. Failing query: `%s`.",
						promText(c.prom.Name(), qr.URI), l, r, node.Expr),
					details:  VectorMatchingCheckDetails,
					severity: Bug,
				})
			} else {
				problems = append(problems, exprProblem{
					summary: fmt.Sprintf("Using `ignoring(%s)` won't produce any results on %s because results from both sides of the query have different labels: `%s` != `%s`. Failing query: `%s`.",
						strings.Join(n.VectorMatching.MatchingLabels, ","), promText(c.prom.Name(), qr.URI), l, r, node.Expr),
					details:  VectorMatchingCheckDetails,
					severity: Bug,
				})
			}
		}
	}

NEXT:
	for _, child := range node.Children {
		problems = append(problems, c.checkNode(ctx, child)...)
	}

	return problems
}

func (c VectorMatchingCheck) seriesLabels(ctx context.Context, query string, ignored ...model.LabelName) (labelSets, error) {
	var expr strings.Builder
	expr.WriteString("count(")
	expr.WriteString(query)
	expr.WriteString(") without(")
	for i, ln := range ignored {
		expr.WriteString(string(ln))
		if i < (len(ignored) - 1) {
			expr.WriteString(",")
		}
	}
	expr.WriteString(")")
	qr, err := c.prom.Query(ctx, expr.String())
	if err != nil {
		return nil, err
	}

	if len(qr.Series) == 0 {
		return nil, nil
	}

	var lsets labelSets
	for _, s := range qr.Series {
		var ls labelSet
		s.Labels.Range(func(l labels.Label) {
			ls.add(l.Name)
		})
		if len(ls.names) > 1 {
			slices.Sort(ls.names)
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

func (ls labelSets) getFirstNonOverlap(bs labelSets) (l, r labelSet) {
	for _, a := range ls {
		for _, b := range bs {
			if !a.isEqual(b) {
				l = a
				r = b
				goto DONE
			}
		}
	}
DONE:
	return l, r
}
