package checks

import (
	"fmt"
	"time"

	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"

	"github.com/prometheus/prometheus/model/labels"
	promParser "github.com/prometheus/prometheus/promql/parser"
)

const (
	SeriesCheckName = "query/series"
)

func NewSeriesCheck(name, uri string, timeout time.Duration, severity Severity) SeriesCheck {
	return SeriesCheck{name: name, uri: uri, timeout: timeout, severity: severity}
}

type SeriesCheck struct {
	name     string
	uri      string
	timeout  time.Duration
	severity Severity
}

func (c SeriesCheck) String() string {
	return fmt.Sprintf("%s(%s)", SeriesCheckName, c.name)
}

func (c SeriesCheck) Check(rule parser.Rule) (problems []Problem) {
	expr := rule.Expr()

	if expr.SyntaxError != nil {
		return
	}

	done := map[string]bool{}

	for _, selector := range getSelectors(expr.Query) {
		if _, ok := done[selector.String()]; ok {
			continue
		}
		problems = append(problems, c.countSeries(expr, selector)...)
		done[selector.String()] = true
	}

	return
}

func (c SeriesCheck) countSeries(expr parser.PromQLExpr, selector promParser.VectorSelector) (problems []Problem) {
	q := fmt.Sprintf("count(%s)", selector.String())
	qr, err := promapi.Query(c.uri, c.timeout, q, &q)
	if err != nil {
		problems = append(problems, Problem{
			Fragment: selector.String(),
			Lines:    expr.Lines(),
			Reporter: SeriesCheckName,
			Text:     fmt.Sprintf("query using %s failed with: %s", c.name, err),
			Severity: Bug,
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
			s := promParser.VectorSelector{
				Name:          selector.Name,
				LabelMatchers: []*labels.Matcher{},
			}
			for _, lm := range selector.LabelMatchers {
				if lm.Name == labels.MetricName {
					s.LabelMatchers = append(s.LabelMatchers, lm)
				}
			}
			p := c.countSeries(expr, s)
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
			Reporter: SeriesCheckName,
			Text:     fmt.Sprintf("query using %s completed without any results for %s", c.name, selector.String()),
			Severity: c.severity,
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
