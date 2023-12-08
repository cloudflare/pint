package checks_test

import (
	"testing"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/promapi"
)

func TestAnnotationCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "ignores recording rules",
			content:     "- record: foo\n  expr: sum(foo) without(\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("severity"), checks.MustTemplatedRegexp("critical"), true, checks.Warning)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "doesn't ignore rules with syntax errors",
			content:     "- alert: foo\n  expr: sum(foo) without(\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("severity"), checks.MustTemplatedRegexp("critical"), true, checks.Warning)
			},
			prometheus: noProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines:    []int{1, 2},
						Reporter: checks.AnnotationCheckName,
						Text:     "`severity` annotation is required.",
						Severity: checks.Warning,
					},
				}
			},
		},
		{
			description: "no annotations / required",
			content:     "- alert: foo\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("severity"), checks.MustTemplatedRegexp("critical"), true, checks.Warning)
			},
			prometheus: noProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines:    []int{1, 2},
						Reporter: checks.AnnotationCheckName,
						Text:     "`severity` annotation is required.",
						Severity: checks.Warning,
					},
				}
			},
		},
		{
			description: "no annotations / not required",
			content:     "- alert: foo\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("severity"), checks.MustTemplatedRegexp("critical"), false, checks.Warning)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "missing annotation / required",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    foo: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("severity"), checks.MustTemplatedRegexp("critical"), true, checks.Warning)
			},
			prometheus: noProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines:    []int{3, 4},
						Reporter: checks.AnnotationCheckName,
						Text:     "`severity` annotation is required.",
						Severity: checks.Warning,
					},
				}
			},
		},
		{
			description: "missing annotation / not required",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    foo: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("severity"), checks.MustTemplatedRegexp("critical"), false, checks.Warning)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "wrong annotation value / required",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    severity: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("severity"), checks.MustTemplatedRegexp("critical"), true, checks.Warning)
			},
			prometheus: noProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines:    []int{4},
						Reporter: checks.AnnotationCheckName,
						Text:     "`severity` annotation value must match `^critical$`.",
						Severity: checks.Warning,
					},
				}
			},
		},
		{
			description: "wrong annotation value / not required",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    severity: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("severity"), checks.MustTemplatedRegexp("critical"), false, checks.Warning)
			},
			prometheus: noProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines:    []int{4},
						Reporter: checks.AnnotationCheckName,
						Text:     "`severity` annotation value must match `^critical$`.",
						Severity: checks.Warning,
					},
				}
			},
		},
		{
			description: "valid annotation / required",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    severity: info\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("severity"), checks.MustTemplatedRegexp("critical|info|debug"), true, checks.Warning)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "valid annotation / not required",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    severity: info\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("severity"), checks.MustTemplatedRegexp("critical|info|debug"), false, checks.Warning)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "templated annotation value / passing",
			content:     "- alert: foo\n  expr: sum(foo)\n  for: 5m\n  annotations:\n    for: 5m\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("for"), checks.MustTemplatedRegexp("{{ $for }}"), true, checks.Bug)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "templated annotation value / passing",
			content:     "- alert: foo\n  expr: sum(foo)\n  for: 5m\n  annotations:\n    for: 4m\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("for"), checks.MustTemplatedRegexp("{{ $for }}"), true, checks.Bug)
			},
			prometheus: noProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines:    []int{5},
						Reporter: checks.AnnotationCheckName,
						Text:     "`for` annotation value must match `^{{ $for }}$`.",
						Severity: checks.Bug,
					},
				}
			},
		},
		{
			description: "valid annotation key regex / required",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    annotation_1: info\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("annotation_.*"), checks.MustTemplatedRegexp("critical|info|debug"), true, checks.Warning)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "valid annotation key regex / not required",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    annotation_1: info\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("annotation_.*"), checks.MustTemplatedRegexp("critical|info|debug"), false, checks.Warning)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "wrong annotation key regex value / required",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    annotation_1: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("annotation_.*"), checks.MustTemplatedRegexp("critical"), true, checks.Warning)
			},
			prometheus: noProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines:    []int{4},
						Reporter: checks.AnnotationCheckName,
						Text:     "`annotation_.*` annotation value must match `^critical$`.",
						Severity: checks.Warning,
					},
				}
			},
		},
		{
			description: "wrong annotation key regex value / not required",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    annotation_1: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("annotation_.*"), checks.MustTemplatedRegexp("critical"), false, checks.Warning)
			},
			prometheus: noProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines:    []int{4},
						Reporter: checks.AnnotationCheckName,
						Text:     "`annotation_.*` annotation value must match `^critical$`.",
						Severity: checks.Warning,
					},
				}
			},
		},
	}
	runTests(t, testCases)
}
