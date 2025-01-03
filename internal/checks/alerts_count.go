package checks

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"github.com/prometheus/common/model"

	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
)

const (
	AlertsCheckName = "alerts/count"
)

func NewAlertsCheck(prom *promapi.FailoverGroup, lookBack, step, resolve time.Duration, minCount int, comment string, severity Severity) AlertsCheck {
	return AlertsCheck{
		prom:     prom,
		lookBack: lookBack,
		step:     step,
		resolve:  resolve,
		minCount: minCount,
		comment:  comment,
		severity: severity,
	}
}

type AlertsCheck struct {
	prom     *promapi.FailoverGroup
	comment  string
	lookBack time.Duration
	step     time.Duration
	resolve  time.Duration
	minCount int
	severity Severity
}

func (c AlertsCheck) Meta() CheckMeta {
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

func (c AlertsCheck) String() string {
	return fmt.Sprintf("%s(%s)", AlertsCheckName, c.prom.Name())
}

func (c AlertsCheck) Reporter() string {
	return AlertsCheckName
}

func (c AlertsCheck) Check(ctx context.Context, _ discovery.Path, rule parser.Rule, _ []discovery.Entry) (problems []Problem) {
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
			Lines:    rule.AlertingRule.Expr.Value.Lines,
			Reporter: c.Reporter(),
			Text:     text,
			Severity: severity,
		})
		return problems
	}

	if len(qr.Series.Ranges) > 0 {
		promUptime, err := c.prom.RangeQuery(ctx, fmt.Sprintf("count(%s)", c.prom.UptimeMetric()), params)
		if err != nil {
			slog.Warn("Cannot detect Prometheus uptime gaps", slog.Any("err", err), slog.String("name", c.prom.Name()))
		} else {
			// FIXME: gaps are not used
			qr.Series.FindGaps(promUptime.Series, qr.Series.From, qr.Series.Until)
		}
	}

	var forDur model.Duration
	if rule.AlertingRule.For != nil {
		forDur, _ = model.ParseDuration(rule.AlertingRule.For.Value)
	}
	var keepFiringForDur model.Duration
	if rule.AlertingRule.KeepFiringFor != nil {
		keepFiringForDur, _ = model.ParseDuration(rule.AlertingRule.KeepFiringFor.Value)
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

	delta := qr.Series.Until.Sub(qr.Series.From).Round(time.Minute)
	details := fmt.Sprintf(`To get a preview of the alerts that would fire please [click here](%s/graph?g0.expr=%s&g0.tab=0&g0.range_input=%s).`,
		qr.URI, url.QueryEscape(rule.AlertingRule.Expr.Value.Value), output.HumanizeDuration(delta),
	)
	if c.comment != "" {
		details = fmt.Sprintf("%s\n%s", details, maybeComment(c.comment))
	}

	problems = append(problems, Problem{
		Lines:    rule.AlertingRule.Expr.Value.Lines,
		Reporter: c.Reporter(),
		Text:     fmt.Sprintf("%s would trigger %d alert(s) in the last %s.", promText(c.prom.Name(), qr.URI), alerts, output.HumanizeDuration(delta)),
		Details:  details,
		Severity: c.severity,
	})
	return problems
}
