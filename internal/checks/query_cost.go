package checks

import (
	"context"
	"fmt"

	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
)

const (
	CostCheckName = "query/cost"
)

func NewCostCheck(prom *promapi.FailoverGroup, bps, maxSeries int, severity Severity) CostCheck {
	return CostCheck{
		prom:           prom,
		bytesPerSample: bps,
		maxSeries:      maxSeries,
		severity:       severity,
	}
}

type CostCheck struct {
	prom           *promapi.FailoverGroup
	bytesPerSample int
	maxSeries      int
	severity       Severity
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

func (c CostCheck) Check(ctx context.Context, rule parser.Rule) (problems []Problem) {
	expr := rule.Expr()

	if expr.SyntaxError != nil {
		return
	}

	query := fmt.Sprintf("count(%s)", expr.Value.Value)
	qr, err := c.prom.Query(ctx, query)
	if err != nil {
		problems = append(problems, Problem{
			Fragment: expr.Value.Value,
			Lines:    expr.Lines(),
			Reporter: c.Reporter(),
			Text:     fmt.Sprintf("query using %s failed with: %s", c.prom.Name(), err),
			Severity: Bug,
		})
		return
	}

	var series int
	for _, s := range qr.Series {
		series += int(s.Value)
	}

	var estimate string
	if c.bytesPerSample > 0 && series > 0 {
		estimate = fmt.Sprintf(" with %s estimated memory usage", output.HumanizeBytes(c.bytesPerSample*series))
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
		Text:     fmt.Sprintf("query using %s completed in %.2fs returning %d result(s)%s%s", c.prom.Name(), qr.DurationSeconds, series, estimate, above),
		Severity: severity,
	})
	return
}
