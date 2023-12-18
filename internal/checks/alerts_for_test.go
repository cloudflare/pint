package checks_test

import (
	"testing"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
)

func newAlertsForCheck(_ *promapi.FailoverGroup) checks.RuleChecker {
	return checks.NewAlertsForCheck()
}

func TestAlertsForCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "ignores rules with syntax errors",
			content:     "- alert: foo\n  expr: sum(foo) without(\n",
			checker:     newAlertsForCheck,
			prometheus:  noProm,
			problems:    noProblems,
		},
		{
			description: "ignores recording rules",
			content:     "- record: foo\n  expr: sum(foo) without(job)\n",
			checker:     newAlertsForCheck,
			prometheus:  noProm,
			problems:    noProblems,
		},
		{
			description: "invalid for value",
			content:     "- alert: foo\n  expr: foo\n  for: abc\n",
			checker:     newAlertsForCheck,
			prometheus:  noProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 3,
							Last:  3,
						},
						Reporter: "alerts/for",
						Text:     `invalid duration: not a valid duration string: "abc"`,
						Details:  checks.AlertForCheckDurationHelp,
						Severity: checks.Bug,
					},
				}
			},
		},
		{
			description: "negative for value",
			content:     "- alert: foo\n  expr: foo\n  for: -5m\n",
			checker:     newAlertsForCheck,
			prometheus:  noProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 3,
							Last:  3,
						},
						Reporter: "alerts/for",
						Text:     `invalid duration: not a valid duration string: "-5m"`,
						Details:  checks.AlertForCheckDurationHelp,
						Severity: checks.Bug,
					},
				}
			},
		},
		{
			description: "default for value",
			content:     "- alert: foo\n  expr: foo\n  for: 0h\n",
			checker:     newAlertsForCheck,
			prometheus:  noProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 3,
							Last:  3,
						},
						Reporter: "alerts/for",
						Text:     "`0h` is the default value of `for`, consider removing this redundant line.",
						Severity: checks.Information,
					},
				}
			},
		},
		{
			description: "invalid keep_firing_for value",
			content:     "- alert: foo\n  expr: foo\n  keep_firing_for: abc\n",
			checker:     newAlertsForCheck,
			prometheus:  noProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 3,
							Last:  3,
						},
						Reporter: "alerts/for",
						Text:     `invalid duration: not a valid duration string: "abc"`,
						Details:  checks.AlertForCheckDurationHelp,
						Severity: checks.Bug,
					},
				}
			},
		},
		{
			description: "negative keep_firing_for value",
			content:     "- alert: foo\n  expr: foo\n  keep_firing_for: -5m\n",
			checker:     newAlertsForCheck,
			prometheus:  noProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 3,
							Last:  3,
						},
						Reporter: "alerts/for",
						Text:     `invalid duration: not a valid duration string: "-5m"`,
						Details:  checks.AlertForCheckDurationHelp,
						Severity: checks.Bug,
					},
				}
			},
		},
		{
			description: "default for value",
			content:     "- alert: foo\n  expr: foo\n  keep_firing_for: 0h\n",
			checker:     newAlertsForCheck,
			prometheus:  noProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 3,
							Last:  3,
						},
						Reporter: "alerts/for",
						Text:     "`0h` is the default value of `keep_firing_for`, consider removing this redundant line.",
						Severity: checks.Information,
					},
				}
			},
		},
	}
	runTests(t, testCases)
}
