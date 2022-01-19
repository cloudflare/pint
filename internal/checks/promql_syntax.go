package checks

import (
	"context"
	"fmt"

	"github.com/cloudflare/pint/internal/parser"
)

const (
	SyntaxCheckName = "promql/syntax"
)

func NewSyntaxCheck() SyntaxCheck {
	return SyntaxCheck{}
}

type SyntaxCheck struct {
}

func (c SyntaxCheck) String() string {
	return SyntaxCheckName
}

func (c SyntaxCheck) Reporter() string {
	return SyntaxCheckName
}

func (c SyntaxCheck) Check(ctx context.Context, rule parser.Rule) (problems []Problem) {
	q := rule.Expr()
	if q.SyntaxError != nil {
		problems = append(problems, Problem{
			Fragment: q.Value.Value,
			Lines:    q.Value.Position.Lines,
			Reporter: c.Reporter(),
			Text:     fmt.Sprintf("syntax error: %s", q.SyntaxError),
			Severity: Fatal,
		})
	}
	return
}
