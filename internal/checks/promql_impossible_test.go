package checks_test

import (
	"testing"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/diags"
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
						Reporter: checks.ImpossibleCheckName,
						Summary:  "dead code in query",
						Severity: checks.Warning,
						Diagnostics: []diags.Diagnostic{
							{
								Message: "this query always evaluates to `0 > 0` which is not possible, so it will never return anything",
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
						Reporter: checks.ImpossibleCheckName,
						Summary:  "dead code in query",
						Severity: checks.Warning,
						Diagnostics: []diags.Diagnostic{
							{
								Message: "this query always evaluates to `0 > 0` which is not possible, so it will never return anything",
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
						Reporter: checks.ImpossibleCheckName,
						Summary:  "dead code in query",
						Severity: checks.Warning,
						Diagnostics: []diags.Diagnostic{
							{
								Message: "this query always evaluates to `0 > 0` which is not possible, so it will never return anything",
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
						Reporter: checks.ImpossibleCheckName,
						Summary:  "dead code in query",
						Severity: checks.Warning,
						Diagnostics: []diags.Diagnostic{
							{
								Message: "The right hand side will never be matched because it doesn't have the `job` label while the left hand side will. Calling `vector()` will return a vector value with no labels.",
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
						Reporter: checks.ImpossibleCheckName,
						Summary:  "dead code in query",
						Severity: checks.Warning,
						Diagnostics: []diags.Diagnostic{
							{
								Message: "The right hand side will never be matched because it doesn't have the `job` label while the left hand side will. Query is using aggregation that removes all labels.",
							},
						},
					},
				}
			},
		},
	}

	runTests(t, testCases)
}
