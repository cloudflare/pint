package checks

import (
	"context"
	"fmt"

	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
)

const (
	RuleDuplicateCheckName = "rule/duplicate"
)

func NewRuleDuplicateCheck(prom *promapi.FailoverGroup) RuleDuplicateCheck {
	return RuleDuplicateCheck{
		prom:     prom,
		instance: fmt.Sprintf("%s(%s)", RuleDuplicateCheckName, prom.Name()),
	}
}

type RuleDuplicateCheck struct {
	prom     *promapi.FailoverGroup
	instance string
}

func (c RuleDuplicateCheck) Meta() CheckMeta {
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

func (c RuleDuplicateCheck) String() string {
	return c.instance
}

func (c RuleDuplicateCheck) Reporter() string {
	return RuleDuplicateCheckName
}

func (c RuleDuplicateCheck) Check(ctx context.Context, entry discovery.Entry, entries []discovery.Entry) (problems []Problem) {
	expr := entry.Rule.Expr()
	if expr.SyntaxError != nil {
		return problems
	}

	for _, other := range entries {
		if ignoreOtherEntry(entry, other, c.prom) {
			continue
		}
		if other.Rule.RecordingRule == nil {
			continue
		}

		// Look for identical recording rules.
		if other.Rule.RecordingRule != nil && entry.Rule.RecordingRule != nil && other.Rule.RecordingRule.Record.Value == entry.Rule.RecordingRule.Record.Value {
			problems = append(problems, c.compareRules(ctx, entry.Rule.RecordingRule, other, entry.Rule.Lines)...)
		}
	}
	return problems
}

func (c RuleDuplicateCheck) compareRules(_ context.Context, rule *parser.RecordingRule, entry discovery.Entry, lines diags.LineRange) (problems []Problem) {
	if !rule.Labels.IsIdentical(entry.Rule.RecordingRule.Labels) {
		return nil
	}

	if rule.Expr.Query.Expr.String() == entry.Rule.RecordingRule.Expr.Query.Expr.String() {
		problems = append(problems, Problem{
			Anchor:   AnchorAfter,
			Lines:    lines,
			Reporter: c.Reporter(),
			Summary:  "duplicated recording rule",
			Details:  "",
			Severity: Bug,
			Diagnostics: []diags.Diagnostic{
				{
					Message:     fmt.Sprintf("Duplicated rule, identical rule found at %s:%d.", entry.Path.SymlinkTarget, entry.Rule.RecordingRule.Record.Pos.Lines().First),
					Pos:         rule.Record.Pos,
					FirstColumn: 1,
					LastColumn:  len(rule.Record.Value),
					Kind:        diags.Issue,
				},
			},
		})
	}

	return problems
}

func ignoreOtherEntry(entry, other discovery.Entry, prom *promapi.FailoverGroup) bool {
	if other.State == discovery.Removed {
		return true
	}
	if other.PathError != nil {
		return true
	}
	if other.Rule.Error.Err != nil {
		return true
	}
	if other.Rule.Expr().SyntaxError != nil {
		return true
	}
	if other.Path.Name == entry.Path.Name && other.Rule.Lines.First == entry.Rule.Lines.First {
		return true
	}
	if !prom.IsEnabledForPath(entry.Path.Name) {
		return true
	}
	if !prom.IsEnabledForPath(other.Path.Name) {
		return true
	}
	return false
}
