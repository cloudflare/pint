package checks_test

import (
	"testing"

	"github.com/cloudflare/pint/internal/checks"
)

func newAlertsForCheck(_ string) checks.RuleChecker {
	return checks.NewAlertsForCheck()
}

func TestAlertsForCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "ignores rules with syntax errors",
			content:     "- alert: foo\n  expr: sum(foo) without(\n",
			checker:     newAlertsForCheck,
			problems:    noProblems,
		},
		{
			description: "ignores recording rules",
			content:     "- record: foo\n  expr: sum(foo) without(job)\n",
			checker:     newAlertsForCheck,
			problems:    noProblems,
		},
		{
			description: "invalid for value",
			content:     "- alert: foo\n  expr: foo\n  for: abc\n",
			checker:     newAlertsForCheck,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "abc",
						Lines:    []int{3},
						Reporter: "alerts/for",
						Text:     `invalid duration: not a valid duration string: "abc"`,
						Severity: checks.Bug,
					},
				}
			},
		},
		{
			description: "negative for value",
			content:     "- alert: foo\n  expr: foo\n  for: -5m\n",
			checker:     newAlertsForCheck,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "-5m",
						Lines:    []int{3},
						Reporter: "alerts/for",
						Text:     `invalid duration: not a valid duration string: "-5m"`,
						Severity: checks.Bug,
					},
				}
			},
		},
		{
			description: "default for value",
			content:     "- alert: foo\n  expr: foo\n  for: 0h\n",
			checker:     newAlertsForCheck,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "0h",
						Lines:    []int{3},
						Reporter: "alerts/for",
						Text:     `"0h" is the default value of "for", consider removing this line`,
						Severity: checks.Information,
					},
				}
			},
		},
	}
	runTests(t, testCases)
}
