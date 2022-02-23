package checks_test

import (
	"testing"

	"github.com/cloudflare/pint/internal/checks"
)

func TestRegexpCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "ignores rules with syntax errors",
			content:     "- alert: foo\n  expr: sum(foo) without(\n",
			checker:     checks.NewRegexpCheck(),
		},
		{
			description: "static match",
			content:     "- record: foo\n  expr: foo{job=\"bar\"}\n",
			checker:     checks.NewRegexpCheck(),
		},
		{
			description: "valid regexp",
			content:     "- record: foo\n  expr: foo{job=~\"bar.+\"}\n",
			checker:     checks.NewRegexpCheck(),
		},
		{
			description: "valid regexp",
			content:     "- record: foo\n  expr: foo{job!~\"(.*)\"}\n",
			checker:     checks.NewRegexpCheck(),
		},
		{
			description: "valid regexp",
			content:     "- record: foo\n  expr: foo{job=~\"a|b|c\"}\n",
			checker:     checks.NewRegexpCheck(),
		},
		{
			description: "unnecessary regexp",
			content:     "- record: foo\n  expr: foo{job=~\"bar\"}\n",
			checker:     checks.NewRegexpCheck(),
			problems: []checks.Problem{
				{
					Fragment: `foo{job=~"bar"}`,
					Lines:    []int{2},
					Reporter: "promql/regexp",
					Text:     `unnecessary regexp match on static string job=~"bar", use job="bar" instead`,
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "unnecessary negative regexp",
			content:     "- record: foo\n  expr: foo{job!~\"bar\"}\n",
			checker:     checks.NewRegexpCheck(),
			problems: []checks.Problem{
				{
					Fragment: `foo{job!~"bar"}`,
					Lines:    []int{2},
					Reporter: "promql/regexp",
					Text:     `unnecessary regexp match on static string job!~"bar", use job!="bar" instead`,
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "empty regexp",
			content:     "- record: foo\n  expr: foo{job=~\"\"}\n",
			checker:     checks.NewRegexpCheck(),
			problems: []checks.Problem{
				{
					Fragment: `foo{job=~""}`,
					Lines:    []int{2},
					Reporter: "promql/regexp",
					Text:     `unnecessary regexp match on static string job=~"", use job="" instead`,
					Severity: checks.Warning,
				},
			},
		},
	}
	runTests(t, testCases)
}
