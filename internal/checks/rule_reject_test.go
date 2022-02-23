package checks_test

import (
	"testing"

	"github.com/cloudflare/pint/internal/checks"
)

func TestRejectCheck(t *testing.T) {
	badRe := checks.MustTemplatedRegexp("bad")
	testCases := []checkTest{
		{
			description: "no rules / alerting",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker:     checks.NewRejectCheck(true, true, nil, nil, checks.Bug),
		},
		{
			description: "no rules / recording",
			content:     "- alert: foo\n  expr: sum(foo)\n",
			checker:     checks.NewRejectCheck(true, true, nil, nil, checks.Bug),
		},
		{
			description: "allowed label / alerting",
			content:     "- alert: foo\n  expr: sum(foo)\n  labels:\n    foo: bar\n",
			checker:     checks.NewRejectCheck(true, true, nil, nil, checks.Bug),
		},
		{
			description: "allowed label / recording",
			content:     "- record: foo\n  expr: sum(foo)\n  labels:\n    foo: bar\n",
			checker:     checks.NewRejectCheck(true, true, nil, nil, checks.Bug),
		},
		{
			description: "allowed label / alerting",
			content:     "- alert: foo\n  expr: sum(foo)\n  labels:\n    foo: bar\n",
			checker:     checks.NewRejectCheck(true, true, badRe, badRe, checks.Bug),
		},
		{
			description: "allowed label / alerting",
			content:     "- record: foo\n  expr: sum(foo)\n  labels:\n    foo: bar\n",
			checker:     checks.NewRejectCheck(true, true, badRe, badRe, checks.Bug),
		},
		{
			description: "rejected key / don't check labels",
			content:     "- alert: foo\n  expr: sum(foo)\n  labels:\n    bad: bar\n",
			checker:     checks.NewRejectCheck(false, true, badRe, badRe, checks.Bug),
		},
		{
			description: "rejected key / alerting",
			content:     "- alert: foo\n  expr: sum(foo)\n  labels:\n    bad: bar\n",
			checker:     checks.NewRejectCheck(true, true, badRe, badRe, checks.Bug),
			problems: []checks.Problem{
				{
					Fragment: `bad`,
					Lines:    []int{4},
					Reporter: "rule/reject",
					Text:     `label key bad is not allowed to match "^bad$"`,
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "rejected value / alerting",
			content:     "- alert: foo\n  expr: sum(foo)\n  labels:\n    foo: bad\n",
			checker:     checks.NewRejectCheck(true, true, badRe, badRe, checks.Warning),
			problems: []checks.Problem{
				{
					Fragment: `bad`,
					Lines:    []int{4},
					Reporter: "rule/reject",
					Text:     `label value bad is not allowed to match "^bad$"`,
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "rejected key / recording",
			content:     "- record: foo\n  expr: sum(foo)\n  labels:\n    bad: bar\n",
			checker:     checks.NewRejectCheck(true, true, badRe, badRe, checks.Bug),
			problems: []checks.Problem{
				{
					Fragment: `bad`,
					Lines:    []int{4},
					Reporter: "rule/reject",
					Text:     `label key bad is not allowed to match "^bad$"`,
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "rejected value / recording",
			content:     "- record: foo\n  expr: sum(foo)\n  labels:\n    foo: bad\n",
			checker:     checks.NewRejectCheck(true, true, badRe, badRe, checks.Bug),
			problems: []checks.Problem{
				{
					Fragment: `bad`,
					Lines:    []int{4},
					Reporter: "rule/reject",
					Text:     `label value bad is not allowed to match "^bad$"`,
					Severity: checks.Bug,
				},
			},
		},

		{
			description: "allowed annotation",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    foo: bar\n",
			checker:     checks.NewRejectCheck(true, true, badRe, badRe, checks.Bug),
		},
		{
			description: "rejected key / don't check annotations",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    bad: bar\n",
			checker:     checks.NewRejectCheck(false, false, badRe, badRe, checks.Bug),
		},
		{
			description: "rejected annotation key",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    bad: bar\n",
			checker:     checks.NewRejectCheck(true, true, badRe, badRe, checks.Information),
			problems: []checks.Problem{
				{
					Fragment: `bad`,
					Lines:    []int{4},
					Reporter: "rule/reject",
					Text:     `annotation key bad is not allowed to match "^bad$"`,
					Severity: checks.Information,
				},
			},
		},
		{
			description: "rejected annotation value",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    foo: bad\n",
			checker:     checks.NewRejectCheck(true, true, badRe, badRe, checks.Bug),
			problems: []checks.Problem{
				{
					Fragment: `bad`,
					Lines:    []int{4},
					Reporter: "rule/reject",
					Text:     `annotation value bad is not allowed to match "^bad$"`,
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "reject templated regexp / passing",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    foo: alert\n",
			checker:     checks.NewRejectCheck(true, true, nil, checks.MustTemplatedRegexp("{{ $alert }}"), checks.Bug),
		},
		{
			description: "reject templated regexp / not passing",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    alert: foo\n",
			checker:     checks.NewRejectCheck(true, true, nil, checks.MustTemplatedRegexp("{{ $alert }}"), checks.Bug),
			problems: []checks.Problem{
				{
					Fragment: `foo`,
					Lines:    []int{4},
					Reporter: "rule/reject",
					Text:     `annotation value foo is not allowed to match "^{{ $alert }}$"`,
					Severity: checks.Bug,
				},
			},
		},
	}
	runTests(t, testCases)
}
