package checks_test

import (
	"testing"

	"github.com/cloudflare/pint/internal/checks"
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
						Text:     "`vector(0)` is dead code because this query always evaluates to `0 > 0` which is not possible, so it will never return anything.",
						Severity: checks.Warning,
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
						Text:     "This is dead code because this query always evaluates to `0 > 0` which is not possible, so it will never return anything.",
						Severity: checks.Warning,
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
						Text:     "`vector(0)` is dead code because this query always evaluates to `0 > 0` which is not possible, so it will never return anything.",
						Severity: checks.Warning,
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
						Text:     "`vector(0)` is dead code because the right hand side will never be matched because it doesn't have the `job` label while the left hand side will.",
						Severity: checks.Warning,
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
						Text:     "`sum(foo)` is dead code because the right hand side will never be matched because it doesn't have the `job` label while the left hand side will.",
						Severity: checks.Warning,
					},
				}
			},
		},
	}

	runTests(t, testCases)
}
