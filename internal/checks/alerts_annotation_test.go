package checks_test

import (
	"testing"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
)

func TestAnnotationCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "ignores recording rules",
			content:     "- record: foo\n  expr: sum(foo) without(\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical"), nil, true, checks.Warning)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "doesn't ignore rules with syntax errors",
			content:     "- alert: foo\n  expr: sum(foo) without(\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical"), nil, true, checks.Warning)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 1,
							Last:  2,
						},
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
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical"), nil, true, checks.Warning)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 1,
							Last:  2,
						},
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
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical"), nil, false, checks.Warning)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "missing annotation / required",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    foo: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical"), nil, true, checks.Warning)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 3,
							Last:  4,
						},
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
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical"), nil, false, checks.Warning)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "wrong annotation value / required",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    severity: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical"), nil, true, checks.Warning)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 4,
							Last:  4,
						},
						Reporter: checks.AnnotationCheckName,
						Text:     "`severity` annotation value `bar` must match `^critical$`.",
						Severity: checks.Warning,
					},
				}
			},
		},
		{
			description: "wrong annotation value / not required",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    severity: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical"), nil, false, checks.Warning)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 4,
							Last:  4,
						},
						Reporter: checks.AnnotationCheckName,
						Text:     "`severity` annotation value `bar` must match `^critical$`.",
						Severity: checks.Warning,
					},
				}
			},
		},
		{
			description: "valid annotation / required",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    severity: info\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical|info|debug"), nil, true, checks.Warning)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "valid annotation / not required",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    severity: info\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical|info|debug"), nil, false, checks.Warning)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "templated annotation value / passing",
			content:     "- alert: foo\n  expr: sum(foo)\n  for: 5m\n  annotations:\n    for: 5m\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("for"), nil, checks.MustTemplatedRegexp("{{ $for }}"), nil, true, checks.Bug)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "templated annotation value / passing",
			content:     "- alert: foo\n  expr: sum(foo)\n  for: 5m\n  annotations:\n    for: 4m\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("for"), nil, checks.MustTemplatedRegexp("{{ $for }}"), nil, true, checks.Bug)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 5,
							Last:  5,
						},
						Reporter: checks.AnnotationCheckName,
						Text:     "`for` annotation value `4m` must match `^{{ $for }}$`.",
						Severity: checks.Bug,
					},
				}
			},
		},
		{
			description: "valid annotation key regex / required",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    annotation_1: info\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("annotation_.*"), nil, checks.MustTemplatedRegexp("critical|info|debug"), nil, true, checks.Warning)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "valid annotation key regex / not required",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    annotation_1: info\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("annotation_.*"), nil, checks.MustTemplatedRegexp("critical|info|debug"), nil, false, checks.Warning)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "wrong annotation key regex value / required",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    annotation_1: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("annotation_.*"), nil, checks.MustTemplatedRegexp("critical"), nil, true, checks.Warning)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 4,
							Last:  4,
						},
						Reporter: checks.AnnotationCheckName,
						Text:     "`annotation_.*` annotation value `bar` must match `^critical$`.",
						Severity: checks.Warning,
					},
				}
			},
		},
		{
			description: "wrong annotation key regex value / not required",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    annotation_1: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("annotation_.*"), nil, checks.MustTemplatedRegexp("critical"), nil, false, checks.Warning)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 4,
							Last:  4,
						},
						Reporter: checks.AnnotationCheckName,
						Text:     "`annotation_.*` annotation value `bar` must match `^critical$`.",
						Severity: checks.Warning,
					},
				}
			},
		},
		{
			description: "invalid value / token / valueRe",
			content:     "- alert: foo\n  expr: rate(foo[1m])\n  annotations:\n    components: api db\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("components"), checks.MustRawTemplatedRegexp("\\w+"), checks.MustTemplatedRegexp("api|memcached"), nil, false, checks.Bug)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 4,
							Last:  4,
						},
						Reporter: checks.AnnotationCheckName,
						Text:     "`components` annotation value `db` must match `^api|memcached$`.",
						Severity: checks.Bug,
					},
				}
			},
		},
		{
			description: "invalid value / token / values",
			content:     "- alert: foo\n  expr: rate(foo[1m])\n  annotations:\n    components: api db\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(
					checks.MustTemplatedRegexp("components"),
					checks.MustRawTemplatedRegexp("\\w+"),
					nil,
					[]string{"api", "memcached", "storage", "prometheus", "kvm", "mysql", "memsql", "haproxy", "postgresql"},
					false,
					checks.Bug,
				)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 4,
							Last:  4,
						},
						Reporter: checks.AnnotationCheckName,
						Text:     "`components` annotation value `db` is not one of valid values.",
						Details:  "List of allowed values:\n\n- `api`\n- `memcached`\n- `storage`\n- `prometheus`\n- `kvm`\n- `mysql`\n\nAnd 3 other value(s).",
						Severity: checks.Bug,
					},
				}
			},
		},
	}
	runTests(t, testCases)
}
