package checks

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	promParser "github.com/prometheus/prometheus/promql/parser"
	"github.com/prometheus/prometheus/promql/parser/posrange"

	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/parser/source"
	"github.com/cloudflare/pint/internal/promapi"
)

const (
	VectorMatchingCheckName    = "promql/vector_matching"
	VectorMatchingCheckDetails = `Trying to match two different time series together will only work if both have the exact same set of labels.
You can match time series with different labels by using special keywords and follow the rules set by PromQL.
[Click here](https://prometheus.io/docs/prometheus/latest/querying/operators/#vector-matching) to read PromQL documentation that explains it.`
)

func NewVectorMatchingCheck(prom *promapi.FailoverGroup) VectorMatchingCheck {
	return VectorMatchingCheck{
		prom:     prom,
		instance: fmt.Sprintf("%s(%s)", VectorMatchingCheckName, prom.Name()),
	}
}

type VectorMatchingCheck struct {
	prom     *promapi.FailoverGroup
	instance string
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
	return c.instance
}

func (c VectorMatchingCheck) Reporter() string {
	return VectorMatchingCheckName
}

func (c VectorMatchingCheck) Check(ctx context.Context, entry *discovery.Entry, _ []*discovery.Entry) (problems []Problem) {
	expr := entry.Rule.Expr()
	if expr.SyntaxError() != nil {
		return nil
	}
	problems = append(problems, c.checkNode(ctx, entry.Rule, expr, expr.Query())...)
	return problems
}

func (c VectorMatchingCheck) checkNode(ctx context.Context, rule parser.Rule, expr *parser.PromQLExpr, node *parser.PromQLNode) (problems []Problem) {
	binExpr, ok := node.Expr.(*promParser.BinaryExpr)
	if !ok {
		goto NEXT
	}

	{
		binPos := binExpr.PositionRange()
		lhsPos := binExpr.LHS.PositionRange()
		rhsPos := binExpr.RHS.PositionRange()
		lhsSources := source.LabelsSource(expr.Value.Value, binExpr.LHS)
		rhsSources := source.LabelsSource(expr.Value.Value, binExpr.RHS)

		n, ok := removeConditions(binExpr).(*promParser.BinaryExpr)
		if !ok || n.VectorMatching == nil ||
			n.Op == promParser.LOR ||
			n.Op == promParser.LUNLESS {
			goto NEXT
		}

		if !canJoinStatically(lhsSources, rhsSources, n.VectorMatching) {
			goto NEXT
		}

		q := wrapExpr(n.String(), "count")
		qr, err := c.prom.Query(ctx, q).Wait()
		if err != nil {
			problems = append(problems, problemFromError(err, rule, c.Reporter(), c.prom.Name(), Bug))
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
				if lm.Name != model.MetricNameLabel && lm.Type == labels.MatchEqual {
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
				if lm.Name != model.MetricNameLabel && lm.Type == labels.MatchEqual {
					rhsMatchers[lm.Name] = lm.Value
				}
			}
		}
		if lhsVec && rhsVec {
			for k, lv := range lhsMatchers {
				if rv, ok := rhsMatchers[k]; ok && rv != lv {
					problems = append(problems, Problem{
						Anchor:   AnchorAfter,
						Lines:    expr.Value.Pos.Lines(),
						Reporter: c.Reporter(),
						Summary:  "impossible binary operation",
						Details:  VectorMatchingCheckDetails,
						Severity: Bug,
						Diagnostics: []diags.Diagnostic{
							{
								Message:     fmt.Sprintf("The left hand side uses `{%s=%q}` while the right hand side uses `{%s=%q}`, this will never match.", k, lv, k, rv),
								Pos:         expr.Value.Pos,
								Expr:        expr.Query().Expr,
								FirstColumn: int(binPos.Start) + 1,
								LastColumn:  int(binPos.End),
								Kind:        diags.Issue,
							},
						},
					})
					return problems
				}
			}
		}

		leftLabels, leftURI, err := c.seriesLabels(ctx, n.LHS.String(), ignored...)
		if err != nil {
			problems = append(problems, problemFromError(err, rule, c.Reporter(), c.prom.Name(), Bug))
			return problems
		}
		if leftLabels == nil {
			goto NEXT
		}

		rightLabels, rightURI, err := c.seriesLabels(ctx, n.RHS.String(), ignored...)
		if err != nil {
			problems = append(problems, problemFromError(err, rule, c.Reporter(), c.prom.Name(), Bug))
			return problems
		}
		if rightLabels == nil {
			goto NEXT
		}

		if n.VectorMatching.On {
			for _, name := range n.VectorMatching.MatchingLabels {
				if !leftLabels.hasName(name) && rightLabels.hasName(name) {
					onPos := source.FindFuncPosition(expr.Value.Value, binPos, "on", []posrange.PositionRange{
						lhsPos, rhsPos,
					})
					link := fmt.Sprintf("%s/query?g0.expr=%s&&g0.tab=table", leftURI, url.QueryEscape(n.LHS.String()))
					problems = append(problems, Problem{
						Anchor:   AnchorAfter,
						Lines:    expr.Value.Pos.Lines(),
						Reporter: c.Reporter(),
						Summary:  "impossible binary operation",
						Details:  VectorMatchingCheckDetails,
						Severity: Bug,
						Diagnostics: []diags.Diagnostic{
							{
								Message: fmt.Sprintf(
									"Using `on(%s)` won't produce any results on %s because [series returned by this query](%s) don't have the `%s` label.",
									name, promText(c.prom.Name(), qr.URI), link, name,
								),
								Pos:         expr.Value.Pos,
								Expr:        expr.Query().Expr,
								FirstColumn: int(onPos.Start) + 1,
								LastColumn:  int(onPos.End),
								Kind:        diags.Issue,
							},
						},
					})
				}
				if leftLabels.hasName(name) && !rightLabels.hasName(name) {
					onPos := source.FindFuncPosition(expr.Value.Value, binPos, "on", []posrange.PositionRange{
						lhsPos, rhsPos,
					})
					link := fmt.Sprintf("%s/query?g0.expr=%s&&g0.tab=table", rightURI, url.QueryEscape(n.RHS.String()))
					problems = append(problems, Problem{
						Anchor:   AnchorAfter,
						Lines:    expr.Value.Pos.Lines(),
						Reporter: c.Reporter(),
						Summary:  "impossible binary operation",
						Details:  VectorMatchingCheckDetails,
						Severity: Bug,
						Diagnostics: []diags.Diagnostic{
							{
								Message: fmt.Sprintf("Using `on(%s)` won't produce any results on %s because [series returned by this query](%s) don't have the `%s` label.",
									name, promText(c.prom.Name(), qr.URI), link, name),
								Pos:         expr.Value.Pos,
								Expr:        expr.Query().Expr,
								FirstColumn: int(onPos.Start) + 1,
								LastColumn:  int(onPos.End),
								Kind:        diags.Issue,
							},
						},
					})
				}
				if !leftLabels.hasName(name) && !rightLabels.hasName(name) {
					pos := source.FindFuncPosition(expr.Value.Value, binPos, "on", []posrange.PositionRange{
						lhsPos, rhsPos,
					})
					link := fmt.Sprintf("%s/query?g0.expr=%s&&g0.tab=table", leftURI, url.QueryEscape(n.String()))
					problems = append(problems, Problem{
						Anchor:   AnchorAfter,
						Lines:    expr.Value.Pos.Lines(),
						Reporter: c.Reporter(),
						Summary:  "impossible binary operation",
						Details:  VectorMatchingCheckDetails,
						Severity: Bug,
						Diagnostics: []diags.Diagnostic{
							{
								Message: fmt.Sprintf("Using `on(%s)` won't produce any results on %s because [series returned from both sides of the query](%s) don't have the `%s` label.",
									name, promText(c.prom.Name(), qr.URI), name, link),
								Pos:         expr.Value.Pos,
								Expr:        expr.Query().Expr,
								FirstColumn: int(pos.Start) + 1,
								LastColumn:  int(pos.End),
								Kind:        diags.Issue,
							},
						},
					})
				}
			}
		} else if !leftLabels.overlaps(rightLabels) {
			l, r := leftLabels.getFirstNonOverlap(rightLabels)
			var lhsDiag, rhsDiag diags.Diagnostic
			if n.VectorMatching.Card == promParser.CardOneToMany {
				rhsDiag = diags.Diagnostic{
					Message: fmt.Sprintf(
						"The right hand side of the query on %s returns labels: `%s`, which don't match the left hand side labels: `%s`. This query will never return any results.",
						promText(c.prom.Name(), qr.URI), r, l,
					),
					Pos:         expr.Value.Pos,
					Expr:        expr.Query().Expr,
					FirstColumn: int(rhsPos.Start) + 1,
					LastColumn:  int(rhsPos.End),
					Kind:        diags.Issue,
				}
				lhsDiag = diags.Diagnostic{
					Message: fmt.Sprintf(
						"The left hand side of the query on %s returns labels: `%s`.",
						promText(c.prom.Name(), qr.URI), l,
					),
					Pos:         expr.Value.Pos,
					Expr:        expr.Query().Expr,
					FirstColumn: int(lhsPos.Start) + 1,
					LastColumn:  int(lhsPos.End),
					Kind:        diags.Context,
				}
			} else {
				lhsDiag = diags.Diagnostic{
					Message: fmt.Sprintf(
						"The left hand side of the query on %s returns labels: `%s`, which don't match the right hand side labels: `%s`. This query will never return any results.",
						promText(c.prom.Name(), qr.URI), l, r,
					),
					Pos:         expr.Value.Pos,
					Expr:        expr.Query().Expr,
					FirstColumn: int(lhsPos.Start) + 1,
					LastColumn:  int(lhsPos.End),
					Kind:        diags.Issue,
				}
				rhsDiag = diags.Diagnostic{
					Message: fmt.Sprintf(
						"The right hand side of the query on %s returns labels: `%s`.",
						promText(c.prom.Name(), qr.URI), r,
					),
					Pos:         expr.Value.Pos,
					Expr:        expr.Query().Expr,
					FirstColumn: int(rhsPos.Start) + 1,
					LastColumn:  int(rhsPos.End),
					Kind:        diags.Context,
				}
			}
			problems = append(problems, Problem{
				Anchor:   AnchorAfter,
				Lines:    expr.Value.Pos.Lines(),
				Reporter: c.Reporter(),
				Summary:  "impossible binary operation",
				Details:  VectorMatchingCheckDetails,
				Severity: Bug,
				Diagnostics: []diags.Diagnostic{
					lhsDiag,
					rhsDiag,
				},
			})
		}
	}

NEXT:
	for _, child := range node.Children {
		problems = append(problems, c.checkNode(ctx, rule, expr, child)...)
	}

	return problems
}

func canJoinStatically(lhsSources, rhsSources []*source.Source, vm *promParser.VectorMatching) bool {
	for _, ls := range lhsSources {
		for _, rs := range rhsSources {
			switch {
			case vm.On && len(vm.MatchingLabels) == 0:
				return true
			case vm.On:
				for _, name := range vm.MatchingLabels {
					if ls.CanHaveLabel(name) && !rs.CanHaveLabel(name) {
						return false
					}
					if rs.CanHaveLabel(name) && !ls.CanHaveLabel(name) {
						return false
					}
				}
			default:
				for name, l := range ls.Labels {
					if l.Kind != source.GuaranteedLabel {
						continue
					}
					if slices.Contains(vm.MatchingLabels, name) {
						continue
					}
					if !rs.CanHaveLabel(name) {
						return false
					}
				}
			}
		}
	}
	return true
}

// removeConditions recursively strips scalar comparisons from a PromQL AST.
// It walks through aggregations, function calls (vector/matrix args only),
// subqueries, parens, and unary expressions to find and remove binary
// operations where one side is a number literal or empty selector.
// "foo / bar > 0" becomes "foo / bar".
// "min_over_time((foo > 0)[5m:1m]) / bar" becomes "min_over_time(foo[5m:1m]) / bar".
func removeConditions(node promParser.Node) promParser.Node {
	switch n := node.(type) {
	case *promParser.AggregateExpr:
		return &promParser.AggregateExpr{
			Op:       n.Op,
			Expr:     removeConditions(n.Expr).(promParser.Expr),
			Param:    n.Param,
			Grouping: n.Grouping,
			Without:  n.Without,
			PosRange: n.PosRange,
		}
	case *promParser.BinaryExpr:
		lhs := removeConditions(n.LHS)
		rhs := removeConditions(n.RHS)
		ln := isNumberOrEmpty(lhs)
		rn := isNumberOrEmpty(rhs)
		if ln && rn {
			return &promParser.VectorSelector{}
		}
		if ln {
			return rhs
		}
		if rn {
			return lhs
		}
		return &promParser.BinaryExpr{
			Op:             n.Op,
			LHS:            lhs.(promParser.Expr),
			RHS:            rhs.(promParser.Expr),
			VectorMatching: n.VectorMatching,
			ReturnBool:     n.ReturnBool,
		}
	case *promParser.Call:
		args := make(promParser.Expressions, 0, len(n.Args))
		for i, e := range n.Args {
			var vt promParser.ValueType
			if i >= len(n.Func.ArgTypes) {
				vt = n.Func.ArgTypes[len(n.Func.ArgTypes)-1]
			} else {
				vt = n.Func.ArgTypes[i]
			}
			switch vt {
			case promParser.ValueTypeVector, promParser.ValueTypeMatrix:
				args = append(args, removeConditions(e).(promParser.Expr))
			case promParser.ValueTypeScalar, promParser.ValueTypeString, promParser.ValueTypeNone:
				args = append(args, e)
			}
		}
		return &promParser.Call{
			Func:     n.Func,
			Args:     args,
			PosRange: n.PosRange,
		}
	case *promParser.SubqueryExpr:
		return &promParser.SubqueryExpr{
			Expr:           removeConditions(n.Expr).(promParser.Expr),
			Range:          n.Range,
			OriginalOffset: n.OriginalOffset,
			Offset:         n.Offset,
			Timestamp:      n.Timestamp,
			StartOrEnd:     n.StartOrEnd,
			Step:           n.Step,
			EndPos:         n.EndPos,
		}
	case *promParser.ParenExpr:
		inner := removeConditions(n.Expr).(promParser.Expr)
		switch inner.(type) {
		case *promParser.NumberLiteral, *promParser.StringLiteral, *promParser.VectorSelector, *promParser.MatrixSelector:
			return inner
		}
		return &promParser.ParenExpr{
			Expr:     inner,
			PosRange: n.PosRange,
		}
	case *promParser.UnaryExpr:
		return &promParser.UnaryExpr{
			Op:       n.Op,
			Expr:     removeConditions(n.Expr).(promParser.Expr),
			StartPos: n.StartPos,
		}
	default:
		return node
	}
}

func isNumberOrEmpty(node promParser.Node) bool {
	if _, ok := node.(*promParser.NumberLiteral); ok {
		return true
	}
	v, ok := node.(*promParser.VectorSelector)
	if !ok {
		return false
	}
	return v.Name == ""
}

func (c VectorMatchingCheck) seriesLabels(ctx context.Context, query string, ignored ...model.LabelName) (labelSets, string, error) {
	var expr strings.Builder
	expr.WriteString(wrapExpr(query, "count"))
	expr.WriteString(" without(")
	for i, ln := range ignored {
		expr.WriteString(string(ln))
		if i < (len(ignored) - 1) {
			expr.WriteString(",")
		}
	}
	expr.WriteString(")")
	qr, err := c.prom.Query(ctx, expr.String()).Wait()
	if err != nil {
		return nil, "", err
	}

	if len(qr.Series) == 0 {
		return nil, qr.URI, nil
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

	return lsets, qr.URI, nil
}

type labelSet struct {
	names []string
}

func (ls labelSet) String() string {
	return "[" + strings.Join(ls.names, ", ") + "]"
}

func (ls *labelSet) add(n string) {
	// Label keys are always unique so we can just append here.
	ls.names = append(ls.names, n)
}

func (ls labelSet) hasName(n string) bool {
	return slices.Contains(ls.names, n)
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
		if slices.ContainsFunc(bs, a.isEqual) {
			return true
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
