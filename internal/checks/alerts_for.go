package checks

import (
	"context"
	"fmt"

	"github.com/prometheus/common/model"

	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/parser"
)

const (
	AlertForCheckName         = "alerts/for"
	AlertForCheckDurationHelp = `Supported time durations are documented [here](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-durations).`
)

func NewAlertsForCheck() AlertsForChecksFor {
	return AlertsForChecksFor{}
}

type AlertsForChecksFor struct{}

func (c AlertsForChecksFor) Meta() CheckMeta {
	return CheckMeta{
		States: []discovery.ChangeType{
			discovery.Noop,
			discovery.Added,
			discovery.Modified,
			discovery.Moved,
		},
		Online:        false,
		AlwaysEnabled: false,
	}
}

func (c AlertsForChecksFor) String() string {
	return AlertForCheckName
}

func (c AlertsForChecksFor) Reporter() string {
	return AlertForCheckName
}

func (c AlertsForChecksFor) Check(_ context.Context, _ discovery.Path, rule parser.Rule, _ []discovery.Entry) (problems []Problem) {
	if rule.AlertingRule == nil {
		return problems
	}

	if rule.AlertingRule.For != nil {
		problems = append(problems, c.checkField("for", rule.AlertingRule.For)...)
	}
	if rule.AlertingRule.KeepFiringFor != nil {
		problems = append(problems, c.checkField("keep_firing_for", rule.AlertingRule.KeepFiringFor)...)
	}

	return problems
}

func (c AlertsForChecksFor) checkField(name string, value *parser.YamlNode) (problems []Problem) {
	d, err := model.ParseDuration(value.Value)
	if err != nil {
		problems = append(problems, Problem{
			Lines:    value.Lines,
			Reporter: c.Reporter(),
			Summary:  "invalid duration",
			Details:  AlertForCheckDurationHelp,
			Severity: Bug,
			Diagnostics: []output.Diagnostic{
				{
					Message:     err.Error(),
					Line:        value.Lines.First,
					FirstColumn: value.Column,
					LastColumn:  nodeLastColumn(value),
				},
			},
		})
		return problems
	}

	if d == 0 {
		problems = append(problems, Problem{
			Lines:    value.Lines,
			Reporter: c.Reporter(),
			Summary:  "redundant field with default value",
			Severity: Information,
			Diagnostics: []output.Diagnostic{
				{
					Message:     fmt.Sprintf("`%s` is the default value of `%s`, this line is unnecessary.", value.Value, name),
					Line:        value.Lines.First,
					FirstColumn: value.Column,
					LastColumn:  nodeLastColumn(value),
				},
			},
		})
	}

	return problems
}
