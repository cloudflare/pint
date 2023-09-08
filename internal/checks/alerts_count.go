package checks

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/prometheus/common/model"
	"github.com/rs/zerolog/log"

	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
)

const (
	AlertsCheckName = "alerts/count"
)

func NewAlertsCheck(prom *promapi.FailoverGroup, lookBack, step, resolve time.Duration, minCount int, severity Severity) AlertsCheck {
	return AlertsCheck{
		prom:     prom,
		lookBack: lookBack,
		step:     step,
		resolve:  resolve,
		minCount: minCount,
		severity: severity,
	}
}

type AlertsCheck struct {
	prom     *promapi.FailoverGroup
	lookBack time.Duration
	step     time.Duration
	resolve  time.Duration
	minCount int
	severity Severity
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

func (c AlertsCheck) Check(ctx context.Context, _ string, rule parser.Rule, _ []discovery.Entry) (problems []Problem) {
	if rule.AlertingRule == nil {
		return problems
	}

	if rule.AlertingRule.Expr.SyntaxError != nil {
		return problems
	}

	params := promapi.NewRelativeRange(c.lookBack, c.step)

	qr, err := c.prom.RangeQuery(ctx, rule.AlertingRule.Expr.Value.Value, params)
	if err != nil {
		text, severity := textAndSeverityFromError(err, c.Reporter(), c.prom.Name(), Bug)
		problems = append(problems, Problem{
			Fragment: rule.AlertingRule.Expr.Value.Value,
			Lines:    rule.AlertingRule.Expr.Lines(),
			Reporter: c.Reporter(),
			Text:     text,
			Severity: severity,
		})
		return problems
	}

	if len(qr.Series.Ranges) > 0 {
		promUptime, err := c.prom.RangeQuery(ctx, "count(up)", params)
		if err != nil {
			log.Warn().Err(err).Str("name", c.prom.Name()).Msg("Cannot detect Prometheus uptime gaps")
		} else {
			// FIXME: gaps are not used
			qr.Series.FindGaps(promUptime.Series, qr.Series.From, qr.Series.Until)
		}
	}

	var forDur model.Duration
	if rule.AlertingRule.For != nil {
		forDur, _ = model.ParseDuration(rule.AlertingRule.For.Value.Value)
	}
	var keepFiringForDur model.Duration
	if rule.AlertingRule.For != nil {
		keepFiringForDur, _ = model.ParseDuration(rule.AlertingRule.KeepFiringFor.Value.Value)
	}

	var alerts int
	for _, r := range qr.Series.Ranges {
		// If `keepFiringFor` is not defined its Duration will be 0
		if r.End.Sub(r.Start) > (time.Duration(forDur) + time.Duration(keepFiringForDur)) {
			alerts++
		}
	}

	if alerts < c.minCount {
		return problems
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
		Severity: c.severity,
	})
	return problems
}
