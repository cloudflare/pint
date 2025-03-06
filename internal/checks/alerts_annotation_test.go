package checks_test

import (
	"testing"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/promapi"
)

func TestAnnotationCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "ignores recording rules",
			content:     "- record: foo\n  expr: sum(foo) without(\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical"), nil, true, "", checks.Warning)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "doesn't ignore rules with syntax errors",
			content:     "- alert: foo\n  expr: sum(foo) without(\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical"), nil, true, "", checks.Warning)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Reporter: checks.AnnotationCheckName,
						Summary:  "required annotation not set",
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Message: "`severity` annotation is required.",
							},
						},
					},
				}
			},
		},
		{
			description: "no annotations / required",
			content:     "- alert: foo\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical"), nil, true, "", checks.Warning)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Reporter: checks.AnnotationCheckName,
						Summary:  "required annotation not set",
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Message: "`severity` annotation is required.",
							},
						},
					},
				}
			},
		},
		{
			description: "empty annotations / required",
			content: `
- alert: foo
  expr: sum(foo)
  annotations:
    foo: bar
    severity:
    level: warning
`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("severity"), nil, nil, nil, true, "", checks.Bug)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Reporter: checks.AnnotationCheckName,
						Summary:  "required annotation not set",
						Severity: checks.Bug,
						Diagnostics: []output.Diagnostic{
							{
								Message: "`severity` annotation is required.",
							},
						},
					},
				}
			},
		},
		{
			description: "no annotations / not required",
			content:     "- alert: foo\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical"), nil, false, "", checks.Warning)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "missing annotation / required",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    foo: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical"), nil, true, "", checks.Warning)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Reporter: checks.AnnotationCheckName,
						Summary:  "required annotation not set",
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Message: "`severity` annotation is required.",
							},
						},
					},
				}
			},
		},
		{
			description: "missing annotation / not required",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    foo: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical"), nil, false, "", checks.Warning)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "wrong annotation value / required",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    severity: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical"), nil, true, "", checks.Warning)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Reporter: checks.AnnotationCheckName,
						Summary:  "invalid annotation value",
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Message: "`severity` annotation value `bar` must match `^critical$`.",
							},
						},
					},
				}
			},
		},
		{
			description: "wrong annotation value / not required",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    severity: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical"), nil, false, "", checks.Warning)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Reporter: checks.AnnotationCheckName,
						Summary:  "invalid annotation value",
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Message: "`severity` annotation value `bar` must match `^critical$`.",
							},
						},
					},
				}
			},
		},
		{
			description: "valid annotation / required",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    severity: info\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical|info|debug"), nil, true, "", checks.Warning)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "valid annotation / not required",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    severity: info\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical|info|debug"), nil, false, "", checks.Warning)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "templated annotation value / passing",
			content:     "- alert: foo\n  expr: sum(foo)\n  for: 5m\n  annotations:\n    for: 5m\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("for"), nil, checks.MustTemplatedRegexp("{{ $for }}"), nil, true, "", checks.Bug)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "templated annotation value / passing",
			content:     "- alert: foo\n  expr: sum(foo)\n  for: 5m\n  annotations:\n    for: 4m\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("for"), nil, checks.MustTemplatedRegexp("{{ $for }}"), nil, true, "", checks.Bug)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Reporter: checks.AnnotationCheckName,
						Summary:  "invalid annotation value",
						Severity: checks.Bug,
						Diagnostics: []output.Diagnostic{
							{
								Message: "`for` annotation value `4m` must match `^{{ $for }}$`.",
							},
						},
					},
				}
			},
		},
		{
			description: "valid annotation key regex / required",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    annotation_1: info\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("annotation_.*"), nil, checks.MustTemplatedRegexp("critical|info|debug"), nil, true, "", checks.Warning)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "valid annotation key regex / not required",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    annotation_1: info\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("annotation_.*"), nil, checks.MustTemplatedRegexp("critical|info|debug"), nil, false, "", checks.Warning)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "wrong annotation key regex value / required",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    annotation_1: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("annotation_.*"), nil, checks.MustTemplatedRegexp("critical"), nil, true, "", checks.Warning)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Reporter: checks.AnnotationCheckName,
						Summary:  "invalid annotation value",
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Message: "`annotation_.*` annotation value `bar` must match `^critical$`.",
							},
						},
					},
				}
			},
		},
		{
			description: "wrong annotation key regex value / not required",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    annotation_1: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("annotation_.*"), nil, checks.MustTemplatedRegexp("critical"), nil, false, "", checks.Warning)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Reporter: checks.AnnotationCheckName,
						Summary:  "invalid annotation value",
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Message: "`annotation_.*` annotation value `bar` must match `^critical$`.",
							},
						},
					},
				}
			},
		},
		{
			description: "invalid value / token / valueRe",
			content:     "- alert: foo\n  expr: rate(foo[1m])\n  annotations:\n    components: api db\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAnnotationCheck(checks.MustTemplatedRegexp("components"), checks.MustRawTemplatedRegexp("\\w+"), checks.MustTemplatedRegexp("api|memcached"), nil, false, "rule comment", checks.Bug)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Reporter: checks.AnnotationCheckName,
						Summary:  "invalid annotation value",
						Details:  "Rule comment: rule comment",
						Severity: checks.Bug,
						Diagnostics: []output.Diagnostic{
							{
								Message: "`components` annotation value `db` must match `^api|memcached$`.",
							},
						},
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
					"rule comment",
					checks.Bug,
				)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Reporter: checks.AnnotationCheckName,
						Summary:  "invalid annotation value",
						Details:  "List of allowed values:\n\n- `api`\n- `memcached`\n- `storage`\n- `prometheus`\n- `kvm`\n- `mysql`\n\nAnd 3 other value(s).\nRule comment: rule comment",
						Severity: checks.Bug,
						Diagnostics: []output.Diagnostic{
							{
								Message: "`components` annotation value `db` is not one of valid values.",
							},
						},
					},
				}
			},
		},
	}
	runTests(t, testCases)
}
