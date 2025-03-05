package checks_test

import (
	"testing"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/parser"
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
			problems:   noProblems,
		},
		{
			description: "no rules / recording",
			content:     "- alert: foo\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRejectCheck(true, true, nil, nil, checks.Bug)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "allowed label / alerting",
			content:     "- alert: foo\n  expr: sum(foo)\n  labels:\n    foo: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRejectCheck(true, true, nil, nil, checks.Bug)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "allowed label / recording",
			content:     "- record: foo\n  expr: sum(foo)\n  labels:\n    foo: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRejectCheck(true, true, nil, nil, checks.Bug)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "allowed label / alerting",
			content:     "- alert: foo\n  expr: sum(foo)\n  labels:\n    foo: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRejectCheck(true, true, badRe, badRe, checks.Bug)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "allowed label / alerting",
			content:     "- record: foo\n  expr: sum(foo)\n  labels:\n    foo: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRejectCheck(true, true, badRe, badRe, checks.Bug)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "rejected key / don't check labels",
			content:     "- alert: foo\n  expr: sum(foo)\n  labels:\n    bad: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRejectCheck(false, true, badRe, badRe, checks.Bug)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "rejected key / alerting",
			content:     "- alert: foo\n  expr: sum(foo)\n  labels:\n    bad: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRejectCheck(true, true, badRe, badRe, checks.Bug)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 4,
							Last:  4,
						},
						Reporter: "rule/reject",
						Summary:  "key not allowed",
						Severity: checks.Bug,
						Diagnostics: []output.Diagnostic{
							{
								Line:        4,
								FirstColumn: 5,
								LastColumn:  7,
								Message:     "key is not allowed to match `^bad$`.",
							},
						},
					},
				}
			},
		},
		{
			description: "rejected value / alerting",
			content:     "- alert: foo\n  expr: sum(foo)\n  labels:\n    foo: bad\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRejectCheck(true, true, badRe, badRe, checks.Warning)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 4,
							Last:  4,
						},
						Reporter: "rule/reject",
						Summary:  "value not allowed",
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Line:        4,
								FirstColumn: 10,
								LastColumn:  12,
								Message:     "value is not allowed to match `^bad$`.",
							},
						},
					},
				}
			},
		},
		{
			description: "rejected key / recording",
			content:     "- record: foo\n  expr: sum(foo)\n  labels:\n    bad: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRejectCheck(true, true, badRe, badRe, checks.Bug)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 4,
							Last:  4,
						},
						Reporter: "rule/reject",
						Summary:  "key not allowed",
						Severity: checks.Bug,
						Diagnostics: []output.Diagnostic{
							{
								Line:        4,
								FirstColumn: 5,
								LastColumn:  7,
								Message:     "key is not allowed to match `^bad$`.",
							},
						},
					},
				}
			},
		},
		{
			description: "rejected value / recording",
			content:     "- record: foo\n  expr: sum(foo)\n  labels:\n    foo: bad\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRejectCheck(true, true, badRe, badRe, checks.Bug)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 4,
							Last:  4,
						},
						Reporter: "rule/reject",
						Summary:  "value not allowed",
						Severity: checks.Bug,
						Diagnostics: []output.Diagnostic{
							{
								Line:        4,
								FirstColumn: 10,
								LastColumn:  12,
								Message:     "value is not allowed to match `^bad$`.",
							},
						},
					},
				}
			},
		},

		{
			description: "allowed annotation",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    foo: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRejectCheck(true, true, badRe, badRe, checks.Bug)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "rejected key / don't check annotations",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    bad: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRejectCheck(false, false, badRe, badRe, checks.Bug)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "rejected annotation key",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    bad: bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRejectCheck(true, true, badRe, badRe, checks.Information)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 4,
							Last:  4,
						},
						Reporter: "rule/reject",
						Summary:  "key not allowed",
						Severity: checks.Information,
						Diagnostics: []output.Diagnostic{
							{
								Line:        4,
								FirstColumn: 5,
								LastColumn:  7,
								Message:     "key is not allowed to match `^bad$`.",
							},
						},
					},
				}
			},
		},
		{
			description: "rejected annotation value",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    foo: bad\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRejectCheck(true, true, badRe, badRe, checks.Bug)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 4,
							Last:  4,
						},
						Reporter: "rule/reject",
						Summary:  "value not allowed",
						Severity: checks.Bug,
						Diagnostics: []output.Diagnostic{
							{
								Line:        4,
								FirstColumn: 10,
								LastColumn:  12,
								Message:     "value is not allowed to match `^bad$`.",
							},
						},
					},
				}
			},
		},
		{
			description: "reject templated regexp / passing",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    foo: alert\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRejectCheck(true, true, nil, checks.MustTemplatedRegexp("{{ $alert }}"), checks.Bug)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "reject templated regexp / not passing",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    alert: foo\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRejectCheck(true, true, nil, checks.MustTemplatedRegexp("{{ $alert }}"), checks.Bug)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 4,
							Last:  4,
						},
						Reporter: "rule/reject",
						Summary:  "value not allowed",
						Severity: checks.Bug,
						Diagnostics: []output.Diagnostic{
							{
								Line:        4,
								FirstColumn: 12,
								LastColumn:  14,
								Message:     "value is not allowed to match `^{{ $alert }}$`.",
							},
						},
					},
				}
			},
		},
	}
	runTests(t, testCases)
}
