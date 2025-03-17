package checks

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/parser/utils"
	"github.com/cloudflare/pint/internal/promapi"

	"github.com/prometheus/common/model"
)

const (
	AlertsAbsentCheckName    = "alerts/absent"
	AlertsAbsentCheckDetails = "When Prometheus restart this alert rule might be evaluated before your service is scraped, which can cause false positives from absent() call.\nAdding `for` option that is at least 2x scrape interval will prevent this from happening."
)

func NewAlertsAbsentCheck(prom *promapi.FailoverGroup) AlertsAbsentCheck {
	return AlertsAbsentCheck{
		prom: prom,
	}
}

type AlertsAbsentCheck struct {
	prom *promapi.FailoverGroup
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
	return fmt.Sprintf("%s(%s)", AlertsAbsentCheckName, c.prom.Name())
}

func (c AlertsAbsentCheck) Reporter() string {
	return AlertsAbsentCheckName
}

func (c AlertsAbsentCheck) Check(ctx context.Context, _ discovery.Path, rule parser.Rule, _ []discovery.Entry) (problems []Problem) {
	if rule.AlertingRule == nil {
		return problems
	}

	if rule.AlertingRule.Expr.SyntaxError != nil {
		return problems
	}

	src := utils.LabelsSource(rule.AlertingRule.Expr.Value.Value, rule.AlertingRule.Expr.Query.Expr)
	absentSources := make([]utils.Source, 0, len(src))
	for _, s := range src {
		if s.Operation != "absent" {
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
		problems = append(problems, problemFromError(err, rule, c.Reporter(), c.prom.Name(), Warning))
		return problems
	}

	var forVal time.Duration
	if rule.AlertingRule.For != nil {
		forDur, err := model.ParseDuration(rule.AlertingRule.For.Value)
		if err != nil {
			return problems
		}
		forVal = time.Duration(forDur)
		if forVal >= cfg.Config.Global.ScrapeInterval*2 {
			return problems
		}
	}

	for _, s := range absentSources {
		var summary string
		dgs := []diags.Diagnostic{
			{
				Message:     "Using `absent()` might cause false positive alerts when Prometheus restarts.",
				Pos:         rule.AlertingRule.Expr.Value.Pos,
				FirstColumn: int(s.Call.PosRange.Start) + 1,
				LastColumn:  int(s.Call.PosRange.End),
			},
		}
		if forVal > 0 {
			summary = "absent() based alert with insufficient for"
			dgs = append(dgs, diags.Diagnostic{
				Message: fmt.Sprintf("Use a value that's at least twice Prometheus scrape interval (`%s`).",
					output.HumanizeDuration(cfg.Config.Global.ScrapeInterval)),
				Pos:         rule.AlertingRule.For.Pos,
				FirstColumn: 1,
				LastColumn:  len(rule.AlertingRule.For.Value),
			})
		} else {
			summary = "absent() based alert without for"
		}

		problems = append(problems, Problem{
			Anchor:      AnchorAfter,
			Lines:       rule.AlertingRule.Expr.Value.Pos.Lines(),
			Reporter:    c.Reporter(),
			Summary:     summary,
			Details:     AlertsAbsentCheckDetails,
			Severity:    Warning,
			Diagnostics: dgs,
		})
	}

	return problems
}
