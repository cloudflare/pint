package checks

import (
	"context"

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
		problems = append(problems, Problem{
			Lines:    expr.Value.Lines,
			Reporter: c.Reporter(),
			Summary:  "PromQL syntax error",
			Details:  SyntaxCheckDetails,
			Diagnostics: []output.Diagnostic{
				{
					Message:     expr.SyntaxError.Error(),
					Line:        expr.Value.Lines.First,
					FirstColumn: expr.Value.Column,
					LastColumn:  nodeLastColumn(expr.Value),
				},
			},
			Severity: Fatal,
		})
	}
	return problems
}
