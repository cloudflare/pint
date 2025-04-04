package checks

import (
	"context"

	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/parser/utils"
)

const (
	ImpossibleCheckName = "promql/impossible"
)

func NewImpossibleCheck() ImpossibleCheck {
	return ImpossibleCheck{}
}

type ImpossibleCheck struct{}

func (c ImpossibleCheck) Meta() CheckMeta {
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

func (c ImpossibleCheck) String() string {
	return ImpossibleCheckName
}

func (c ImpossibleCheck) Reporter() string {
	return ImpossibleCheckName
}

func (c ImpossibleCheck) Check(_ context.Context, entry discovery.Entry, _ []discovery.Entry) (problems []Problem) {
	expr := entry.Rule.Expr()
	if expr.SyntaxError != nil {
		return problems
	}

	for _, src := range utils.LabelsSource(expr.Value.Value, expr.Query.Expr) {
		src.WalkSources(func(s utils.Source) {
			problems = append(problems, c.checkSource(expr, s)...)
		})
	}

	return problems
}

func (c ImpossibleCheck) checkSource(expr parser.PromQLExpr, s utils.Source) (problems []Problem) {
	if s.IsDead {
		problems = append(problems, Problem{
			Anchor:   AnchorAfter,
			Lines:    expr.Value.Pos.Lines(),
			Reporter: c.Reporter(),
			Summary:  "dead code in query",
			Details:  "",
			Diagnostics: []diags.Diagnostic{
				{
					Pos:         expr.Value.Pos,
					FirstColumn: int(s.IsDeadPosition.Start) + 1,
					LastColumn:  int(s.IsDeadPosition.End),
					Message:     s.IsDeadReason,
				},
			},
			Severity: Warning,
		})
	}
	return problems
}
