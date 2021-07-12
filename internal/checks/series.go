package checks

import (
	"fmt"
	"time"

	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"

	"github.com/prometheus/prometheus/pkg/labels"
	promParser "github.com/prometheus/prometheus/promql/parser"
)

const (
	SeriesCheckName = "query/series"
)

func NewSeriesCheck(name, uri string, timeout time.Duration, severity Severity, ignoreRR bool, recordingRules *[]*parser.RecordingRule) SeriesCheck {
	return SeriesCheck{ignoreRR: ignoreRR, name: name, uri: uri, timeout: timeout, severity: severity, recordingRules: recordingRules}
}

type SeriesCheck struct {
	name           string
	uri            string
	timeout        time.Duration
	severity       Severity
	recordingRules *[]*parser.RecordingRule
	ignoreRR       bool
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

outerloop:
	for _, selector := range getSelectors(expr.Query) {
		if _, ok := done[selector.String()]; ok {
			continue
		}
		done[selector.String()] = true

		countProblems := c.countSeries(expr, selector)
		if c.ignoreRR && c.recordingRules != nil && len(*c.recordingRules) > 0 {
			for _, rr := range *c.recordingRules {
				if selectorSubsetOfRR(rr, &selector) {
					continue outerloop
				}
			}

		}
		problems = append(problems, countProblems...)
	}

	return
}

// selectorSubsetOfRR returns whether a given selector is a subset of the recording rule.
// For example, foo{a="b"} selector is a subset of recording rule
// record: foo
// labels: a: b
//         c: d
func selectorSubsetOfRR(rr *parser.RecordingRule, sel *promParser.VectorSelector) bool {
	rrMatchers := make(map[string]string)
	if rr.Labels != nil {
		for _, m := range rr.Labels.Items {
			rrMatchers[m.Key.Value] = m.Value.Value
		}
	}
	rrMatchers["__name__"] = rr.Record.Value.Value

	for _, m := range sel.LabelMatchers {
		rVal, ok := rrMatchers[m.Name]
		if !ok {
			return false
		}
		if rVal != m.Value {
			return false
		}
	}

	return true
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
			// retry selector without labels
			s := promParser.VectorSelector{
				Name: selector.Name,
				LabelMatchers: []*labels.Matcher{
					{Name: labels.MetricName, Value: selector.Name},
				},
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
