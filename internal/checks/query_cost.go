package checks

import (
	"fmt"
	"time"

	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
)

const (
	CostCheckName = "query/cost"
)

func NewCostCheck(name, uri string, timeout time.Duration, bps, maxSeries int, severity Severity) CostCheck {
	return CostCheck{
		name:           name,
		uri:            uri,
		timeout:        timeout,
		bytesPerSample: bps,
		maxSeries:      maxSeries,
		severity:       severity,
	}
}

type CostCheck struct {
	name           string
	uri            string
	timeout        time.Duration
	bytesPerSample int
	maxSeries      int
	severity       Severity
}

func (c CostCheck) String() string {
	return fmt.Sprintf("%s(%s)", CostCheckName, c.name)
}

func (c CostCheck) Check(rule parser.Rule) (problems []Problem) {
	expr := rule.Expr()

	if expr.SyntaxError != nil {
		return
	}

	query := fmt.Sprintf("count(%s)", expr.Value.Value)
	qr, err := promapi.Query(c.uri, c.timeout, query, nil)
	if err != nil {
		problems = append(problems, Problem{
			Fragment: expr.Value.Value,
			Lines:    expr.Lines(),
			Reporter: CostCheckName,
			Text:     fmt.Sprintf("query using %s failed with: %s", c.name, err),
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
		estimate = fmt.Sprintf(" with %s estimated memory usage", promapi.HumanizeBytes(c.bytesPerSample*series))
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
		Reporter: CostCheckName,
		Text:     fmt.Sprintf("query using %s completed in %.2fs returning %d result(s)%s%s", c.name, qr.DurationSeconds, series, estimate, above),
		Severity: severity,
	})
	return
}
