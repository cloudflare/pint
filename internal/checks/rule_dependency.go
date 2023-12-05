package checks

import (
	"context"
	"fmt"

	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/parser/utils"
	"github.com/cloudflare/pint/internal/promapi"
)

const (
	RuleDependencyCheckName = "rule/dependency"
)

func NewRuleDependencyCheck(prom *promapi.FailoverGroup) RuleDependencyCheck {
	return RuleDependencyCheck{prom: prom}
}

type RuleDependencyCheck struct {
	prom *promapi.FailoverGroup
}

func (c RuleDependencyCheck) Meta() CheckMeta {
	return CheckMeta{
		States: []discovery.ChangeType{
			discovery.Removed,
		},
		IsOnline: false,
	}
}

func (c RuleDependencyCheck) String() string {
	return fmt.Sprintf("%s(%s)", RuleDependencyCheckName, c.prom.Name())
}

func (c RuleDependencyCheck) Reporter() string {
	return RuleDependencyCheckName
}

func (c RuleDependencyCheck) Check(_ context.Context, path string, rule parser.Rule, entries []discovery.Entry) (problems []Problem) {
	if rule.RecordingRule == nil {
		return problems
	}

	for _, entry := range entries {
		if entry.State == discovery.Removed {
			continue
		}
		if !c.prom.IsEnabledForPath(entry.SourcePath) {
			continue
		}
		if c.usesVector(entry, rule.RecordingRule.Record.Value.Value) {
			expr := entry.Rule.Expr()
			problems = append(problems, Problem{
				Fragment: fmt.Sprintf("%s: %s", expr.Key.Value, expr.Value.Value),
				Lines:    expr.Lines(),
				Reporter: c.Reporter(),
				Text: fmt.Sprintf(
					"This rule uses a metric produced by %s rule `%s` which was removed from %s.",
					rule.Type(),
					rule.RecordingRule.Record.Value.Value,
					path,
				),
				Details: fmt.Sprintf(
					"If you remove the recording rule generating `%s` and there is no other source of `%s` metric, then this and other rule depending on `%s` will break.",
					rule.RecordingRule.Record.Value.Value, rule.RecordingRule.Record.Value.Value, rule.RecordingRule.Record.Value.Value,
				),
				Severity: Warning,
			})
		}
	}

	return problems
}

func (c RuleDependencyCheck) usesVector(entry discovery.Entry, name string) bool {
	expr := entry.Rule.Expr()
	if expr.SyntaxError != nil {
		return false
	}

	for _, vs := range utils.HasVectorSelector(expr.Query) {
		if vs.Name == name {
			return true
		}
	}

	return false
}
