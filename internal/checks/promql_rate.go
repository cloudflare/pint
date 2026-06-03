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
	"github.com/cloudflare/pint/internal/parser/source"
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

var allowedRateTypes = []v1.MetricType{
	v1.MetricTypeCounter,
	v1.MetricTypeHistogram,
	v1.MetricTypeSummary,
	v1.MetricTypeUnknown,
}

func NewRateCheck(prom *promapi.FailoverGroup) RateCheck {
	return RateCheck{
		prom:         prom,
		minIntervals: 2,
		instance:     fmt.Sprintf("%s(%s)", RateCheckName, prom.Name()),
	}
}

type RateCheck struct {
	prom         *promapi.FailoverGroup
	instance     string
	minIntervals int
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

func (c RateCheck) Check(ctx context.Context, entry *discovery.Entry, entries []*discovery.Entry) (problems []Problem) {
	expr := entry.Rule.Expr()

	if expr.SyntaxError() != nil {
		return problems
	}

	cfg, err := c.prom.Config(ctx, 0).Wait()
	if err != nil {
		if errors.Is(err, promapi.ErrUnsupported) {
			c.prom.DisableCheck(promapi.APIPathConfig, c.Reporter())
			return problems
		}
		problems = append(problems, problemFromError(err, entry.Rule, c.Reporter(), c.prom.Name(), Bug))
		return problems
	}

	metadataNames := c.collectRateMetricNames(expr, entries)
	pending := make(map[string]*promapi.Request[*promapi.MetadataResult], len(metadataNames))
	for _, name := range metadataNames {
		pending[name] = c.prom.Metadata(ctx, name)
	}

	problems = append(problems, c.checkSources(entry.Rule, expr, entries, cfg, pending)...)
	return problems
}

func (c RateCheck) collectRateMetricNames(expr *parser.PromQLExpr, entries []*discovery.Entry) []string {
	seen := map[string]struct{}{}
	var names []string

	add := func(name string) {
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}

	for _, src := range expr.Source() {
		src.WalkSources(func(s *source.Source, _ *source.Join, _ *source.Unless) {
			call, found := findRateCall(s)
			// deriv() doesn't require a counter, skip metadata collection for it.
			if !found || call.Func.Name == "deriv" {
				return
			}
			// Skip subqueries like rate(foo[5m:1m]) — only collect names from range selectors.
			if _, ok := source.MostOuterOperation[*promParser.MatrixSelector](s); !ok {
				return
			}
			// We confirmed MatrixSelector above, which always wraps a VectorSelector.
			vs, _ := source.MostOuterOperation[*promParser.VectorSelector](s)
			add(vs.Name)
			for _, e := range entries {
				if e.PathError != nil {
					continue
				}
				if e.Rule.Error.Err != nil {
					continue
				}
				if e.Rule.RecordingRule != nil && e.Rule.RecordingRule.Expr.SyntaxError() == nil && e.Rule.RecordingRule.Record.Value == vs.Name {
					for _, rsrc := range e.Rule.RecordingRule.Expr.Source() {
						if rsrc.Type != source.AggregateSource {
							continue
						}
						if rvs, ok := source.MostOuterOperation[*promParser.VectorSelector](rsrc); ok {
							add(rvs.Name)
						}
					}
				}
			}
		})
	}

	return names
}

func (c RateCheck) checkSources(
	rule parser.Rule,
	expr *parser.PromQLExpr,
	entries []*discovery.Entry,
	cfg *promapi.ConfigResult,
	pending map[string]*promapi.Request[*promapi.MetadataResult],
) (problems []Problem) {
	done := map[string]struct{}{}

	for _, src := range expr.Source() {
		src.WalkSources(func(s *source.Source, _ *source.Join, _ *source.Unless) {
			call, ok := findRateCall(s)
			if !ok {
				return
			}

			m, ok := source.MostOuterOperation[*promParser.MatrixSelector](s)
			if !ok {
				return
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
							Message: fmt.Sprintf(
								"Duration for `%s()` must be at least %d x scrape_interval, %s is using `%s` scrape_interval.",
								call.Func.Name, c.minIntervals,
								promText(c.prom.Name(), cfg.URI),
								output.HumanizeDuration(cfg.Config.Global.ScrapeInterval),
							),
							Pos:         expr.Value.Pos,
							Expr:        expr.Query().Expr,
							FirstColumn: int(call.PosRange.Start) + 1,
							LastColumn:  int(call.PosRange.End),
							Kind:        diags.Issue,
						},
					},
				})
			}

			// deriv() works on gauges, no need to check metric type via metadata.
			if call.Func.Name == "deriv" {
				return
			}

			// MatrixSelector always wraps a VectorSelector, so this can't fail.
			vs, _ := source.MostOuterOperation[*promParser.VectorSelector](s)

			if _, ok := done[vs.Name]; ok {
				return
			}
			done[vs.Name] = struct{}{}

			metadata, err := pending[vs.Name].Wait()
			if err != nil {
				if errors.Is(err, promapi.ErrUnsupported) {
					return
				}
				problems = append(problems, problemFromError(err, rule, c.Reporter(), c.prom.Name(), Bug))
				return
			}
			for _, md := range metadata.Metadata {
				if !slices.Contains(allowedRateTypes, md.Type) {
					problems = append(problems, Problem{
						Anchor:   AnchorAfter,
						Lines:    expr.Value.Pos.Lines(),
						Reporter: c.Reporter(),
						Summary:  "counter based function called on a non-counter",
						Details:  RateCheckDetails,
						Severity: Bug,
						Diagnostics: []diags.Diagnostic{
							{
								Message: fmt.Sprintf(
									"`%s()` should only be used with counters but `%s` is a %s according to metrics metadata from %s.",
									call.Func.Name, vs.Name, md.Type,
									promText(c.prom.Name(), metadata.URI),
								),
								Pos:         expr.Value.Pos,
								Expr:        expr.Query().Expr,
								FirstColumn: int(call.PosRange.Start) + 1,
								LastColumn:  int(call.PosRange.End),
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
				if e.Rule.RecordingRule != nil && e.Rule.RecordingRule.Expr.SyntaxError() == nil && e.Rule.RecordingRule.Record.Value == vs.Name {
					for _, rsrc := range e.Rule.RecordingRule.Expr.Source() {
						if rsrc.Type != source.AggregateSource {
							continue
						}
						if rvs, ok := source.MostOuterOperation[*promParser.VectorSelector](rsrc); ok {
							metadata, err := pending[rvs.Name].Wait()
							if err != nil {
								if errors.Is(err, promapi.ErrUnsupported) {
									continue
								}
								problems = append(problems, problemFromError(err, rule, c.Reporter(), c.prom.Name(), Bug))
								continue
							}
							canReport := true
							severity := Warning
							for _, md := range metadata.Metadata {
								// nolint:exhaustive
								switch md.Type {
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
									rsrc.Operation(), rsrc.Operation(),
								),
								Severity: severity,
								Diagnostics: []diags.Diagnostic{
									{
										Message: fmt.Sprintf(
											"`rate(%s(counter))` chain detected, `%s` is called here on results of `%s(%s)`.",
											rsrc.Operation(), call, rsrc.Operation(), rvs,
										),
										Pos:         expr.Value.Pos,
										Expr:        expr.Query().Expr,
										FirstColumn: int(rsrc.Position.Start) + 1,
										LastColumn:  int(rsrc.Position.End),
										Kind:        diags.Issue,
									},
								},
							})
						}
					}
				}
			}
		})
	}

	return problems
}

func findRateCall(s *source.Source) (*promParser.Call, bool) {
	for _, op := range s.Operations {
		call, ok := op.Node.(*promParser.Call)
		if !ok {
			continue
		}
		switch call.Func.Name {
		case "rate", "irate", "deriv":
			return call, true
		}
	}
	return nil, false
}
