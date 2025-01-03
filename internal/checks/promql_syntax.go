package checks

import (
	"context"
	"fmt"

	"github.com/cloudflare/pint/internal/discovery"
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
			Text:     fmt.Sprintf("Prometheus failed to parse the query with this PromQL error: %s.", expr.SyntaxError),
			Details:  SyntaxCheckDetails,
			Severity: Fatal,
		})
	}
	return problems
}
