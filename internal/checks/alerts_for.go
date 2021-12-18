package checks

import (
	"fmt"

	"github.com/cloudflare/pint/internal/parser"
	"github.com/prometheus/common/model"
)

const (
	AlertForCheckName = "alerts/for"
)

func NewAlertsForCheck() AlertsForChecksFor {
	return AlertsForChecksFor{}
}

type AlertsForChecksFor struct {
}

func (c AlertsForChecksFor) String() string {
	return AlertForCheckName
}

func (c AlertsForChecksFor) Check(rule parser.Rule) (problems []Problem) {
	if rule.AlertingRule == nil || rule.AlertingRule.For == nil {
		return
	}

	d, err := model.ParseDuration(rule.AlertingRule.For.Value.Value)
	if err != nil {
		problems = append(problems, Problem{
			Fragment: rule.AlertingRule.For.Value.Value,
			Lines:    rule.AlertingRule.For.Lines(),
			Reporter: AlertForCheckName,
			Text:     fmt.Sprintf("invalid duration: %s", err),
			Severity: Bug,
		})
		return
	}

	if d == 0 {
		problems = append(problems, Problem{
			Fragment: rule.AlertingRule.For.Value.Value,
			Lines:    rule.AlertingRule.For.Lines(),
			Reporter: AlertForCheckName,
			Text: fmt.Sprintf("%q is the default value of %q, consider removing this line",
				rule.AlertingRule.For.Value.Value, rule.AlertingRule.For.Key.Value),
			Severity: Information,
		})
	}

	return
}
