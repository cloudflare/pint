package checks

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/exp/slices"

	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/parser/utils"
	"github.com/cloudflare/pint/internal/promapi"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	promParser "github.com/prometheus/prometheus/promql/parser"
)

// FIXME: add native histograms

const (
	RateCheckName    = "promql/rate"
	RateCheckDetails = `Using [rate](https://prometheus.io/docs/prometheus/latest/querying/functions/#rate) and [irate](https://prometheus.io/docs/prometheus/latest/querying/functions/#irate) function comes with a few requirements:

- The metric you calculate (i)rate from must be a counter or native histograms.
- The time window of the (i)rate function must have at least 2 samples.

The type of your metric is defined by the application that exports that metric.
The number of samples depends on how often your application is being scraped by Prometheus.
Each scrape produces a sample, so if your application is scrape every minute then the minimal time window you can use is two minutes.`
)

func NewRateCheck(prom *promapi.FailoverGroup) RateCheck {
	return RateCheck{prom: prom, minIntervals: 2}
}

type RateCheck struct {
	prom         *promapi.FailoverGroup
	minIntervals int
}

func (c RateCheck) Meta() CheckMeta {
	return CheckMeta{IsOnline: true}
}

func (c RateCheck) String() string {
	return fmt.Sprintf("%s(%s)", RateCheckName, c.prom.Name())
}

func (c RateCheck) Reporter() string {
	return RateCheckName
}

func (c RateCheck) Check(ctx context.Context, _ string, rule parser.Rule, entries []discovery.Entry) (problems []Problem) {
	expr := rule.Expr()

	if expr.SyntaxError != nil {
		return problems
	}

	cfg, err := c.prom.Config(ctx)
	if err != nil {
		text, severity := textAndSeverityFromError(err, c.Reporter(), c.prom.Name(), Bug)
		problems = append(problems, Problem{
			Fragment: expr.Value.Value,
			Lines:    expr.Lines(),
			Reporter: c.Reporter(),
			Text:     text,
			Severity: severity,
		})
		return problems
	}

	done := &completedList{}
	for _, problem := range c.checkNode(ctx, expr.Query, entries, cfg, done) {
		problems = append(problems, Problem{
			Fragment: problem.expr,
			Lines:    expr.Lines(),
			Reporter: c.Reporter(),
			Text:     problem.text,
			Details:  problem.details,
			Severity: problem.severity,
		})
	}

	return problems
}

func (c RateCheck) checkNode(ctx context.Context, node *parser.PromQLNode, entries []discovery.Entry, cfg *promapi.ConfigResult, done *completedList) (problems []exprProblem) {
	if n, ok := node.Node.(*promParser.Call); ok && (n.Func.Name == "rate" || n.Func.Name == "irate" || n.Func.Name == "deriv") {
		for _, arg := range n.Args {
			m, ok := arg.(*promParser.MatrixSelector)
			if !ok {
				continue
			}
			if m.Range < cfg.Config.Global.ScrapeInterval*time.Duration(c.minIntervals) {
				p := exprProblem{
					expr: node.Expr,
					text: fmt.Sprintf("Duration for `%s()` must be at least %d x scrape_interval, %s is using `%s` scrape_interval.",
						n.Func.Name, c.minIntervals, promText(c.prom.Name(), cfg.URI), output.HumanizeDuration(cfg.Config.Global.ScrapeInterval)),
					details:  RateCheckDetails,
					severity: Bug,
				}
				problems = append(problems, p)
			}
			if n.Func.Name == "deriv" {
				continue
			}
			if s, ok := m.VectorSelector.(*promParser.VectorSelector); ok {
				if slices.Contains(done.values, s.Name) {
					continue
				}
				done.values = append(done.values, s.Name)
				metadata, err := c.prom.Metadata(ctx, s.Name)
				if err != nil {
					text, severity := textAndSeverityFromError(err, c.Reporter(), c.prom.Name(), Bug)
					problems = append(problems, exprProblem{
						expr:     s.Name,
						text:     text,
						severity: severity,
					})
					continue
				}
				for _, m := range metadata.Metadata {
					if m.Type != v1.MetricTypeCounter && m.Type != v1.MetricTypeUnknown {
						problems = append(problems, exprProblem{
							expr: s.Name,
							text: fmt.Sprintf("`%s()` should only be used with counters but `%s` is a %s according to metrics metadata from %s.",
								n.Func.Name, s.Name, m.Type, promText(c.prom.Name(), metadata.URI)),
							details:  RateCheckDetails,
							severity: Bug,
						})
					}
				}

				for _, e := range entries {
					if e.PathError != nil {
						continue
					}
					if e.Rule.Error.Err != nil {
						continue
					}
					if e.Rule.RecordingRule != nil && e.Rule.RecordingRule.Expr.SyntaxError == nil && e.Rule.RecordingRule.Record.Value.Value == s.Name {
						for _, sm := range utils.HasOuterSum(e.Rule.RecordingRule.Expr.Query) {
							if sv, ok := sm.Expr.(*promParser.VectorSelector); ok {
								metadata, err := c.prom.Metadata(ctx, sv.Name)
								if err != nil {
									text, severity := textAndSeverityFromError(err, c.Reporter(), c.prom.Name(), Bug)
									problems = append(problems, exprProblem{
										expr:     sv.Name,
										text:     text,
										severity: severity,
									})
									continue
								}
								for _, m := range metadata.Metadata {
									if m.Type == v1.MetricTypeCounter {
										problems = append(problems, exprProblem{
											expr: node.Expr,
											text: fmt.Sprintf("`rate(sum(counter))` chain detected, `%s` is called here on results of `%s`, calling `rate()` on `sum()` results will return bogus results, always `sum(rate(counter))`, never `rate(sum(counter))`.",
												node.Expr, sm),
											severity: Bug,
										})
									}
								}
							}
						}
					}
				}
			}
		}
	}

	for _, child := range node.Children {
		problems = append(problems, c.checkNode(ctx, child, entries, cfg, done)...)
	}

	return problems
}

type completedList struct {
	values []string
}
