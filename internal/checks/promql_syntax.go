package checks

import (
	"context"
	"errors"

	promParser "github.com/prometheus/prometheus/promql/parser"

	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
)

const (
	SyntaxCheckName    = "promql/syntax"
	SyntaxCheckDetails = "[Click here](https://prometheus.io/docs/prometheus/latest/querying/basics/) for PromQL documentation."
)

func NewSyntaxCheck() SyntaxCheck {
	return SyntaxCheck{}
}

type SyntaxCheck struct{}

func (c SyntaxCheck) Meta() CheckMeta {
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

func (c SyntaxCheck) String() string {
	return SyntaxCheckName
}

func (c SyntaxCheck) Reporter() string {
	return SyntaxCheckName
}

func (c SyntaxCheck) Check(_ context.Context, entry *discovery.Entry, _ []*discovery.Entry) (problems []Problem) {
	expr := entry.Rule.Expr()
	if err := expr.SyntaxError(); err != nil {
		// This is pretty much dead code because PromQL always returns ParseErrors.
		// The only exception is if it panics and recover gives as a generic error,
		// which would require a bug in the parser itself.
		diag := diags.Diagnostic{
			Message:     err.Error(),
			Pos:         expr.Value.Pos,
			Expr:        nil,
			FirstColumn: 1,
			LastColumn:  len(expr.Value.Value),
			Kind:        diags.Issue,
		}

		perrs, ok := errors.AsType[promParser.ParseErrors](err)
		if ok {
			for _, perr := range perrs { // Use only the last error.
				diag = diags.Diagnostic{
					Message:     perr.Err.Error(),
					Pos:         expr.Value.Pos,
					Expr:        nil,
					FirstColumn: int(perr.PositionRange.Start) + 1,
					LastColumn:  int(perr.PositionRange.End),
					Kind:        diags.Issue,
				}
			}
		}

		problems = append(problems, Problem{
			Anchor:      AnchorAfter,
			Lines:       expr.Value.Pos.Lines(),
			Reporter:    c.Reporter(),
			Summary:     "PromQL syntax error",
			Details:     SyntaxCheckDetails,
			Diagnostics: []diags.Diagnostic{diag},
			Severity:    Fatal,
		})
	}
	return problems
}
