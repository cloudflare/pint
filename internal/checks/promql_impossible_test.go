package checks_test

import (
	"testing"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
)

func newImpossibleCheck(_ *promapi.FailoverGroup) checks.RuleChecker {
	return checks.NewImpossibleCheck()
}

func TestImpossibleCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "ignores rules with syntax errors",
			content:     "- record: foo\n  expr: sum(foo) without(\n",
			checker:     newImpossibleCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
		},
		{
			description: "vector(0) > 0",
			content: `
- alert: Foo
  expr: ((( group(vector(0)) ))) > 0
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 3,
							Last:  3,
						},
						Reporter: checks.ImpossibleCheckName,
						Summary:  "dead code in query",
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Line:        3,
								FirstColumn: 19,
								LastColumn:  27,
								Message:     "this query always evaluates to `0 > 0` which is not possible, so it will never return anything",
							},
						},
					},
				}
			},
		},
		{
			description: "0 > 0",
			content: `
- alert: Foo
  expr: 0 > bool 0
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 3,
							Last:  3,
						},
						Reporter: checks.ImpossibleCheckName,
						Summary:  "dead code in query",
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Line:        3,
								FirstColumn: 9,
								LastColumn:  9,
								Message:     "this query always evaluates to `0 > 0` which is not possible, so it will never return anything",
							},
						},
					},
				}
			},
		},
		{
			description: "sum(foo or vector(0)) > 0",
			content: `
- alert: Foo
  expr: sum(foo or vector(0)) > 0
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 3,
							Last:  3,
						},
						Reporter: checks.ImpossibleCheckName,
						Summary:  "dead code in query",
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Line:        3,
								FirstColumn: 20,
								LastColumn:  28,
								Message:     "this query always evaluates to `0 > 0` which is not possible, so it will never return anything",
							},
						},
					},
				}
			},
		},
		{
			description: "foo{job=bar} unless vector(0)",
			content: `
- alert: Foo
  expr: foo{job="bar"} unless vector(0)
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 3,
							Last:  3,
						},
						Reporter: checks.ImpossibleCheckName,
						Summary:  "dead code in query",
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Line:        3,
								FirstColumn: 31,
								LastColumn:  39,
								Message:     "the right hand side will never be matched because it doesn't have the `job` label while the left hand side will",
							},
						},
					},
				}
			},
		},
		{
			description: "foo{job=bar} unless sum(foo)",
			content: `
- alert: Foo
  expr: foo{job="bar"} unless sum(foo)
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 3,
							Last:  3,
						},
						Reporter: checks.ImpossibleCheckName,
						Summary:  "dead code in query",
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Line:        3,
								FirstColumn: 35, // FIXME 31
								LastColumn:  37, // FIXME 38
								Message:     "the right hand side will never be matched because it doesn't have the `job` label while the left hand side will",
							},
						},
					},
				}
			},
		},
	}

	runTests(t, testCases)
}
