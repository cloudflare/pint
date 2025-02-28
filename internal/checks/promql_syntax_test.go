package checks_test

import (
	"testing"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/output"
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
		/* FIXME this test rendomly fails because promql error has empty position.
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
						Summary:  "PromQL syntax error",
						Details:  checks.SyntaxCheckDetails,
						Severity: checks.Fatal,
						Diagnostics: []output.Diagnostic{
							{
								Line:        2,
								FirstColumn: 9,
								LastColumn:  12,
								Message:     "no arguments for aggregate expression provided",
							},
						},
					},
				}
			},
		},
		*/
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
						Summary:  "PromQL syntax error",
						Details:  checks.SyntaxCheckDetails,
						Severity: checks.Fatal,
						Diagnostics: []output.Diagnostic{
							{
								Line:        2,
								FirstColumn: 20,
								LastColumn:  20,
								Message:     "unclosed left parenthesis",
							},
						},
					},
				}
			},
		},
	}
	runTests(t, testCases)
}
