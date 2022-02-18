package checks

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
)

const (
	AlertsCheckName = "alerts/count"
)

func NewAlertsCheck(prom *promapi.FailoverGroup, lookBack, step, resolve time.Duration) AlertsCheck {
	return AlertsCheck{
		prom:     prom,
		lookBack: lookBack,
		step:     step,
		resolve:  resolve,
	}
}

type AlertsCheck struct {
	prom     *promapi.FailoverGroup
	lookBack time.Duration
	step     time.Duration
	resolve  time.Duration
}

func (c AlertsCheck) String() string {
	return fmt.Sprintf("%s(%s)", AlertsCheckName, c.prom.Name())
}

func (c AlertsCheck) Reporter() string {
	return AlertsCheckName
}

func (c AlertsCheck) Check(ctx context.Context, rule parser.Rule) (problems []Problem) {
	if rule.AlertingRule == nil {
		return
	}

	if rule.AlertingRule.Expr.SyntaxError != nil {
		return
	}

	end := time.Now()
	start := end.Add(-1 * c.lookBack)

	qr, err := c.prom.RangeQuery(ctx, rule.AlertingRule.Expr.Value.Value, start, end, c.step)
	if err != nil {
		problems = append(problems, Problem{
			Fragment: rule.AlertingRule.Expr.Value.Value,
			Lines:    rule.AlertingRule.Expr.Lines(),
			Reporter: c.Reporter(),
			Text:     fmt.Sprintf("query using %s failed with: %s", c.prom.Name(), err),
			Severity: Bug,
		})
		return
	}

	var forDur time.Duration
	if rule.AlertingRule.For != nil {
		forDur, _ = time.ParseDuration(rule.AlertingRule.For.Value.Value)
	}

	var alerts int
	for _, sample := range qr.Samples {
		var isAlerting, isNew bool
		var firstTime, lastTime time.Time
		for _, value := range sample.Values {
			isNew = value.Timestamp.Time().After(lastTime.Add(c.step))
			if isNew {
				if rule.AlertingRule.For != nil {
					isAlerting = false
				} else {
					isAlerting = true
					alerts++
				}
				firstTime = value.Timestamp.Time()
			} else {
				if !isAlerting && rule.AlertingRule.For != nil {
					if !value.Timestamp.Time().Before(firstTime.Add(forDur)) {
						isAlerting = true
						alerts++
					}
				}
			}
			lastTime = value.Timestamp.Time()
		}
	}

	lines := []int{}
	lines = append(lines, rule.AlertingRule.Expr.Lines()...)
	if rule.AlertingRule.For != nil {
		lines = append(lines, rule.AlertingRule.For.Lines()...)
	}
	sort.Ints(lines)

	delta := qr.End.Sub(qr.Start)
	problems = append(problems, Problem{
		Fragment: rule.AlertingRule.Expr.Value.Value,
		Lines:    lines,
		Reporter: c.Reporter(),
		Text:     fmt.Sprintf("query using %s would trigger %d alert(s) in the last %s", c.prom.Name(), alerts, output.HumanizeDuration(delta)),
		Severity: Information,
	})
	return
}
