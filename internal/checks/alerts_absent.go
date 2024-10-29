package checks

import (
	"context"
	"fmt"
	"time"

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
		IsOnline: true,
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

	var hasAbsent bool
	src := utils.LabelsSource(rule.AlertingRule.Expr.Value.Value, rule.AlertingRule.Expr.Query.Expr)
	for _, s := range append(src.Alternatives, src) {
		if s.Operation == "absent" {
			hasAbsent = true
		}
	}

	if !hasAbsent {
		return problems
	}

	cfg, err := c.prom.Config(ctx, 0)
	if err != nil {
		text, severity := textAndSeverityFromError(err, c.Reporter(), c.prom.Name(), Warning)
		problems = append(problems, Problem{
			Lines:    rule.AlertingRule.Expr.Value.Lines,
			Reporter: c.Reporter(),
			Text:     text,
			Severity: severity,
		})
		return problems
	}

	if rule.AlertingRule.For != nil {
		forDur, err := model.ParseDuration(rule.AlertingRule.For.Value)
		if err != nil {
			return problems
		}
		if time.Duration(forDur) >= cfg.Config.Global.ScrapeInterval*2 {
			return problems
		}
	}

	problems = append(problems, Problem{
		Lines:    rule.AlertingRule.Expr.Value.Lines,
		Reporter: c.Reporter(),
		Text: fmt.Sprintf("Alert query is using absent() which might cause false positives when %s restarts, please add `for: %s` to avoid this.",
			promText(c.prom.Name(), cfg.URI),
			output.HumanizeDuration((cfg.Config.Global.ScrapeInterval * 2).Round(time.Minute)),
		),
		Details:  AlertsAbsentCheckDetails,
		Severity: Warning,
	})

	return problems
}
