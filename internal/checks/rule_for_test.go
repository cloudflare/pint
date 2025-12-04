package checks_test

import (
	"testing"
	"time"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/promapi"
)

func TestRuleForCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "recording rule",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleForCheck(checks.RuleForFor, 0, 0, "", checks.Bug)
			},
			prometheus: noProm,
		},
		{
			description: "alerting rule, no for, 0-0",
			content:     "- alert: foo\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleForCheck(checks.RuleForFor, 0, 0, "", checks.Bug)
			},
			prometheus: noProm,
		},
		{
			description: "alerting rule, for:1m, 0-0",
			content:     "- alert: foo\n  for: 1m\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleForCheck(checks.RuleForFor, 0, 0, "", checks.Bug)
			},
			prometheus: noProm,
		},
		{
			description: "alerting rule, for:1m, 1s-0",
			content:     "- alert: foo\n  for: 1m\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleForCheck(checks.RuleForFor, time.Second, 0, "", checks.Bug)
			},
			prometheus: noProm,
		},
		{
			description: "alerting rule, for:1m, 1s-2m",
			content:     "- alert: foo\n  for: 1m\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleForCheck(checks.RuleForFor, time.Second, time.Minute*2, "", checks.Bug)
			},
			prometheus: noProm,
		},
		{
			description: "alerting rule, for:4m, 5m-10m",
			content:     "- alert: foo\n  for: 4m\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleForCheck(checks.RuleForFor, time.Minute*5, time.Minute*10, "", checks.Warning)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "alerting rule, for:5m, 1s-2m",
			content:     "- alert: foo\n  for: 5m\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleForCheck(checks.RuleForFor, time.Second, time.Minute*2, "some text", checks.Warning)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "alerting rule, for:1d, 5m-0",
			content:     "- alert: foo\n  for: 1d\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleForCheck(checks.RuleForFor, time.Minute*5, 0, "", checks.Warning)
			},
			prometheus: noProm,
		},
		{
			description: "alerting rule, for:14m, 5m-10m, keep_firing_for enforced",
			content:     "- alert: foo\n  for: 14m\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleForCheck(checks.RuleForKeepFiringFor, 0, time.Minute*10, "", checks.Warning)
			},
			prometheus: noProm,
		},
		{
			description: "alerting rule, keep_firing_for:4m, 5m-10m",
			content:     "- alert: foo\n  keep_firing_for: 4m\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleForCheck(checks.RuleForKeepFiringFor, time.Minute*5, time.Minute*10, "", checks.Warning)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "alerting rule, keep_firing_for:15m, 5m-10m",
			content:     "- alert: foo\n  keep_firing_for: 15m\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleForCheck(checks.RuleForKeepFiringFor, time.Minute*5, time.Minute*10, "", checks.Warning)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "alerting rule, no for, 5m-10m",
			content:     "- alert: foo\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleForCheck(checks.RuleForFor, time.Minute*5, time.Minute*10, "", checks.Warning)
			},
			prometheus: noProm,
			problems:   true,
		},
	}
	runTests(t, testCases)
}
