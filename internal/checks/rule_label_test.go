package checks_test

import (
	"testing"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
)

func TestLabelCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "doesn't ignore rules with syntax errors",
			content:     "- record: foo\n  expr: sum(foo) without(\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical"), nil, true, "some text", checks.Warning)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 1,
							Last:  2,
						},
						Reporter: checks.LabelCheckName,
						Summary:  "required label not set",
						Details:  "Rule comment: some text",
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Message: "`severity` label is required.",
							},
						},
					},
				}
			},
		},
		{
			description: "no labels in recording rule / required",
			content:     "- record: foo\n  expr: rate(foo[1m])\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical"), nil, true, "", checks.Warning)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 1,
							Last:  2,
						},
						Reporter: checks.LabelCheckName,
						Summary:  "required label not set",
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Message: "`severity` label is required.",
							},
						},
					},
				}
			},
		},
		{
			description: "empty label in recording rule / required",
			content: `
- record: foo
  expr: rate(foo[1m])
  labels:
    foo: bar
    severity:
    level: warning
`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("severity"), nil, nil, nil, true, "", checks.Warning)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 4,
							Last:  7,
						},
						Reporter: checks.LabelCheckName,
						Summary:  "required label not set",
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Message: "`severity` label is required.",
							},
						},
					},
				}
			},
		},
		{
			description: "no labels in recording rule / not required",
			content:     "- record: foo\n  expr: rate(foo[1m])\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical"), nil, false, "", checks.Warning)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "missing label in recording rule / required",
			content:     "- record: foo\n  expr: rate(foo[1m])\n  labels:\n    foo: bar\n    bob: alice\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(
					checks.MustTemplatedRegexp("sev.+"),
					checks.MustRawTemplatedRegexp("\\w+"),
					checks.MustTemplatedRegexp("critical"),
					nil,
					true,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 3,
							Last:  5,
						},
						Reporter: checks.LabelCheckName,
						Summary:  "required label not set",
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Message: "`sev.+` label is required.",
							},
						},
					},
				}
			},
		},
		{
			description: "missing label in recording rule / not required",
			content:     "- record: foo\n  expr: rate(foo[1m])\n  labels:\n    foo: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical"), nil, false, "", checks.Warning)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "invalid value in recording rule / required",
			content:     "- record: foo\n  expr: rate(foo[1m])\n  labels:\n    severity: warning\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical"), nil, true, "", checks.Warning)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 4,
							Last:  4,
						},
						Reporter: checks.LabelCheckName,
						Summary:  "invalid label value",
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Message: "`severity` label value `warning` must match `^critical$`.",
							},
						},
					},
				}
			},
		},
		{
			description: "invalid value in recording rule / not required",
			content:     "- record: foo\n  expr: rate(foo[1m])\n  labels:\n    severity: warning\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical"), nil, false, "", checks.Warning)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 4,
							Last:  4,
						},
						Reporter: checks.LabelCheckName,
						Summary:  "invalid label value",
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Message: "`severity` label value `warning` must match `^critical$`.",
							},
						},
					},
				}
			},
		},
		{
			description: "typo in recording rule / required",
			content:     "- record: foo\n  expr: rate(foo[1m])\n  labels:\n    priority: 2a\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(
					checks.MustTemplatedRegexp("priority"),
					nil,
					checks.MustTemplatedRegexp("(1|2|3)"),
					nil,
					true,
					"some text",
					checks.Warning,
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
						Reporter: checks.LabelCheckName,
						Summary:  "invalid label value",
						Details:  "Rule comment: some text",
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Message: "`priority` label value `2a` must match `^(1|2|3)$`.",
							},
						},
					},
				}
			},
		},
		{
			description: "typo in recording rule / not required",
			content:     "- record: foo\n  expr: rate(foo[1m])\n  labels:\n    priority: 2a\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(
					checks.MustTemplatedRegexp("priority"),
					nil,
					checks.MustTemplatedRegexp("(1|2|3)"),
					nil,
					false,
					"",
					checks.Warning,
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
						Reporter: checks.LabelCheckName,
						Summary:  "invalid label value",
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Message: "`priority` label value `2a` must match `^(1|2|3)$`.",
							},
						},
					},
				}
			},
		},
		{
			description: "no labels in alerting rule / required",
			content:     "- alert: foo\n  expr: rate(foo[1m])\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical"), nil, true, "", checks.Warning)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 1,
							Last:  2,
						},
						Reporter: checks.LabelCheckName,
						Summary:  "required label not set",
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Message: "`severity` label is required.",
							},
						},
					},
				}
			},
		},
		{
			description: "empty label in alerting rule / required",
			content: `
- alert: foo
  expr: rate(foo[1m])
  labels:
    foo: bar
    severity:
    level: warning
`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("severity"), nil, nil, nil, true, "", checks.Warning)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 4,
							Last:  7,
						},
						Reporter: checks.LabelCheckName,
						Summary:  "required label not set",
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Message: "`severity` label is required.",
							},
						},
					},
				}
			},
		},
		{
			description: "no labels in alerting rule / not required",
			content:     "- alert: foo\n  expr: rate(foo[1m])\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical"), nil, false, "", checks.Warning)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "missing label in alerting rule / required",
			content:     "- alert: foo\n  expr: rate(foo[1m])\n  labels:\n    foo: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical"), nil, true, "", checks.Warning)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 3,
							Last:  4,
						},
						Reporter: checks.LabelCheckName,
						Summary:  "required label not set",
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Message: "`severity` label is required.",
							},
						},
					},
				}
			},
		},
		{
			description: "missing label in alerting rule / not required",
			content:     "- alert: foo\n  expr: rate(foo[1m])\n  labels:\n    foo: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical"), nil, false, "", checks.Warning)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "invalid value in alerting rule / required",
			content:     "- alert: foo\n  expr: rate(foo[1m])\n  labels:\n    severity: warning\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical|info"), nil, true, "", checks.Warning)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 4,
							Last:  4,
						},
						Reporter: checks.LabelCheckName,
						Summary:  "invalid label value",
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Message: "`severity` label value `warning` must match `^critical|info$`.",
							},
						},
					},
				}
			},
		},
		{
			description: "invalid value in alerting rule / not required",
			content:     "- alert: foo\n  expr: rate(foo[1m])\n  labels:\n    severity: warning\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical|info"), nil, false, "", checks.Warning)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 4,
							Last:  4,
						},
						Reporter: checks.LabelCheckName,
						Summary:  "invalid label value",
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Message: "`severity` label value `warning` must match `^critical|info$`.",
							},
						},
					},
				}
			},
		},
		{
			description: "valid recording rule / required",
			content:     "- record: foo\n  expr: rate(foo[1m])\n  labels:\n    severity: critical\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical|info"), nil, true, "", checks.Warning)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "valid recording rule / not required",
			content:     "- record: foo\n  expr: rate(foo[1m])\n  labels:\n    severity: critical\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical|info"), nil, false, "", checks.Warning)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "valid alerting rule / required",
			content:     "- alert: foo\n  expr: rate(foo[1m])\n  labels:\n    severity: info\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical|info"), nil, true, "", checks.Warning)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "valid alerting rule / not required",
			content:     "- alert: foo\n  expr: rate(foo[1m])\n  labels:\n    severity: info\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical|info"), nil, false, "", checks.Warning)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "templated label value / passing",
			content:     "- alert: foo\n  expr: sum(foo)\n  for: 5m\n  labels:\n    for: 'must wait 5m to fire'\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("for"), nil, checks.MustTemplatedRegexp("must wait {{$for}} to fire"), nil, true, "", checks.Warning)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "templated label value / not passing",
			content:     "- alert: foo\n  expr: sum(foo)\n  for: 4m\n  labels:\n    for: 'must wait 5m to fire'\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("for"), nil, checks.MustTemplatedRegexp("must wait {{$for}} to fire"), nil, true, "", checks.Warning)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 5,
							Last:  5,
						},
						Reporter: checks.LabelCheckName,
						Summary:  "invalid label value",
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Message: "`for` label value `must wait 5m to fire` must match `^must wait {{$for}} to fire$`.",
							},
						},
					},
				}
			},
		},
		{
			description: "invalid value in alerting rule / token / valueRe",
			content:     "- alert: foo\n  expr: rate(foo[1m])\n  labels:\n    components: api db\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("components"), checks.MustRawTemplatedRegexp("\\w+"), checks.MustTemplatedRegexp("api|memcached"), nil, false, "", checks.Bug)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 4,
							Last:  4,
						},
						Reporter: checks.LabelCheckName,
						Summary:  "invalid label value",
						Severity: checks.Bug,
						Diagnostics: []output.Diagnostic{
							{
								Message: "`components` label value `db` must match `^api|memcached$`.",
							},
						},
					},
				}
			},
		},
		{
			description: "invalid value in alerting rule / token / values",
			content:     "- alert: foo\n  expr: rate(foo[1m])\n  labels:\n    components: api db\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(
					checks.MustTemplatedRegexp("components"),
					checks.MustRawTemplatedRegexp("\\w+"),
					nil,
					[]string{"api", "memcached", "storage", "prometheus", "kvm", "mysql", "memsql", "haproxy", "postgresql"},
					false,
					"",
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
						Reporter: checks.LabelCheckName,
						Summary:  "invalid label value",
						Details:  "List of allowed values:\n\n- `api`\n- `memcached`\n- `storage`\n- `prometheus`\n- `kvm`\n- `mysql`\n\nAnd 3 other value(s).",
						Severity: checks.Bug,
						Diagnostics: []output.Diagnostic{
							{
								Message: "`components` label value `db` is not one of valid values.",
							},
						},
					},
				}
			},
		},
		{
			description: "invalid value in recording rule / token / valueRe",
			content:     "- record: foo\n  expr: rate(foo[1m])\n  labels:\n    components: api db\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("components"), checks.MustRawTemplatedRegexp("\\w+"), checks.MustTemplatedRegexp("api|memcached"), nil, false, "", checks.Bug)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 4,
							Last:  4,
						},
						Reporter: checks.LabelCheckName,
						Summary:  "invalid label value",
						Severity: checks.Bug,
						Diagnostics: []output.Diagnostic{
							{
								Message: "`components` label value `db` must match `^api|memcached$`.",
							},
						},
					},
				}
			},
		},
		{
			description: "invalid value in recording rule / token / values",
			content:     "- record: foo\n  expr: rate(foo[1m])\n  labels:\n    components: api db\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(
					checks.MustTemplatedRegexp("components"),
					checks.MustRawTemplatedRegexp("\\w+"),
					nil,
					[]string{"api", "memcached", "storage", "prometheus", "kvm", "mysql", "memsql", "haproxy"},
					false,
					"some text",
					checks.Warning,
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
						Reporter: checks.LabelCheckName,
						Summary:  "invalid label value",
						Details:  "List of allowed values:\n\n- `api`\n- `memcached`\n- `storage`\n- `prometheus`\n- `kvm`\n- `mysql`\n- `memsql`\n- `haproxy`\n\nRule comment: some text",
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Message: "`components` label value `db` is not one of valid values.",
							},
						},
					},
				}
			},
		},
	}
	runTests(t, testCases)
}
