package checks_test

import (
	"testing"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
)

func newSyntaxCheck(_ *promapi.FailoverGroup) checks.RuleChecker {
	return checks.NewSyntaxCheck()
}

func TestSyntaxCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "valid recording rule",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker:     newSyntaxCheck,
			prometheus:  noProm,
			problems:    noProblems,
		},
		{
			description: "valid alerting rule",
			content:     "- alert: foo\n  expr: sum(foo)\n",
			checker:     newSyntaxCheck,
			prometheus:  noProm,
			problems:    noProblems,
		},
		{
			description: "no arguments for aggregate expression provided",
			content:     "- record: foo\n  expr: sum(\n",
			checker:     newSyntaxCheck,
			prometheus:  noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "promql/syntax",
						Text:     "Prometheus failed to parse the query with this PromQL error: no arguments for aggregate expression provided.",
						Details:  checks.SyntaxCheckDetails,
						Severity: checks.Fatal,
					},
				}
			},
		},
		{
			description: "unclosed left parenthesis",
			content:     "- record: foo\n  expr: sum(foo) by(",
			checker:     newSyntaxCheck,
			prometheus:  noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "promql/syntax",
						Text:     "Prometheus failed to parse the query with this PromQL error: unclosed left parenthesis.",
						Details:  checks.SyntaxCheckDetails,
						Severity: checks.Fatal,
					},
				}
			},
		},
	}
	runTests(t, testCases)
}
