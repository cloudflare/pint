package checks_test

import (
	"testing"

	"github.com/cloudflare/pint/internal/checks"
)

func TestSyntaxCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "valid recording rule",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker:     checks.NewSyntaxCheck(),
		},
		{
			description: "valid alerting rule",
			content:     "- alert: foo\n  expr: sum(foo)\n",
			checker:     checks.NewSyntaxCheck(),
		},
		{
			description: "no arguments for aggregate expression provided",
			content:     "- record: foo\n  expr: sum(\n",
			checker:     checks.NewSyntaxCheck(),
			problems: []checks.Problem{
				{
					Fragment: "sum(",
					Lines:    []int{2},
					Reporter: "promql/syntax",
					Text:     "syntax error: no arguments for aggregate expression provided",
					Severity: checks.Fatal,
				},
			},
		},
		{
			description: "unclosed left parenthesis",
			content:     "- record: foo\n  expr: sum(foo) by(",
			checker:     checks.NewSyntaxCheck(),
			problems: []checks.Problem{
				{
					Fragment: "sum(foo) by(",
					Lines:    []int{2},
					Reporter: "promql/syntax",
					Text:     "syntax error: unclosed left parenthesis",
					Severity: checks.Fatal,
				},
			},
		},
	}
	runTests(t, testCases)
}
