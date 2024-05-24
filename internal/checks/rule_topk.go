package checks

import (
	"context"
	"strings"

	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
)

const (
	RuleTopkCheckName    = "rule/topk"
	TopkCheckRuleDetails = `Using topk or bottomk in recording rules can mean the series in the recording rule change frequently.
This leads to high series churn which negatively impacts Prometheus performance and consumes more storage. It is generally not advisable to use topk or bottomk in recording rules unless the result is expected to be consistent.
`
)

func NewRuleTopkCheck() RuleTopkCheck {
	return RuleTopkCheck{}
}

type RuleTopkCheck struct{}

func (c RuleTopkCheck) Meta() CheckMeta {
	return CheckMeta{
		States: []discovery.ChangeType{
			discovery.Added,
			discovery.Modified,
			discovery.Noop,
		},
		IsOnline: false,
	}
}

func (c RuleTopkCheck) String() string {
	return RuleTopkCheckName
}

func (c RuleTopkCheck) Reporter() string {
	return RuleTopkCheckName
}

func (c RuleTopkCheck) Check(_ context.Context, _ discovery.Path, rule parser.Rule, _ []discovery.Entry) (problems []Problem) {

	if rule.RecordingRule == nil || rule.RecordingRule.Expr.SyntaxError != nil {
		return nil
	}

	if rule.RecordingRule != nil {
		problems = append(problems, c.checkRecordingRule(rule)...)
	}

	return problems
}

func (c RuleTopkCheck) checkRecordingRule(rule parser.Rule) (problems []Problem) {

	if strings.Contains(rule.RecordingRule.Expr.Value.Value, "topk") || strings.Contains(rule.RecordingRule.Expr.Value.Value, "bottomk") {
		problems = append(problems, Problem{
			Lines:    rule.RecordingRule.Expr.Value.Lines,
			Reporter: c.Reporter(),
			Text:     "usage of topk or bottomk in recording rules is discouraged and creates churn",
			Details:  TopkCheckRuleDetails,
			Severity: Warning,
		})
	}
	return problems
}
