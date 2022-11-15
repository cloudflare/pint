package checks

import (
	"context"
	"fmt"

	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
)

const (
	CostCheckName       = "query/cost"
	BytesPerSampleQuery = "avg(avg_over_time(go_memstats_alloc_bytes[2h]) / avg_over_time(prometheus_tsdb_head_series[2h]))"
)

func NewCostCheck(prom *promapi.FailoverGroup, maxSeries int, severity Severity) CostCheck {
	return CostCheck{
		prom:      prom,
		maxSeries: maxSeries,
		severity:  severity,
	}
}

type CostCheck struct {
	prom      *promapi.FailoverGroup
	maxSeries int
	severity  Severity
}

func (c CostCheck) Meta() CheckMeta {
	return CheckMeta{IsOnline: true}
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

func (c CostCheck) Check(ctx context.Context, rule parser.Rule, entries []discovery.Entry) (problems []Problem) {
	expr := rule.Expr()

	if expr.SyntaxError != nil {
		return
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
		return
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
		above = fmt.Sprintf(", maximum allowed series is %d", c.maxSeries)
	}

	problems = append(problems, Problem{
		Fragment: expr.Value.Value,
		Lines:    expr.Lines(),
		Reporter: c.Reporter(),
		Text:     fmt.Sprintf("%s returned %d result(s)%s%s", promText(c.prom.Name(), qr.URI), series, estimate, above),
		Severity: severity,
	})
	return
}
