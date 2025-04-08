package checks

import (
	"context"
	"fmt"

	"github.com/prometheus/common/model"

	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
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

func (c AlertsForChecksFor) Check(
	_ context.Context,
	entry discovery.Entry,
	_ []discovery.Entry,
) (problems []Problem) {
	if entry.Rule.AlertingRule == nil {
		return problems
	}

	if entry.Rule.AlertingRule.For != nil {
		problems = append(problems, c.checkField("for", entry.Rule.AlertingRule.For)...)
	}
	if entry.Rule.AlertingRule.KeepFiringFor != nil {
		problems = append(
			problems,
			c.checkField("keep_firing_for", entry.Rule.AlertingRule.KeepFiringFor)...)
	}

	return problems
}

func (c AlertsForChecksFor) checkField(name string, value *parser.YamlNode) (problems []Problem) {
	d, err := model.ParseDuration(value.Value)
	if err != nil {
		problems = append(problems, Problem{
			Anchor:   AnchorAfter,
			Lines:    value.Pos.Lines(),
			Reporter: c.Reporter(),
			Summary:  "invalid duration",
			Details:  AlertForCheckDurationHelp,
			Severity: Bug,
			Diagnostics: []diags.Diagnostic{
				{
					Message:     err.Error(),
					Pos:         value.Pos,
					FirstColumn: 1,
					LastColumn:  len(value.Value),
				},
			},
		})
		return problems
	}

	if d == 0 {
		problems = append(problems, Problem{
			Anchor:   AnchorAfter,
			Lines:    value.Pos.Lines(),
			Reporter: c.Reporter(),
			Summary:  "redundant field with default value",
			Details:  "",
			Severity: Information,
			Diagnostics: []diags.Diagnostic{
				{
					Message: fmt.Sprintf(
						"`%s` is the default value of `%s`, this line is unnecessary.",
						value.Value,
						name,
					),
					Pos:         value.Pos,
					FirstColumn: 1,
					LastColumn:  len(value.Value),
				},
			},
		})
	}

	return problems
}
