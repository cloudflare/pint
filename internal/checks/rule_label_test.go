package checks_test

import (
	"testing"

	"github.com/cloudflare/pint/internal/checks"
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
			problems:   true,
		},
		{
			description: "no labels in recording rule / required",
			content:     "- record: foo\n  expr: rate(foo[1m])\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical"), nil, true, "", checks.Warning)
			},
			prometheus: noProm,
			problems:   true,
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
			problems:   true,
		},
		{
			description: "no labels in recording rule / not required",
			content:     "- record: foo\n  expr: rate(foo[1m])\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical"), nil, false, "", checks.Warning)
			},
			prometheus: noProm,
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
			problems:   true,
		},
		{
			description: "missing label in recording rule / not required",
			content:     "- record: foo\n  expr: rate(foo[1m])\n  labels:\n    foo: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical"), nil, false, "", checks.Warning)
			},
			prometheus: noProm,
		},
		{
			description: "invalid value in recording rule / required",
			content:     "- record: foo\n  expr: rate(foo[1m])\n  labels:\n    severity: warning\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical"), nil, true, "", checks.Warning)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "invalid value in recording rule / not required",
			content:     "- record: foo\n  expr: rate(foo[1m])\n  labels:\n    severity: warning\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical"), nil, false, "", checks.Warning)
			},
			prometheus: noProm,
			problems:   true,
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
			problems:   true,
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
			problems:   true,
		},
		{
			description: "no labels in alerting rule / required",
			content:     "- alert: foo\n  expr: rate(foo[1m])\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical"), nil, true, "", checks.Warning)
			},
			prometheus: noProm,
			problems:   true,
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
			problems:   true,
		},
		{
			description: "no labels in alerting rule / not required",
			content:     "- alert: foo\n  expr: rate(foo[1m])\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical"), nil, false, "", checks.Warning)
			},
			prometheus: noProm,
		},
		{
			description: "missing label in alerting rule / required",
			content:     "- alert: foo\n  expr: rate(foo[1m])\n  labels:\n    foo: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical"), nil, true, "", checks.Warning)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "missing label in alerting rule / not required",
			content:     "- alert: foo\n  expr: rate(foo[1m])\n  labels:\n    foo: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical"), nil, false, "", checks.Warning)
			},
			prometheus: noProm,
		},
		{
			description: "invalid value in alerting rule / required",
			content:     "- alert: foo\n  expr: rate(foo[1m])\n  labels:\n    severity: warning\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical|info"), nil, true, "", checks.Warning)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "invalid value in alerting rule / not required",
			content:     "- alert: foo\n  expr: rate(foo[1m])\n  labels:\n    severity: warning\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical|info"), nil, false, "", checks.Warning)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "valid recording rule / required",
			content:     "- record: foo\n  expr: rate(foo[1m])\n  labels:\n    severity: critical\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical|info"), nil, true, "", checks.Warning)
			},
			prometheus: noProm,
		},
		{
			description: "valid recording rule / not required",
			content:     "- record: foo\n  expr: rate(foo[1m])\n  labels:\n    severity: critical\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical|info"), nil, false, "", checks.Warning)
			},
			prometheus: noProm,
		},
		{
			description: "valid alerting rule / required",
			content:     "- alert: foo\n  expr: rate(foo[1m])\n  labels:\n    severity: info\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical|info"), nil, true, "", checks.Warning)
			},
			prometheus: noProm,
		},
		{
			description: "valid alerting rule / not required",
			content:     "- alert: foo\n  expr: rate(foo[1m])\n  labels:\n    severity: info\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical|info"), nil, false, "", checks.Warning)
			},
			prometheus: noProm,
		},
		{
			description: "templated label value / passing",
			content:     "- alert: foo\n  expr: sum(foo)\n  for: 5m\n  labels:\n    for: 'must wait 5m to fire'\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("for"), nil, checks.MustTemplatedRegexp("must wait {{$for}} to fire"), nil, true, "", checks.Warning)
			},
			prometheus: noProm,
		},
		{
			description: "templated label value / not passing",
			content:     "- alert: foo\n  expr: sum(foo)\n  for: 4m\n  labels:\n    for: 'must wait 5m to fire'\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("for"), nil, checks.MustTemplatedRegexp("must wait {{$for}} to fire"), nil, true, "", checks.Warning)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "invalid value in alerting rule / token / valueRe",
			content:     "- alert: foo\n  expr: rate(foo[1m])\n  labels:\n    components: api db\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("components"), checks.MustRawTemplatedRegexp("\\w+"), checks.MustTemplatedRegexp("api|memcached"), nil, false, "", checks.Bug)
			},
			prometheus: noProm,
			problems:   true,
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
			problems:   true,
		},
		{
			description: "invalid value in recording rule / token / valueRe",
			content:     "- record: foo\n  expr: rate(foo[1m])\n  labels:\n    components: api db\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("components"), checks.MustRawTemplatedRegexp("\\w+"), checks.MustTemplatedRegexp("api|memcached"), nil, false, "", checks.Bug)
			},
			prometheus: noProm,
			problems:   true,
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
			problems:   true,
		},
		{
			description:   "no labels in recording rule / strict / required / group label",
			contentStrict: true,
			content: `
groups:
- name: mygroup
  labels:
    severity: critical
  rules:
  - record: foo
    expr: rate(foo[1m])
`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical"), nil, true, "", checks.Warning)
			},
			prometheus: noProm,
		},
		{
			description: "no labels in recording rule / relaxed / required / group label",
			content: `
groups:
- name: mygroup
  labels:
    severity: critical
  rules:
  - record: foo
    expr: rate(foo[1m])
`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical"), nil, true, "", checks.Warning)
			},
			prometheus: noProm,
		},
		{
			description:   "no labels in alerting rule / strict / required / group label",
			contentStrict: true,
			content: `
groups:
- name: mygroup
  labels:
    severity: critical
  rules:
  - alert: foo
    expr: rate(foo[1m])
`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical"), nil, true, "", checks.Warning)
			},
			prometheus: noProm,
		},
		{
			description: "no labels in alerting rule / relaxed / required / group label",
			content: `
groups:
- name: mygroup
  labels:
    severity: critical
  rules:
  - alert: foo
    expr: rate(foo[1m])
`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical"), nil, true, "", checks.Warning)
			},
			prometheus: noProm,
		},
		{
			description:   "invalid value in alerting rule / strict / required",
			contentStrict: true,
			content: `
groups:
- name: mygroup
  labels:
    severity: bogus
  rules:
  - alert: foo
    expr: rate(foo[1m])
`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical|info"), nil, true, "", checks.Warning)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "invalid value in alerting rule / relaxed / required",
			content: `
groups:
- name: mygroup
  labels:
    severity: bogus
  rules:
  - alert: foo
    expr: rate(foo[1m])
`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical|info"), nil, true, "", checks.Warning)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description:   "invalid value in recording rule / strict / required",
			contentStrict: true,
			content: `
groups:
- name: mygroup
  labels:
    severity: bogus
  rules:
  - record: foo
    expr: rate(foo[1m])
`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical|info"), nil, true, "", checks.Warning)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "invalid value in recording rule / relaxed / required",
			content: `
groups:
- name: mygroup
  labels:
    severity: bogus
  rules:
  - record: foo
    expr: rate(foo[1m])
`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewLabelCheck(checks.MustTemplatedRegexp("severity"), nil, checks.MustTemplatedRegexp("critical|info"), nil, true, "", checks.Warning)
			},
			prometheus: noProm,
			problems:   true,
		},
	}
	runTests(t, testCases)
}
