package checks

import (
	"context"

	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/parser/source"
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
		AlwaysEnabled: false,
	}
}

func (c ImpossibleCheck) String() string {
	return ImpossibleCheckName
}

func (c ImpossibleCheck) Reporter() string {
	return ImpossibleCheckName
}

func (c ImpossibleCheck) Check(_ context.Context, entry *discovery.Entry, _ []*discovery.Entry) (problems []Problem) {
	expr := entry.Rule.Expr()
	if expr.SyntaxError() != nil {
		return problems
	}

	for _, src := range expr.Source() {
		src.WalkSources(func(s *source.Source, j *source.Join, u *source.Unless) {
			problems = append(problems, c.checkSource(expr, s)...)
			if j != nil {
				problems = append(problems, c.checkDeadInfo(expr, j.DeadInfo)...)
				for _, dl := range j.DeadLabels {
					problems = append(problems, c.deadLabelProblem(expr, dl))
				}
			}
			if u != nil {
				problems = append(problems, c.checkDeadInfo(expr, u.DeadInfo)...)
			}
		})
	}

	return problems
}

func (c ImpossibleCheck) checkSource(expr *parser.PromQLExpr, s *source.Source) (problems []Problem) {
	problems = append(problems, c.checkDeadInfo(expr, s.DeadInfo)...)
	for _, dl := range s.DeadLabels {
		problems = append(problems, c.deadLabelProblem(expr, dl))
	}
	return problems
}

func (c ImpossibleCheck) checkDeadInfo(expr *parser.PromQLExpr, di *source.DeadInfo) (problems []Problem) {
	if di == nil {
		return problems
	}
	problems = append(problems, Problem{
		Anchor:   AnchorAfter,
		Lines:    expr.Value.Pos.Lines(),
		Reporter: c.Reporter(),
		Summary:  "dead code in query",
		Details:  "",
		Diagnostics: []diags.Diagnostic{
			{
				Pos:         expr.Value.Pos,
				Expr:        expr.Query().Expr,
				FirstColumn: int(di.Fragment.Start) + 1,
				LastColumn:  int(di.Fragment.End),
				Message:     di.Reason,
				Kind:        diags.Issue,
			},
		},
		Severity: Warning,
	})
	return problems
}

func (c ImpossibleCheck) deadLabelProblem(expr *parser.PromQLExpr, dl source.DeadLabel) Problem {
	return Problem{
		Anchor:   AnchorAfter,
		Lines:    expr.Value.Pos.Lines(),
		Reporter: c.Reporter(),
		Summary:  dl.Kind.String(),
		Details:  "",
		Diagnostics: []diags.Diagnostic{
			{
				Pos:         expr.Value.Pos,
				Expr:        expr.Query().Expr,
				FirstColumn: int(dl.UsageFragment.Start) + 1,
				LastColumn:  int(dl.UsageFragment.End),
				Message:     dl.Reason,
				Kind:        diags.Issue,
			},
			{
				Pos:         expr.Value.Pos,
				Expr:        expr.Query().Expr,
				FirstColumn: int(dl.LabelFragment.Start) + 1,
				LastColumn:  int(dl.LabelFragment.End),
				Message:     dl.LabelReason,
				Kind:        diags.Context,
			},
		},
		Severity: Warning,
	}
}
