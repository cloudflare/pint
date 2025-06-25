package checks

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/cloudflare/pint/internal/diags"
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
	return RateCheck{
		prom:         prom,
		minIntervals: 2,
		instance:     fmt.Sprintf("%s(%s)", RateCheckName, prom.Name()),
	}
}

type RateCheck struct {
	prom         *promapi.FailoverGroup
	minIntervals int
	instance     string
}

func (c RateCheck) Meta() CheckMeta {
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

func (c RateCheck) String() string {
	return c.instance
}

func (c RateCheck) Reporter() string {
	return RateCheckName
}

func (c RateCheck) Check(ctx context.Context, entry discovery.Entry, entries []discovery.Entry) (problems []Problem) {
	expr := entry.Rule.Expr()

	if expr.SyntaxError != nil {
		return problems
	}

	cfg, err := c.prom.Config(ctx, 0)
	if err != nil {
		if errors.Is(err, promapi.ErrUnsupported) {
			c.prom.DisableCheck(promapi.APIPathConfig, c.Reporter())
			return problems
		}
		problems = append(problems, problemFromError(err, entry.Rule, c.Reporter(), c.prom.Name(), Bug))
		return problems
	}

	problems = append(problems, c.checkNode(ctx, entry.Rule, expr, expr.Query, entries, cfg, &completedList{values: nil})...)
	return problems
}

func (c RateCheck) checkNode(ctx context.Context, rule parser.Rule, expr parser.PromQLExpr, node *parser.PromQLNode, entries []discovery.Entry, cfg *promapi.ConfigResult, done *completedList) (problems []Problem) {
	if n, ok := node.Expr.(*promParser.Call); ok && (n.Func.Name == "rate" || n.Func.Name == "irate" || n.Func.Name == "deriv") {
		for _, arg := range n.Args {
			m, ok := arg.(*promParser.MatrixSelector)
			if !ok {
				continue
			}
			if m.Range < cfg.Config.Global.ScrapeInterval*time.Duration(c.minIntervals) {
				problems = append(problems, Problem{
					Anchor:   AnchorAfter,
					Lines:    expr.Value.Pos.Lines(),
					Reporter: c.Reporter(),
					Summary:  "duration too small",
					Details:  RateCheckDetails,
					Severity: Bug,
					Diagnostics: []diags.Diagnostic{
						{
							Message: fmt.Sprintf("Duration for `%s()` must be at least %d x scrape_interval, %s is using `%s` scrape_interval.",
								n.Func.Name, c.minIntervals, promText(c.prom.Name(), cfg.URI), output.HumanizeDuration(cfg.Config.Global.ScrapeInterval)),
							Pos:         expr.Value.Pos,
							FirstColumn: int(n.PosRange.Start) + 1,
							LastColumn:  int(n.PosRange.End),
							Kind:        diags.Issue,
						},
					},
				})
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
					if errors.Is(err, promapi.ErrUnsupported) {
						continue
					}
					problems = append(problems, problemFromError(err, rule, c.Reporter(), c.prom.Name(), Bug))
					continue
				}
				for _, m := range metadata.Metadata {
					if m.Type != v1.MetricTypeCounter && m.Type != v1.MetricTypeUnknown {
						problems = append(problems, Problem{
							Anchor:   AnchorAfter,
							Lines:    expr.Value.Pos.Lines(),
							Reporter: c.Reporter(),
							Summary:  "counter based function called on a non-counter",
							Details:  RateCheckDetails,
							Severity: Bug,
							Diagnostics: []diags.Diagnostic{
								{
									Message: fmt.Sprintf("`%s()` should only be used with counters but `%s` is a %s according to metrics metadata from %s.",
										n.Func.Name, s.Name, m.Type, promText(c.prom.Name(), metadata.URI)),
									Pos:         expr.Value.Pos,
									FirstColumn: int(n.PosRange.Start) + 1,
									LastColumn:  int(n.PosRange.End),
									Kind:        diags.Issue,
								},
							},
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
					if e.Rule.RecordingRule != nil && e.Rule.RecordingRule.Expr.SyntaxError == nil && e.Rule.RecordingRule.Record.Value == s.Name {
						for _, src := range utils.LabelsSource(e.Rule.RecordingRule.Expr.Value.Value, e.Rule.RecordingRule.Expr.Query.Expr) {
							if src.Type != utils.AggregateSource {
								continue
							}
							if vs, ok := utils.MostOuterOperation[*promParser.VectorSelector](src); ok {
								metadata, err := c.prom.Metadata(ctx, vs.Name)
								if err != nil {
									if errors.Is(err, promapi.ErrUnsupported) {
										continue
									}
									problems = append(problems, problemFromError(err, rule, c.Reporter(), c.prom.Name(), Bug))
									continue
								}
								canReport := true
								severity := Warning
								for _, m := range metadata.Metadata {
									// nolint:exhaustive
									switch m.Type {
									case v1.MetricTypeCounter:
										severity = Bug
									default:
										canReport = false
									}
								}
								if !canReport {
									continue
								}
								problems = append(problems, Problem{
									Anchor:   AnchorAfter,
									Lines:    expr.Value.Pos.Lines(),
									Reporter: c.Reporter(),
									Summary:  "chained rate call",
									Details: fmt.Sprintf(
										"You can only calculate `rate()` directly from a counter metric. "+
											"Calling `rate()` on `%s()` results will return bogus results because `%s()` will hide information on when each counter resets. "+
											"You must first calculate `rate()` before calling any aggregation function. Always `sum(rate(counter))`, never `rate(sum(counter))`",
										src.Operation(), src.Operation()),
									Severity: severity,
									Diagnostics: []diags.Diagnostic{
										{
											Message: fmt.Sprintf("`rate(%s(counter))` chain detected, `%s` is called here on results of `%s(%s)`.",
												src.Operation(), node.Expr, src.Operation(), vs),
											Pos:         expr.Value.Pos,
											FirstColumn: int(src.Position.Start) + 1,
											LastColumn:  int(src.Position.End),
											Kind:        diags.Issue,
										},
									},
								})
							}
						}
					}
				}
			}
		}
	}

	for _, child := range node.Children {
		problems = append(problems, c.checkNode(ctx, rule, expr, child, entries, cfg, done)...)
	}

	return problems
}

type completedList struct {
	values []string
}
