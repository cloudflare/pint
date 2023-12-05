package checks

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
)

const (
	CostCheckName       = "query/cost"
	BytesPerSampleQuery = "avg(avg_over_time(go_memstats_alloc_bytes[2h]) / avg_over_time(prometheus_tsdb_head_series[2h]))"
)

func NewCostCheck(prom *promapi.FailoverGroup, maxSeries, maxTotalSamples, maxPeakSamples int, maxEvaluationDuration time.Duration, severity Severity) CostCheck {
	return CostCheck{
		prom:                  prom,
		maxSeries:             maxSeries,
		maxTotalSamples:       maxTotalSamples,
		maxPeakSamples:        maxPeakSamples,
		maxEvaluationDuration: maxEvaluationDuration,
		severity:              severity,
	}
}

type CostCheck struct {
	prom                  *promapi.FailoverGroup
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
		IsOnline: true,
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

func (c CostCheck) Check(ctx context.Context, _ string, rule parser.Rule, _ []discovery.Entry) (problems []Problem) {
	expr := rule.Expr()

	if expr.SyntaxError != nil {
		return problems
	}

	query := fmt.Sprintf("count(%s)", expr.Value.Value)
	qr, err := c.prom.Query(ctx, query)
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

	var above string
	severity := Information
	if c.maxSeries > 0 && series > c.maxSeries {
		severity = c.severity
		above = fmt.Sprintf(", maximum allowed series is %d.", c.maxSeries)
	} else {
		estimate += "."
	}

	problems = append(problems, Problem{
		Fragment: expr.Value.Value,
		Lines:    expr.Lines(),
		Reporter: c.Reporter(),
		Text:     fmt.Sprintf("%s returned %d result(s)%s%s", promText(c.prom.Name(), qr.URI), series, estimate, above),
		Severity: severity,
	})

	if c.maxTotalSamples > 0 && qr.Stats.Samples.TotalQueryableSamples > c.maxTotalSamples {
		problems = append(problems, Problem{
			Fragment: expr.Value.Value,
			Lines:    expr.Lines(),
			Reporter: c.Reporter(),
			Text:     fmt.Sprintf("%s queried %d samples in total when executing this query, which is more than the configured limit of %d.", promText(c.prom.Name(), qr.URI), qr.Stats.Samples.TotalQueryableSamples, c.maxTotalSamples),
			Severity: c.severity,
		})
	}

	if c.maxPeakSamples > 0 && qr.Stats.Samples.PeakSamples > c.maxPeakSamples {
		problems = append(problems, Problem{
			Fragment: expr.Value.Value,
			Lines:    expr.Lines(),
			Reporter: c.Reporter(),
			Text:     fmt.Sprintf("%s queried %d peak samples when executing this query, which is more than the configured limit of %d.", promText(c.prom.Name(), qr.URI), qr.Stats.Samples.PeakSamples, c.maxPeakSamples),
			Severity: c.severity,
		})
	}

	evalDur := time.Duration(qr.Stats.Timings.EvalTotalTime * float64(time.Second))
	if c.maxEvaluationDuration > 0 && evalDur > c.maxEvaluationDuration {
		problems = append(problems, Problem{
			Fragment: expr.Value.Value,
			Lines:    expr.Lines(),
			Reporter: c.Reporter(),
			Text:     fmt.Sprintf("%s took %s when executing this query, which is more than the configured limit of %s.", promText(c.prom.Name(), qr.URI), output.HumanizeDuration(evalDur), output.HumanizeDuration(c.maxEvaluationDuration)),
			Severity: c.severity,
		})
	}

	return problems
}
