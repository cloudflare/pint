package checks

import (
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

func (c SyntaxCheck) Check(rule parser.Rule) (problems []Problem) {
	q := rule.Expr()
	if q.SyntaxError != nil {
		problems = append(problems, Problem{
			Fragment: q.Value.Value,
			Lines:    q.Value.Position.Lines,
			Reporter: SyntaxCheckName,
			Text:     fmt.Sprintf("syntax error: %s", q.SyntaxError),
			Severity: Fatal,
		})
	}
	return
}
