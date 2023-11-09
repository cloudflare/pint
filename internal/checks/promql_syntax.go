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
	return CheckMeta{IsOnline: false}
}

func (c SyntaxCheck) String() string {
	return SyntaxCheckName
}

func (c SyntaxCheck) Reporter() string {
	return SyntaxCheckName
}

func (c SyntaxCheck) Check(_ context.Context, _ string, rule parser.Rule, _ []discovery.Entry) (problems []Problem) {
	q := rule.Expr()
	if q.SyntaxError != nil {
		problems = append(problems, Problem{
			Fragment: q.Value.Value,
			Lines:    q.Value.Position.Lines,
			Reporter: c.Reporter(),
			Text:     fmt.Sprintf("Prometheus failed to parse the query with this PromQL error: %s.", q.SyntaxError),
			Details:  SyntaxCheckDetails,
			Severity: Fatal,
		})
	}
	return problems
}
