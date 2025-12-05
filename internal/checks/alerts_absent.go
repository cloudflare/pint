package checks

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/parser/source"
	"github.com/cloudflare/pint/internal/promapi"

	"github.com/prometheus/common/model"
	promParser "github.com/prometheus/prometheus/promql/parser"
)

const (
	AlertsAbsentCheckName    = "alerts/absent"
	AlertsAbsentCheckDetails = "When Prometheus restart this alert rule might be evaluated before your service is scraped, which can cause false positives from absent() call.\nAdding `for` option that is at least 2x scrape interval will prevent this from happening."
)

func NewAlertsAbsentCheck(prom *promapi.FailoverGroup) AlertsAbsentCheck {
	return AlertsAbsentCheck{
		prom:     prom,
		instance: fmt.Sprintf("%s(%s)", AlertsAbsentCheckName, prom.Name()),
	}
}

type AlertsAbsentCheck struct {
	prom     *promapi.FailoverGroup
	instance string
}

func (c AlertsAbsentCheck) Meta() CheckMeta {
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

func (c AlertsAbsentCheck) String() string {
	return c.instance
}

func (c AlertsAbsentCheck) Reporter() string {
	return AlertsAbsentCheckName
}

func (c AlertsAbsentCheck) Check(ctx context.Context, entry *discovery.Entry, _ []*discovery.Entry) (problems []Problem) {
	if entry.Rule.AlertingRule == nil {
		return problems
	}

	if entry.Rule.AlertingRule.Expr.SyntaxError() != nil {
		return problems
	}

	src := entry.Rule.AlertingRule.Expr.Source()
	absentSources := make([]source.Source, 0, len(src))
	for _, s := range src {
		if s.Operation() != "absent" {
			continue
		}
		absentSources = append(absentSources, s)
	}

	if len(absentSources) == 0 {
		return problems
	}

	cfg, err := c.prom.Config(ctx, 0)
	if err != nil {
		if errors.Is(err, promapi.ErrUnsupported) {
			c.prom.DisableCheck(promapi.APIPathConfig, c.Reporter())
			return problems
		}
		problems = append(problems, problemFromError(err, entry.Rule, c.Reporter(), c.prom.Name(), Warning))
		return problems
	}

	var forVal time.Duration
	if entry.Rule.AlertingRule.For != nil {
		forDur, err := model.ParseDuration(entry.Rule.AlertingRule.For.Value)
		if err != nil {
			return problems
		}
		forVal = time.Duration(forDur)
	}

	for _, s := range absentSources {
		var summary string

		call, _ := source.MostOuterOperation[*promParser.Call](s)
		if s.ReturnInfo.AlwaysReturns {
			problems = append(problems, Problem{
				Anchor:   AnchorAfter,
				Lines:    entry.Rule.AlertingRule.Expr.Value.Pos.Lines(),
				Reporter: c.Reporter(),
				Summary:  "never firing alert",
				Details:  "",
				Severity: Warning,
				Diagnostics: []diags.Diagnostic{
					{
						Message:     "Query passed inside `absent()` will always return some results, so absent will never return anything.",
						Pos:         entry.Rule.AlertingRule.Expr.Value.Pos,
						FirstColumn: int(call.PosRange.Start) + 1,
						LastColumn:  int(call.PosRange.End),
						Kind:        diags.Issue,
					},
				},
			})
		}

		if forVal >= cfg.Config.Global.ScrapeInterval*2 {
			continue
		}

		dgs := []diags.Diagnostic{
			{
				Message:     "Using `absent()` might cause false positive alerts when Prometheus restarts.",
				Pos:         entry.Rule.AlertingRule.Expr.Value.Pos,
				FirstColumn: int(call.PosRange.Start) + 1,
				LastColumn:  int(call.PosRange.End),
				Kind:        diags.Issue,
			},
		}
		if forVal > 0 {
			summary = "absent() based alert with insufficient for"
			dgs = append(dgs, diags.Diagnostic{
				Message: fmt.Sprintf("Use a value that's at least twice Prometheus scrape interval (`%s`).",
					output.HumanizeDuration(cfg.Config.Global.ScrapeInterval)),
				Pos:         entry.Rule.AlertingRule.For.Pos,
				FirstColumn: 1,
				LastColumn:  len(entry.Rule.AlertingRule.For.Value),
				Kind:        diags.Issue,
			})
		} else {
			summary = "absent() based alert without for"
		}

		problems = append(problems, Problem{
			Anchor:      AnchorAfter,
			Lines:       entry.Rule.AlertingRule.Expr.Value.Pos.Lines(),
			Reporter:    c.Reporter(),
			Summary:     summary,
			Details:     AlertsAbsentCheckDetails,
			Severity:    Warning,
			Diagnostics: dgs,
		})
	}

	return problems
}
