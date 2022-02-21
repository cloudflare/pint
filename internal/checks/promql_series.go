package checks

import (
	"context"
	"fmt"

	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"

	"github.com/prometheus/prometheus/model/labels"
	promParser "github.com/prometheus/prometheus/promql/parser"
)

const (
	SeriesCheckName = "promql/series"
)

func NewSeriesCheck(prom *promapi.FailoverGroup) SeriesCheck {
	return SeriesCheck{prom: prom}
}

type SeriesCheck struct {
	prom *promapi.FailoverGroup
}

func (c SeriesCheck) String() string {
	return fmt.Sprintf("%s(%s)", SeriesCheckName, c.prom.Name())
}

func (c SeriesCheck) Reporter() string {
	return SeriesCheckName
}

func (c SeriesCheck) Check(ctx context.Context, rule parser.Rule) (problems []Problem) {
	expr := rule.Expr()

	if expr.SyntaxError != nil {
		return
	}

	done := map[string]bool{}

	for _, selector := range getSelectors(expr.Query) {
		bareSelector := stripLabels(selector)
		c1 := fmt.Sprintf("disable %s(%s)", SeriesCheckName, selector.String())
		c2 := fmt.Sprintf("disable %s(%s)", SeriesCheckName, bareSelector.String())
		if rule.HasComment(c1) || rule.HasComment(c2) {
			done[selector.String()] = true
			continue
		}
		if _, ok := done[selector.String()]; ok {
			continue
		}
		problems = append(problems, c.countSeries(ctx, expr, selector)...)
		done[selector.String()] = true
	}

	return
}

func (c SeriesCheck) countSeries(ctx context.Context, expr parser.PromQLExpr, selector promParser.VectorSelector) (problems []Problem) {
	q := fmt.Sprintf("count(%s)", selector.String())
	qr, err := c.prom.Query(ctx, q)
	if err != nil {
		text, severity := textAndSeverityFromError(err, c.Reporter(), c.prom.Name(), Bug)
		problems = append(problems, Problem{
			Fragment: selector.String(),
			Lines:    expr.Lines(),
			Reporter: c.Reporter(),
			Text:     text,
			Severity: severity,
		})
		return
	}

	var series int
	for _, s := range qr.Series {
		series += int(s.Value)
	}

	if series == 0 {
		if len(selector.LabelMatchers) > 1 {
			// retry selector with only __name__ label
			s := stripLabels(selector)
			p := c.countSeries(ctx, expr, s)
			// if we have zero series without any label selector then the whole
			// series is missing, but if we have some then report missing series
			// with labels
			if len(p) > 0 {
				problems = append(problems, p...)
				return
			}
		}
		problems = append(problems, Problem{
			Fragment: selector.String(),
			Lines:    expr.Lines(),
			Reporter: c.Reporter(),
			Text:     fmt.Sprintf("query using %s completed without any results for %s", c.prom.Name(), selector.String()),
			Severity: Warning,
		})
		return
	}

	return nil
}

func getSelectors(n *parser.PromQLNode) (selectors []promParser.VectorSelector) {
	if node, ok := n.Node.(*promParser.VectorSelector); ok {
		// copy node without offset
		nc := promParser.VectorSelector{
			Name:          node.Name,
			LabelMatchers: node.LabelMatchers,
		}
		selectors = append(selectors, nc)
	}

	for _, child := range n.Children {
		selectors = append(selectors, getSelectors(child)...)
	}

	return
}

func stripLabels(selector promParser.VectorSelector) promParser.VectorSelector {
	s := promParser.VectorSelector{
		Name:          selector.Name,
		LabelMatchers: []*labels.Matcher{},
	}
	for _, lm := range selector.LabelMatchers {
		if lm.Name == labels.MetricName {
			s.LabelMatchers = append(s.LabelMatchers, lm)
		}
	}
	return s
}
