package checks_test

import (
	"testing"

	"github.com/cloudflare/pint/internal/checks"
)

func TestAnnotationCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "ignores recording rules",
			content:     "- record: foo\n  expr: sum(foo) without(\n",
			checker: func(uri string) checks.RuleChecker {
				return checks.NewAnnotationCheck("severity", checks.MustTemplatedRegexp("critical"), true, checks.Warning)
			},
			problems: noProblems,
		},
		{
			description: "doesn't ignore rules with syntax errors",
			content:     "- alert: foo\n  expr: sum(foo) without(\n",
			checker: func(uri string) checks.RuleChecker {
				return checks.NewAnnotationCheck("severity", checks.MustTemplatedRegexp("critical"), true, checks.Warning)
			},
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "alert: foo",
						Lines:    []int{1, 2},
						Reporter: checks.AnnotationCheckName,
						Text:     "severity annotation is required",
						Severity: checks.Warning,
					},
				}
			},
		},
		{
			description: "no annotations / required",
			content:     "- alert: foo\n  expr: sum(foo)\n",
			checker: func(uri string) checks.RuleChecker {
				return checks.NewAnnotationCheck("severity", checks.MustTemplatedRegexp("critical"), true, checks.Warning)
			},
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "alert: foo",
						Lines:    []int{1, 2},
						Reporter: checks.AnnotationCheckName,
						Text:     "severity annotation is required",
						Severity: checks.Warning,
					},
				}
			},
		},
		{
			description: "no annotations / not required",
			content:     "- alert: foo\n  expr: sum(foo)\n",
			checker: func(uri string) checks.RuleChecker {
				return checks.NewAnnotationCheck("severity", checks.MustTemplatedRegexp("critical"), false, checks.Warning)
			},
			problems: noProblems,
		},
		{
			description: "missing annotation / required",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    foo: bar\n",
			checker: func(uri string) checks.RuleChecker {
				return checks.NewAnnotationCheck("severity", checks.MustTemplatedRegexp("critical"), true, checks.Warning)
			},
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "annotations:",
						Lines:    []int{3, 4},
						Reporter: checks.AnnotationCheckName,
						Text:     "severity annotation is required",
						Severity: checks.Warning,
					},
				}
			},
		},
		{
			description: "missing annotation / not required",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    foo: bar\n",
			checker: func(uri string) checks.RuleChecker {
				return checks.NewAnnotationCheck("severity", checks.MustTemplatedRegexp("critical"), false, checks.Warning)
			},
			problems: noProblems,
		},
		{
			description: "wrong annotation value / required",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    severity: bar\n",
			checker: func(uri string) checks.RuleChecker {
				return checks.NewAnnotationCheck("severity", checks.MustTemplatedRegexp("critical"), true, checks.Warning)
			},
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "severity: bar",
						Lines:    []int{4},
						Reporter: checks.AnnotationCheckName,
						Text:     `severity annotation value must match "^critical$"`,
						Severity: checks.Warning,
					},
				}
			},
		},
		{
			description: "wrong annotation value / not required",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    severity: bar\n",
			checker: func(uri string) checks.RuleChecker {
				return checks.NewAnnotationCheck("severity", checks.MustTemplatedRegexp("critical"), false, checks.Warning)
			},
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "severity: bar",
						Lines:    []int{4},
						Reporter: checks.AnnotationCheckName,
						Text:     `severity annotation value must match "^critical$"`,
						Severity: checks.Warning,
					},
				}
			},
		},
		{
			description: "valid annotation / required",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    severity: info\n",
			checker: func(uri string) checks.RuleChecker {
				return checks.NewAnnotationCheck("severity", checks.MustTemplatedRegexp("critical|info|debug"), true, checks.Warning)
			},
			problems: noProblems,
		},
		{
			description: "valid annotation / not required",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    severity: info\n",
			checker: func(uri string) checks.RuleChecker {
				return checks.NewAnnotationCheck("severity", checks.MustTemplatedRegexp("critical|info|debug"), false, checks.Warning)
			},
			problems: noProblems,
		},
		{
			description: "templated annotation value / passing",
			content:     "- alert: foo\n  expr: sum(foo)\n  for: 5m\n  annotations:\n    for: 5m\n",
			checker: func(uri string) checks.RuleChecker {
				return checks.NewAnnotationCheck("for", checks.MustTemplatedRegexp("{{ $for }}"), true, checks.Bug)
			},
			problems: noProblems,
		},
		{
			description: "templated annotation value / passing",
			content:     "- alert: foo\n  expr: sum(foo)\n  for: 5m\n  annotations:\n    for: 4m\n",
			checker: func(uri string) checks.RuleChecker {
				return checks.NewAnnotationCheck("for", checks.MustTemplatedRegexp("{{ $for }}"), true, checks.Bug)
			},
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "for: 4m",
						Lines:    []int{5},
						Reporter: checks.AnnotationCheckName,
						Text:     `for annotation value must match "^{{ $for }}$"`,
						Severity: checks.Bug,
					},
				}
			},
		},
	}
	runTests(t, testCases)
}
