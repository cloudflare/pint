package checks

import (
	"context"

	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/output"
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

func (c ImpossibleCheck) Check(_ context.Context, _ discovery.Path, rule parser.Rule, _ []discovery.Entry) (problems []Problem) {
	expr := rule.Expr()
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
		pos := s.GetSmallestPosition()
		problems = append(problems, Problem{
			Anchor:   AnchorAfter,
			Lines:    expr.Value.Lines,
			Reporter: c.Reporter(),
			Summary:  "dead code in query",
			Details:  "",
			Diagnostics: []output.Diagnostic{
				{
					Pos:         expr.Value.Pos,
					FirstColumn: int(pos.Start) + 1,
					LastColumn:  int(pos.End),
					Message:     s.IsDeadReason,
				},
			},
			Severity: Warning,
		})
	}
	return problems
}
