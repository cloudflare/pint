package checks_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
)

func forMin(key, m string) string {
	return fmt.Sprintf("This alert rule must have a `%s` field with a minimum duration of %s.", key, m)
}

func forMax(key, m string) string {
	return fmt.Sprintf("This alert rule must have a `%s` field with a maximum duration of %s.", key, m)
}

func TestRuleForCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "recording rule",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleForCheck(checks.RuleForFor, 0, 0, "", checks.Bug)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "alerting rule, no for, 0-0",
			content:     "- alert: foo\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleForCheck(checks.RuleForFor, 0, 0, "", checks.Bug)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "alerting rule, for:1m, 0-0",
			content:     "- alert: foo\n  for: 1m\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleForCheck(checks.RuleForFor, 0, 0, "", checks.Bug)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "alerting rule, for:1m, 1s-0",
			content:     "- alert: foo\n  for: 1m\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleForCheck(checks.RuleForFor, time.Second, 0, "", checks.Bug)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "alerting rule, for:1m, 1s-2m",
			content:     "- alert: foo\n  for: 1m\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleForCheck(checks.RuleForFor, time.Second, time.Minute*2, "", checks.Bug)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "alerting rule, for:4m, 5m-10m",
			content:     "- alert: foo\n  for: 4m\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleForCheck(checks.RuleForFor, time.Minute*5, time.Minute*10, "", checks.Warning)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "rule/for",
						Summary:  "duration required",
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Message: forMin("for", "5m"),
							},
						},
					},
				}
			},
		},
		{
			description: "alerting rule, for:5m, 1s-2m",
			content:     "- alert: foo\n  for: 5m\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleForCheck(checks.RuleForFor, time.Second, time.Minute*2, "some text", checks.Warning)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "rule/for",
						Summary:  "duration too long",
						Details:  "Rule comment: some text",
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Message: forMax("for", "2m"),
							},
						},
					},
				}
			},
		},
		{
			description: "alerting rule, for:1d, 5m-0",
			content:     "- alert: foo\n  for: 1d\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleForCheck(checks.RuleForFor, time.Minute*5, 0, "", checks.Warning)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "alerting rule, for:14m, 5m-10m, keep_firing_for enforced",
			content:     "- alert: foo\n  for: 14m\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleForCheck(checks.RuleForKeepFiringFor, 0, time.Minute*10, "", checks.Warning)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "alerting rule, keep_firing_for:4m, 5m-10m",
			content:     "- alert: foo\n  keep_firing_for: 4m\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleForCheck(checks.RuleForKeepFiringFor, time.Minute*5, time.Minute*10, "", checks.Warning)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "rule/for",
						Summary:  "duration required",
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Message: forMin("keep_firing_for", "5m"),
							},
						},
					},
				}
			},
		},
	}
	runTests(t, testCases)
}
