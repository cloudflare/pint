package checks

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/cloudflare/pint/internal/discovery"
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

func (c AlertsCheck) Meta() CheckMeta {
	return CheckMeta{IsOnline: true}
}

func (c AlertsCheck) String() string {
	return fmt.Sprintf("%s(%s)", AlertsCheckName, c.prom.Name())
}

func (c AlertsCheck) Reporter() string {
	return AlertsCheckName
}

func (c AlertsCheck) Check(ctx context.Context, rule parser.Rule, entries []discovery.Entry) (problems []Problem) {
	if rule.AlertingRule == nil {
		return
	}

	if rule.AlertingRule.Expr.SyntaxError != nil {
		return
	}

	qr, err := c.prom.RangeQuery(ctx, rule.AlertingRule.Expr.Value.Value, promapi.NewRelativeRange(c.lookBack, c.step))
	if err != nil {
		text, severity := textAndSeverityFromError(err, c.Reporter(), c.prom.Name(), Bug)
		problems = append(problems, Problem{
			Fragment: rule.AlertingRule.Expr.Value.Value,
			Lines:    rule.AlertingRule.Expr.Lines(),
			Reporter: c.Reporter(),
			Text:     text,
			Severity: severity,
		})
		return
	}

	if len(qr.Series.Ranges) > 0 {
		promUptime, err := c.prom.RangeQuery(ctx, "count(up)", promapi.NewRelativeRange(c.lookBack, c.step))
		if err != nil {
			log.Warn().Err(err).Str("name", c.prom.Name()).Msg("Cannot detect Prometheus uptime gaps")
		} else {
			// FIXME: gaps are not used
			qr.Series.FindGaps(promUptime.Series, qr.Series.From, qr.Series.Until)
		}
	}

	var forDur time.Duration
	if rule.AlertingRule.For != nil {
		forDur, _ = time.ParseDuration(rule.AlertingRule.For.Value.Value)
	}

	var alerts int
	for _, r := range qr.Series.Ranges {
		if r.End.Sub(r.Start) > forDur {
			alerts++
		}
	}

	lines := []int{}
	lines = append(lines, rule.AlertingRule.Expr.Lines()...)
	if rule.AlertingRule.For != nil {
		lines = append(lines, rule.AlertingRule.For.Lines()...)
	}
	sort.Ints(lines)

	delta := qr.Series.Until.Sub(qr.Series.From)
	problems = append(problems, Problem{
		Fragment: rule.AlertingRule.Expr.Value.Value,
		Lines:    lines,
		Reporter: c.Reporter(),
		Text:     fmt.Sprintf("%s would trigger %d alert(s) in the last %s", promText(c.prom.Name(), qr.URI), alerts, output.HumanizeDuration(delta)),
		Severity: Information,
	})
	return
}
