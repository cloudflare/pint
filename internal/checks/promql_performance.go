package checks

import (
	"context"
	"fmt"

	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser/utils"
	"github.com/cloudflare/pint/internal/promapi"
)

const (
	PerformanceCheckName = "promql/performance"
)

func NewPerformanceCheck(prom *promapi.FailoverGroup) PerformanceCheck {
	return PerformanceCheck{prom: prom}
}

type PerformanceCheck struct {
	prom *promapi.FailoverGroup
}

func (c PerformanceCheck) Meta() CheckMeta {
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

func (c PerformanceCheck) String() string {
	return fmt.Sprintf("%s(%s)", PerformanceCheckName, c.prom.Name())
}

func (c PerformanceCheck) Reporter() string {
	return PerformanceCheckName
}

func (c PerformanceCheck) Check(_ context.Context, entry discovery.Entry, entries []discovery.Entry) (problems []Problem) {
	expr := entry.Rule.Expr()
	if expr.SyntaxError != nil {
		return problems
	}

	src := utils.LabelsSource(expr.Value.Value, expr.Query.Expr)

	for _, other := range entries {
		if ignoreOtherEntry(entry, other, c.prom) {
			continue
		}
		if other.Rule.RecordingRule == nil {
			continue
		}

		otherSrc := utils.LabelsSource(other.Rule.RecordingRule.Expr.Value.Value, other.Rule.RecordingRule.Expr.Query.Expr)
		if len(otherSrc) > 1 {
			continue
		}
		for _, s := range src {
			s.WalkSources(func(s utils.Source) {
				for _, os := range otherSrc {
					if os.Type != utils.FuncSource && os.Type != utils.AggregateSource {
						return
					}
					if os.Operation == "vector" {
						return
					}
					oop := os.Operations[len(os.Operations)-1]
					for _, op := range s.Operations {
						if op.Pretty(0) == oop.Pretty(0) {
							problems = append(problems, Problem{
								Anchor:   AnchorAfter,
								Lines:    expr.Value.Pos.Lines(),
								Reporter: c.Reporter(),
								Summary:  "query should use recording rule",
								Details:  "There is a recording rule that already stores the result of this query, use it here to speed up this query.",
								Severity: Information,
								Diagnostics: []diags.Diagnostic{
									{
										Message:     fmt.Sprintf("Use `%s` here instead to speed up the query", other.Rule.RecordingRule.Record.Value),
										Pos:         expr.Value.Pos,
										FirstColumn: int(op.PositionRange().Start) + 1,
										LastColumn:  int(op.PositionRange().End),
										Kind:        diags.Issue,
									},
								},
							})
						}
					}
				}
			})
		}
	}
	return problems
}
