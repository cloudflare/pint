package checks_test

import (
	"testing"

	"github.com/cloudflare/pint/internal/checks"
)

func TestLabelCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "doesn't ignore rules with syntax errors",
			content:     "- record: foo\n  expr: sum(foo) without(\n",
			checker: func(s string) checks.RuleChecker {
				return checks.NewLabelCheck("severity", checks.MustTemplatedRegexp("critical"), true, checks.Warning)
			},
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "record: foo",
						Lines:    []int{1, 2},
						Reporter: "rule/label",
						Text:     "severity label is required",
						Severity: checks.Warning,
					},
				}
			},
		},
		{
			description: "no labels in recording rule / required",
			content:     "- record: foo\n  expr: rate(foo[1m])\n",
			checker: func(s string) checks.RuleChecker {
				return checks.NewLabelCheck("severity", checks.MustTemplatedRegexp("critical"), true, checks.Warning)
			},
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "record: foo",
						Lines:    []int{1, 2},
						Reporter: "rule/label",
						Text:     "severity label is required",
						Severity: checks.Warning,
					},
				}
			},
		},
		{
			description: "no labels in recording rule / not required",
			content:     "- record: foo\n  expr: rate(foo[1m])\n",
			checker: func(s string) checks.RuleChecker {
				return checks.NewLabelCheck("severity", checks.MustTemplatedRegexp("critical"), false, checks.Warning)
			},
			problems: noProblems,
		},
		{
			description: "missing label in recording rule / required",
			content:     "- record: foo\n  expr: rate(foo[1m])\n  labels:\n    foo: bar\n",
			checker: func(s string) checks.RuleChecker {
				return checks.NewLabelCheck("severity", checks.MustTemplatedRegexp("critical"), true, checks.Warning)
			},
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "labels:",
						Lines:    []int{3, 4},
						Reporter: "rule/label",
						Text:     "severity label is required",
						Severity: checks.Warning,
					},
				}
			},
		},
		{
			description: "missing label in recording rule / not required",
			content:     "- record: foo\n  expr: rate(foo[1m])\n  labels:\n    foo: bar\n",
			checker: func(s string) checks.RuleChecker {
				return checks.NewLabelCheck("severity", checks.MustTemplatedRegexp("critical"), false, checks.Warning)
			},
			problems: noProblems,
		},
		{
			description: "invalid value in recording rule / required",
			content:     "- record: foo\n  expr: rate(foo[1m])\n  labels:\n    severity: warning\n",
			checker: func(s string) checks.RuleChecker {
				return checks.NewLabelCheck("severity", checks.MustTemplatedRegexp("critical"), true, checks.Warning)
			},
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "severity: warning",
						Lines:    []int{4},
						Reporter: "rule/label",
						Text:     `severity label value must match "^critical$"`,
						Severity: checks.Warning,
					},
				}
			},
		},
		{
			description: "invalid value in recording rule / not required",
			content:     "- record: foo\n  expr: rate(foo[1m])\n  labels:\n    severity: warning\n",
			checker: func(s string) checks.RuleChecker {
				return checks.NewLabelCheck("severity", checks.MustTemplatedRegexp("critical"), false, checks.Warning)
			},
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "severity: warning",
						Lines:    []int{4},
						Reporter: "rule/label",
						Text:     `severity label value must match "^critical$"`,
						Severity: checks.Warning,
					},
				}
			},
		},
		{
			description: "typo in recording rule / required",
			content:     "- record: foo\n  expr: rate(foo[1m])\n  labels:\n    priority: 2a\n",
			checker: func(s string) checks.RuleChecker {
				return checks.NewLabelCheck("priority", checks.MustTemplatedRegexp("(1|2|3)"), true, checks.Warning)
			},
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "priority: 2a",
						Lines:    []int{4},
						Reporter: "rule/label",
						Text:     `priority label value must match "^(1|2|3)$"`,
						Severity: checks.Warning,
					},
				}
			},
		},
		{
			description: "typo in recording rule / not required",
			content:     "- record: foo\n  expr: rate(foo[1m])\n  labels:\n    priority: 2a\n",
			checker: func(s string) checks.RuleChecker {
				return checks.NewLabelCheck("priority", checks.MustTemplatedRegexp("(1|2|3)"), false, checks.Warning)
			},
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "priority: 2a",
						Lines:    []int{4},
						Reporter: "rule/label",
						Text:     `priority label value must match "^(1|2|3)$"`,
						Severity: checks.Warning,
					},
				}
			},
		},
		{
			description: "no labels in alerting rule / required",
			content:     "- alert: foo\n  expr: rate(foo[1m])\n",
			checker: func(s string) checks.RuleChecker {
				return checks.NewLabelCheck("severity", checks.MustTemplatedRegexp("critical"), true, checks.Warning)
			},
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "alert: foo",
						Lines:    []int{1, 2},
						Reporter: "rule/label",
						Text:     "severity label is required",
						Severity: checks.Warning,
					},
				}
			},
		},
		{
			description: "no labels in alerting rule / not required",
			content:     "- alert: foo\n  expr: rate(foo[1m])\n",
			checker: func(s string) checks.RuleChecker {
				return checks.NewLabelCheck("severity", checks.MustTemplatedRegexp("critical"), false, checks.Warning)
			},
			problems: noProblems,
		},
		{
			description: "missing label in alerting rule / required",
			content:     "- alert: foo\n  expr: rate(foo[1m])\n  labels:\n    foo: bar\n",
			checker: func(s string) checks.RuleChecker {
				return checks.NewLabelCheck("severity", checks.MustTemplatedRegexp("critical"), true, checks.Warning)
			},
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "labels:",
						Lines:    []int{3, 4},
						Reporter: "rule/label",
						Text:     "severity label is required",
						Severity: checks.Warning,
					},
				}
			},
		},
		{
			description: "missing label in alerting rule / not required",
			content:     "- alert: foo\n  expr: rate(foo[1m])\n  labels:\n    foo: bar\n",
			checker: func(s string) checks.RuleChecker {
				return checks.NewLabelCheck("severity", checks.MustTemplatedRegexp("critical"), false, checks.Warning)
			},
			problems: noProblems,
		},
		{
			description: "invalid value in alerting rule / required",
			content:     "- alert: foo\n  expr: rate(foo[1m])\n  labels:\n    severity: warning\n",
			checker: func(s string) checks.RuleChecker {
				return checks.NewLabelCheck("severity", checks.MustTemplatedRegexp("critical|info"), true, checks.Warning)
			},
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "severity: warning",
						Lines:    []int{4},
						Reporter: "rule/label",
						Text:     `severity label value must match "^critical|info$"`,
						Severity: checks.Warning,
					},
				}
			},
		},
		{
			description: "invalid value in alerting rule / not required",
			content:     "- alert: foo\n  expr: rate(foo[1m])\n  labels:\n    severity: warning\n",
			checker: func(s string) checks.RuleChecker {
				return checks.NewLabelCheck("severity", checks.MustTemplatedRegexp("critical|info"), false, checks.Warning)
			},
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "severity: warning",
						Lines:    []int{4},
						Reporter: "rule/label",
						Text:     `severity label value must match "^critical|info$"`,
						Severity: checks.Warning,
					},
				}
			},
		},
		{
			description: "valid recording rule / required",
			content:     "- record: foo\n  expr: rate(foo[1m])\n  labels:\n    severity: critical\n",
			checker: func(s string) checks.RuleChecker {
				return checks.NewLabelCheck("severity", checks.MustTemplatedRegexp("critical|info"), true, checks.Warning)
			},
			problems: noProblems,
		},
		{
			description: "valid recording rule / not required",
			content:     "- record: foo\n  expr: rate(foo[1m])\n  labels:\n    severity: critical\n",
			checker: func(s string) checks.RuleChecker {
				return checks.NewLabelCheck("severity", checks.MustTemplatedRegexp("critical|info"), false, checks.Warning)
			},
			problems: noProblems,
		},
		{
			description: "valid alerting rule / required",
			content:     "- alert: foo\n  expr: rate(foo[1m])\n  labels:\n    severity: info\n",
			checker: func(s string) checks.RuleChecker {
				return checks.NewLabelCheck("severity", checks.MustTemplatedRegexp("critical|info"), true, checks.Warning)
			},
			problems: noProblems,
		},
		{
			description: "valid alerting rule / not required",
			content:     "- alert: foo\n  expr: rate(foo[1m])\n  labels:\n    severity: info\n",
			checker: func(s string) checks.RuleChecker {
				return checks.NewLabelCheck("severity", checks.MustTemplatedRegexp("critical|info"), false, checks.Warning)
			},
			problems: noProblems,
		},
		{
			description: "templated label value / passing",
			content:     "- alert: foo\n  expr: sum(foo)\n  for: 5m\n  labels:\n    for: 'must wait 5m to fire'\n",
			checker: func(s string) checks.RuleChecker {
				return checks.NewLabelCheck("for", checks.MustTemplatedRegexp("must wait {{$for}} to fire"), true, checks.Warning)
			},
			problems: noProblems,
		},
		{
			description: "templated label value / not passing",
			content:     "- alert: foo\n  expr: sum(foo)\n  for: 4m\n  labels:\n    for: 'must wait 5m to fire'\n",
			checker: func(s string) checks.RuleChecker {
				return checks.NewLabelCheck("for", checks.MustTemplatedRegexp("must wait {{$for}} to fire"), true, checks.Warning)
			},
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "for: must wait 5m to fire",
						Lines:    []int{5},
						Reporter: "rule/label",
						Text:     `for label value must match "^must wait {{$for}} to fire$"`,
						Severity: checks.Warning,
					},
				}
			},
		},
	}
	runTests(t, testCases)
}
