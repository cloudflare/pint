package checks

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"

	"github.com/prometheus/prometheus/model/labels"
	promParser "github.com/prometheus/prometheus/promql/parser"

	"github.com/prometheus/prometheus/promql/parser/posrange"

	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/parser/utils"
	"github.com/cloudflare/pint/internal/promapi"
)

const (
	CostCheckName       = "query/cost"
	BytesPerSampleQuery = "avg(avg_over_time(go_memstats_alloc_bytes[2h]) / avg_over_time(prometheus_tsdb_head_series[2h]))"
)

func NewCostCheck(prom *promapi.FailoverGroup, maxSeries, maxTotalSamples, maxPeakSamples int, maxEvaluationDuration time.Duration, comment string, severity Severity) CostCheck {
	return CostCheck{
		prom:                  prom,
		maxSeries:             maxSeries,
		maxTotalSamples:       maxTotalSamples,
		maxPeakSamples:        maxPeakSamples,
		maxEvaluationDuration: maxEvaluationDuration,
		comment:               comment,
		severity:              severity,
	}
}

type CostCheck struct {
	prom                  *promapi.FailoverGroup
	comment               string
	maxSeries             int
	maxTotalSamples       int
	maxPeakSamples        int
	maxEvaluationDuration time.Duration
	severity              Severity
}

func (c CostCheck) Meta() CheckMeta {
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

func (c CostCheck) String() string {
	if c.maxSeries > 0 {
		return fmt.Sprintf("%s(%s:%d)", CostCheckName, c.prom.Name(), c.maxSeries)
	}
	return fmt.Sprintf("%s(%s)", CostCheckName, c.prom.Name())
}

func (c CostCheck) Reporter() string {
	return CostCheckName
}

func (c CostCheck) Check(ctx context.Context, entry discovery.Entry, entries []discovery.Entry) (problems []Problem) {
	expr := entry.Rule.Expr()

	if expr.SyntaxError != nil {
		return problems
	}

	qr, series, err := c.getQueryCost(ctx, expr.Value.Value)
	if err != nil {
		problems = append(problems, problemFromError(err, entry.Rule, c.Reporter(), c.prom.Name(), Bug))
		return problems
	}

	var estimate string
	if series > 0 {
		result, err := c.prom.Query(ctx, BytesPerSampleQuery)
		if err == nil {
			for _, s := range result.Series {
				estimate = fmt.Sprintf(" with %s estimated memory usage", output.HumanizeBytes(int(s.Value*float64(series))))
				break
			}
		}
	}

	problems = append(problems, c.suggestRecordingRules(ctx, expr, entry, entries, qr.Stats, series)...)

	if c.maxSeries > 0 && series > c.maxSeries {
		problems = append(problems, Problem{
			Anchor:   AnchorAfter,
			Lines:    expr.Value.Pos.Lines(),
			Reporter: c.Reporter(),
			Summary:  "query is too expensive",
			Details:  maybeComment(c.comment),
			Severity: c.severity,
			Diagnostics: []diags.Diagnostic{
				{
					Message:     fmt.Sprintf("%s returned %d result(s)%s, maximum allowed series is %d.", promText(c.prom.Name(), qr.URI), series, estimate, c.maxSeries),
					Pos:         expr.Value.Pos,
					FirstColumn: 1,
					LastColumn:  len(expr.Value.Value),
					Kind:        diags.Issue,
				},
			},
		})
		return problems
	}

	if c.maxTotalSamples > 0 && qr.Stats.Samples.TotalQueryableSamples > c.maxTotalSamples {
		problems = append(problems, Problem{
			Anchor:   AnchorAfter,
			Lines:    expr.Value.Pos.Lines(),
			Reporter: c.Reporter(),
			Summary:  "query is too expensive",
			Details:  maybeComment(c.comment),
			Severity: c.severity,
			Diagnostics: []diags.Diagnostic{
				{
					Message:     fmt.Sprintf("%s queried %d samples in total when executing this query, which is more than the configured limit of %d.", promText(c.prom.Name(), qr.URI), qr.Stats.Samples.TotalQueryableSamples, c.maxTotalSamples),
					Pos:         expr.Value.Pos,
					FirstColumn: 1,
					LastColumn:  len(expr.Value.Value),
					Kind:        diags.Issue,
				},
			},
		})
		return problems
	}

	if c.maxPeakSamples > 0 && qr.Stats.Samples.PeakSamples > c.maxPeakSamples {
		problems = append(problems, Problem{
			Anchor:   AnchorAfter,
			Lines:    expr.Value.Pos.Lines(),
			Reporter: c.Reporter(),
			Summary:  "query is too expensive",
			Details:  maybeComment(c.comment),
			Severity: c.severity,
			Diagnostics: []diags.Diagnostic{
				{
					Message:     fmt.Sprintf("%s queried %d peak samples when executing this query, which is more than the configured limit of %d.", promText(c.prom.Name(), qr.URI), qr.Stats.Samples.PeakSamples, c.maxPeakSamples),
					Pos:         expr.Value.Pos,
					FirstColumn: 1,
					LastColumn:  len(expr.Value.Value),
					Kind:        diags.Issue,
				},
			},
		})
		return problems
	}

	evalDur := c.statToDuration(qr.Stats.Timings.EvalTotalTime)
	if c.maxEvaluationDuration > 0 && evalDur > c.maxEvaluationDuration {
		problems = append(problems, Problem{
			Anchor:   AnchorAfter,
			Lines:    expr.Value.Pos.Lines(),
			Reporter: c.Reporter(),
			Summary:  "query is too expensive",
			Details:  maybeComment(c.comment),
			Severity: c.severity,
			Diagnostics: []diags.Diagnostic{
				{
					Message:     fmt.Sprintf("%s took %s when executing this query, which is more than the configured limit of %s.", promText(c.prom.Name(), qr.URI), output.HumanizeDuration(evalDur), output.HumanizeDuration(c.maxEvaluationDuration)),
					Pos:         expr.Value.Pos,
					FirstColumn: 1,
					LastColumn:  len(expr.Value.Value),
					Kind:        diags.Issue,
				},
			},
		})
		return problems
	}

	if series > 0 && c.maxSeries == 0 && c.maxTotalSamples == 0 && c.maxPeakSamples == 0 && c.maxEvaluationDuration == 0 {
		problems = append(problems, Problem{
			Anchor:   AnchorAfter,
			Lines:    expr.Value.Pos.Lines(),
			Reporter: c.Reporter(),
			Summary:  "query cost estimate",
			Details:  maybeComment(c.comment),
			Severity: Information,
			Diagnostics: []diags.Diagnostic{
				{
					Message:     fmt.Sprintf("%s returned %d result(s)%s.", promText(c.prom.Name(), qr.URI), series, estimate),
					Pos:         expr.Value.Pos,
					FirstColumn: 1,
					LastColumn:  len(expr.Value.Value),
					Kind:        diags.Issue,
				},
			},
		})
	}

	return problems
}

func (c CostCheck) statToDuration(f float64) time.Duration {
	return time.Duration(f * float64(time.Second))
}

func (c CostCheck) getQueryCost(ctx context.Context, expr string) (*promapi.QueryResult, int, error) {
	qr, err := c.prom.Query(ctx, fmt.Sprintf("count(%s)", expr))
	if err != nil {
		return nil, 0, err
	}

	var series int
	for _, s := range qr.Series {
		series += int(s.Value)
	}
	return qr, series, nil
}

func (c CostCheck) suggestRecordingRules(
	ctx context.Context,
	expr parser.PromQLExpr,
	entry discovery.Entry, entries []discovery.Entry,
	beforeStats promapi.QueryStats, beforeSeries int,
) (problems []Problem) {
	src := utils.LabelsSource(expr.Value.Value, expr.Query.Expr)

	for _, other := range entries {
		if ignoreOtherEntry(entry, other, c.prom) {
			continue
		}
		if other.Rule.RecordingRule == nil {
			continue
		}

		otherSrc := utils.LabelsSource(other.Rule.RecordingRule.Expr.Value.Value, other.Rule.RecordingRule.Expr.Query.Expr)
		if len(otherSrc) > 1 {
			continue
		}
		for _, s := range src {
			s.WalkSources(func(s utils.Source, j *utils.Join) {
				for _, os := range otherSrc {
					op, extra, exact, ok := c.isSuggestionFor(s, os, j)
					if !ok {
						continue
					}

					var prefix string
					if exact {
						prefix = "There is a recording rule that already stores the result of this query, use it here to speed up this query."
					} else {
						prefix = "There is a recording rule that stores result of a query that might work the same, you can try to use it here to speed up this query."
					}

					sq := c.rewriteRuleFragment(expr.Value.Value, op.PositionRange(), other.Rule.RecordingRule.Record.Value+extra)
					var suffix strings.Builder
					qr, afterSeries, err := c.getQueryCost(ctx, sq)
					if err == nil {
						if qr.Stats.Samples.TotalQueryableSamples >= beforeStats.Samples.TotalQueryableSamples &&
							qr.Stats.Samples.PeakSamples >= beforeStats.Samples.PeakSamples {
							// Suggestion doesn't reduce the number of samples queried, ignore it.
							continue
						}
						if beforeSeries != afterSeries {
							// Got different number of series returned, using suggestion is unsafe.
							continue
						}
						suffix.WriteRune('\n')
						suffix.WriteString("Using `")
						suffix.WriteString(other.Rule.RecordingRule.Record.Value)
						suffix.WriteString("` rule would speed up this query:\n\n")
						suffix.WriteString("- Total queried samples would be ")
						suffix.WriteString(c.diffStatsInt(beforeStats.Samples.TotalQueryableSamples, qr.Stats.Samples.TotalQueryableSamples))
						suffix.WriteRune('\n')
						suffix.WriteString("- Peak queried samples would be ")
						suffix.WriteString(c.diffStatsInt(beforeStats.Samples.PeakSamples, qr.Stats.Samples.PeakSamples))
						suffix.WriteRune('\n')
						suffix.WriteString("- Query evaluation time would be ")
						suffix.WriteString(c.diffStatsDuration(beforeStats.Timings.EvalTotalTime, qr.Stats.Timings.EvalTotalTime))
						suffix.WriteRune('\n')
						suffix.WriteRune('\n')
						suffix.WriteString("To get results for both original and suggested query click below:\n\n")
						suffix.WriteString(fmt.Sprintf("- [Original query](%s/graph?g0.expr=%s&g0.tab=table)\n",
							qr.URI, url.QueryEscape(expr.Value.Value)))
						suffix.WriteString(fmt.Sprintf("- [Suggested query](%s/graph?g0.expr=%s&g0.tab=table)\n",
							qr.URI, url.QueryEscape(sq)))
					}

					problems = append(problems, Problem{
						Anchor:   AnchorAfter,
						Lines:    expr.Value.Pos.Lines(),
						Reporter: c.Reporter(),
						Summary:  "query could use a recording rule",
						Details:  prefix + suffix.String(),
						Severity: Information,
						Diagnostics: []diags.Diagnostic{
							{
								Message:     fmt.Sprintf("Use `%s` here instead to speed up the query.", other.Rule.RecordingRule.Record.Value),
								Pos:         expr.Value.Pos,
								FirstColumn: int(op.PositionRange().Start) + 1,
								LastColumn:  int(op.PositionRange().End),
								Kind:        diags.Issue,
							},
						},
					})
				}
			})
		}
	}
	return problems
}

func (c CostCheck) rewriteRuleFragment(expr string, fragment posrange.PositionRange, replacement string) string {
	var buf strings.Builder
	if fragment.Start > 0 {
		buf.WriteString(expr[:int(fragment.Start)])
	}
	buf.WriteString(replacement)
	if int(fragment.End)+1 < len(expr) {
		buf.WriteString(expr[int(fragment.End):])
	}
	return buf.String()
}

func (c CostCheck) diffStatsInt(a, b int) string {
	delta := (float64(b-a) / float64(a)) * 100
	if delta == 0 || math.IsNaN(delta) {
		return fmt.Sprintf("%d (no change)", a)
	}
	return fmt.Sprintf("%d instead of %d (%0.2f%%)", b, a, delta)
}

func (c CostCheck) diffStatsDuration(a, b float64) string {
	delta := ((b - a) / a) * 100
	if delta == 0 || math.IsNaN(delta) {
		return output.HumanizeDuration(c.statToDuration(a)) + " (no change)"
	}
	return fmt.Sprintf("%s instead of %s (%0.2f%%)",
		output.HumanizeDuration(c.statToDuration(b)),
		output.HumanizeDuration(c.statToDuration(a)),
		delta)
}

func (c CostCheck) isSuggestionFor(src, potential utils.Source, join *utils.Join) (promParser.Node, string, bool, bool) {
	if potential.Type != utils.FuncSource && potential.Type != utils.AggregateSource {
		return nil, "", false, false
	}

	if potential.Returns != src.Returns {
		return nil, "", false, false
	}

	// We're only looking at potential source that have a vector selector.
	if _, ok := utils.MostOuterOperation[*promParser.VectorSelector](potential); !ok {
		return nil, "", false, false
	}

	if join != nil {
		// Check if potential can have all the labels we use in a join.
		for _, name := range join.On {
			if src.CanHaveLabel(name) && !potential.CanHaveLabel(name) {
				return nil, "", false, false
			}
		}
	}

	// Check if we part of the source query can be substitute with a recording rule
	// that uses the exact same query.
	oop := potential.Operations[len(potential.Operations)-1]
	for _, op := range src.Operations {
		if op.Node.Pretty(0) == oop.Node.Pretty(0) {
			return op.Node, "", true, true
		}
	}

	// If not we do a fuzzy search where we look for recording rules of similar "shape":
	// - Same operations (normalize rate/irate):
	//   * sum -> rate -> selector
	//   * rate -> selector
	// - On same the selector.
	// - With the same labels possible. All? Only from joins?

	// Src must have all operations potential does, so skip checks if potential is shorter.
	if len(potential.Operations) > len(src.Operations) {
		return nil, "", false, false
	}

	for i := len(src.Operations); i > 0; i-- {
		ops := src.Operations[:i]
		if c.equalOperations(ops, potential.Operations) {
			if c.metricName(ops) != c.metricName(potential.Operations) {
				goto NEXT
			}

			lms := c.selectorLabels(ops)
			for _, lm := range lms {
				if lm.Name == labels.MetricName {
					continue
				}
				if !potential.CanHaveLabel(lm.Name) {
					goto NEXT
				}
			}
			var extra string
			if len(lms) > 0 {
				var buf strings.Builder
				var added int
				for _, lm := range lms {
					if lm.Name == labels.MetricName {
						continue
					}
					if added == 0 {
						buf.WriteRune('{')
					} else if added > 0 {
						buf.WriteString(", ")
					}
					buf.WriteString(lm.String())
					added++
				}
				if added > 0 {
					buf.WriteRune('}')
				}
				extra = buf.String()
			}
			return src.Operations[i-1].Node, extra, false, true
		}
	NEXT:
	}

	return nil, "", false, false
}

func (c CostCheck) equalOperations(a, b utils.SourceOperations) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if c.normalizeFuncName(a[i].Operation) != c.normalizeFuncName(b[i].Operation) {
			return false
		}
	}
	return true
}

func (c CostCheck) normalizeFuncName(s string) string {
	switch s {
	case "irate":
		return "rate"
	default:
		return s
	}
}

func (c CostCheck) metricName(ops utils.SourceOperations) string {
	for _, op := range ops {
		if vs, ok := op.Node.(*promParser.VectorSelector); ok {
			for _, lm := range vs.LabelMatchers {
				if lm.Type == labels.MatchEqual && lm.Name == labels.MetricName {
					return lm.Value
				}
			}
		}
	}
	return ""
}

func (c CostCheck) selectorLabels(ops utils.SourceOperations) (lms []*labels.Matcher) {
	for i := len(ops) - 1; i >= 0; i-- {
		if vs, ok := ops[i].Node.(*promParser.VectorSelector); ok {
			lms = vs.LabelMatchers
			break
		}
	}
	return lms
}
