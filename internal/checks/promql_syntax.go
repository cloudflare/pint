package checks

import (
	"context"
	"errors"

	promParser "github.com/prometheus/prometheus/promql/parser"

	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/parser"
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
		Online:        false,
		AlwaysEnabled: false,
	}
}

func (c SyntaxCheck) String() string {
	return SyntaxCheckName
}

func (c SyntaxCheck) Reporter() string {
	return SyntaxCheckName
}

func (c SyntaxCheck) Check(_ context.Context, _ discovery.Path, rule parser.Rule, _ []discovery.Entry) (problems []Problem) {
	expr := rule.Expr()
	if expr.SyntaxError != nil {
		diag := output.Diagnostic{
			Message:     expr.SyntaxError.Error(),
			Line:        expr.Value.Lines.First,
			FirstColumn: expr.Value.Column,
			LastColumn:  nodeLastColumn(expr.Value),
		}

		var perrs promParser.ParseErrors
		ok := errors.As(expr.SyntaxError, &perrs)
		if ok {
			for _, perr := range perrs { // Use only the last error.
				start := expr.Value.Column + int(perr.PositionRange.Start)
				end := expr.Value.Column + int(perr.PositionRange.End)
				if end > start {
					end--
				}
				diag = output.Diagnostic{
					Message:     perr.Err.Error(),
					Line:        expr.Value.Lines.First,
					FirstColumn: min(start, nodeLastColumn(expr.Value)),
					LastColumn:  min(end, nodeLastColumn(expr.Value)),
				}
			}
		}

		problems = append(problems, Problem{
			Lines:       expr.Value.Lines,
			Reporter:    c.Reporter(),
			Summary:     "PromQL syntax error",
			Details:     SyntaxCheckDetails,
			Diagnostics: []output.Diagnostic{diag},
			Severity:    Fatal,
		})
	}
	return problems
}
