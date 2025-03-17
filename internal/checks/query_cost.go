package checks

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/parser"
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

func (c CostCheck) Check(ctx context.Context, _ discovery.Path, rule parser.Rule, _ []discovery.Entry) (problems []Problem) {
	expr := rule.Expr()

	if expr.SyntaxError != nil {
		return problems
	}

	query := fmt.Sprintf("count(%s)", expr.Value.Value)
	qr, err := c.prom.Query(ctx, query)
	if err != nil {
		problems = append(problems, problemFromError(err, rule, c.Reporter(), c.prom.Name(), Bug))
		return problems
	}

	var series int
	for _, s := range qr.Series {
		series += int(s.Value)
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
				},
			},
		})
		return problems
	}

	evalDur := time.Duration(qr.Stats.Timings.EvalTotalTime * float64(time.Second))
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
				},
			},
		})
	}

	return problems
}
