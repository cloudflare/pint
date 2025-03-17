package checks_test

import (
	"testing"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/promapi"
)

func TestRejectCheck(t *testing.T) {
	badRe := checks.MustTemplatedRegexp("bad")
	testCases := []checkTest{
		{
			description: "no rules / alerting",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRejectCheck(true, true, nil, nil, checks.Bug)
			},
			prometheus: noProm,
		},
		{
			description: "no rules / recording",
			content:     "- alert: foo\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRejectCheck(true, true, nil, nil, checks.Bug)
			},
			prometheus: noProm,
		},
		{
			description: "allowed label / alerting",
			content:     "- alert: foo\n  expr: sum(foo)\n  labels:\n    foo: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRejectCheck(true, true, nil, nil, checks.Bug)
			},
			prometheus: noProm,
		},
		{
			description: "allowed label / recording",
			content:     "- record: foo\n  expr: sum(foo)\n  labels:\n    foo: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRejectCheck(true, true, nil, nil, checks.Bug)
			},
			prometheus: noProm,
		},
		{
			description: "allowed label / alerting",
			content:     "- alert: foo\n  expr: sum(foo)\n  labels:\n    foo: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRejectCheck(true, true, badRe, badRe, checks.Bug)
			},
			prometheus: noProm,
		},
		{
			description: "allowed label / alerting",
			content:     "- record: foo\n  expr: sum(foo)\n  labels:\n    foo: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRejectCheck(true, true, badRe, badRe, checks.Bug)
			},
			prometheus: noProm,
		},
		{
			description: "rejected key / don't check labels",
			content:     "- alert: foo\n  expr: sum(foo)\n  labels:\n    bad: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRejectCheck(false, true, badRe, badRe, checks.Bug)
			},
			prometheus: noProm,
		},
		{
			description: "rejected key / alerting",
			content:     "- alert: foo\n  expr: sum(foo)\n  labels:\n    bad: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRejectCheck(true, true, badRe, badRe, checks.Bug)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "rejected value / alerting",
			content:     "- alert: foo\n  expr: sum(foo)\n  labels:\n    foo: bad\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRejectCheck(true, true, badRe, badRe, checks.Warning)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "rejected key / recording",
			content:     "- record: foo\n  expr: sum(foo)\n  labels:\n    bad: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRejectCheck(true, true, badRe, badRe, checks.Bug)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "rejected value / recording",
			content:     "- record: foo\n  expr: sum(foo)\n  labels:\n    foo: bad\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRejectCheck(true, true, badRe, badRe, checks.Bug)
			},
			prometheus: noProm,
			problems:   true,
		},

		{
			description: "allowed annotation",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    foo: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRejectCheck(true, true, badRe, badRe, checks.Bug)
			},
			prometheus: noProm,
		},
		{
			description: "rejected key / don't check annotations",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    bad: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRejectCheck(false, false, badRe, badRe, checks.Bug)
			},
			prometheus: noProm,
		},
		{
			description: "rejected annotation key",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    bad: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRejectCheck(true, true, badRe, badRe, checks.Information)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "rejected annotation value",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    foo: bad\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRejectCheck(true, true, badRe, badRe, checks.Bug)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "reject templated regexp / passing",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    foo: alert\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRejectCheck(true, true, nil, checks.MustTemplatedRegexp("{{ $alert }}"), checks.Bug)
			},
			prometheus: noProm,
		},
		{
			description: "reject templated regexp / not passing",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    alert: foo\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRejectCheck(true, true, nil, checks.MustTemplatedRegexp("{{ $alert }}"), checks.Bug)
			},
			prometheus: noProm,
			problems:   true,
		},
	}
	runTests(t, testCases)
}
