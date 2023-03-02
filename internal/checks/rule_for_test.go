package checks_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/promapi"
)

func forMin(min string) string {
	return fmt.Sprintf("this alert rule must have a 'for' field with a minimum duration of %s", min)
}

func forMax(max string) string {
	return fmt.Sprintf("this alert rule must have a 'for' field with a maximum duration of %s", max)
}

func TestRuleForCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "recording rule",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleForCheck(0, 0, checks.Bug)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "alerting rule, no for, 0-0",
			content:     "- alert: foo\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleForCheck(0, 0, checks.Bug)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "alerting rule, for:1m, 0-0",
			content:     "- alert: foo\n  for: 1m\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleForCheck(0, 0, checks.Bug)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "alerting rule, for:1m, 1s-0",
			content:     "- alert: foo\n  for: 1m\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleForCheck(time.Second, 0, checks.Bug)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "alerting rule, for:1m, 1s-2m",
			content:     "- alert: foo\n  for: 1m\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleForCheck(time.Second, time.Minute*2, checks.Bug)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "alerting rule, for:4m, 5m-10m",
			content:     "- alert: foo\n  for: 4m\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleForCheck(time.Minute*5, time.Minute*10, checks.Warning)
			},
			prometheus: noProm,
			problems: func(s string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "4m",
						Lines:    []int{2},
						Reporter: "rule/for",
						Text:     forMin("5m"),
						Severity: checks.Warning,
					},
				}
			},
		},
		{
			description: "alerting rule, for:5m, 1s-2m",
			content:     "- alert: foo\n  for: 5m\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleForCheck(time.Second, time.Minute*2, checks.Warning)
			},
			prometheus: noProm,
			problems: func(s string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "5m",
						Lines:    []int{2},
						Reporter: "rule/for",
						Text:     forMax("2m"),
						Severity: checks.Warning,
					},
				}
			},
		},
		{
			description: "alerting rule, for:1d, 5m-0",
			content:     "- alert: foo\n  for: 1d\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleForCheck(time.Minute*5, 0, checks.Warning)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
	}
	runTests(t, testCases)
}
