package checks_test

import (
	"testing"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/promapi"
)

func newComparisonCheck(_ *promapi.FailoverGroup) checks.RuleChecker {
	return checks.NewComparisonCheck()
}

func TestComparisonCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "ignores recording rules",
			content:     "- record: foo\n  expr: up == 0\n",
			checker:     newComparisonCheck,
			prometheus:  noProm,
		},
		{
			description: "ignores rules with syntax errors",
			content:     "- alert: Foo Is Down\n  expr: sum(\n",
			checker:     newComparisonCheck,
			prometheus:  noProm,
		},
		{
			description: "alert expr with > condition",
			content:     "- alert: Foo Is Down\n  for: 10m\n  expr: up{job=\"foo\"} > 0\n",
			checker:     newComparisonCheck,
			prometheus:  noProm,
		},
		{
			description: "alert expr with >= condition",
			content:     "- alert: Foo Is Down\n  for: 10m\n  expr: up{job=\"foo\"} >= 1\n",
			checker:     newComparisonCheck,
			prometheus:  noProm,
		},
		{
			description: "alert expr with == condition",
			content:     "- alert: Foo Is Down\n  for: 10m\n  expr: up{job=\"foo\"} == 1\n",
			checker:     newComparisonCheck,
			prometheus:  noProm,
		},
		{
			description: "alert expr without any condition",
			content:     "- alert: Foo Is Down\n  expr: up{job=\"foo\"}\n",
			checker:     newComparisonCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "deep level comparison",
			content: `
- alert: High_UDP_Receive_Errors
  expr: quantile_over_time(0.7,(irate(udp_packets_drops[2m]))[10m:2m]) > 200
        AND ON (instance)
        (rate(node_netstat_Udp_RcvbufErrors[5m])+rate(node_netstat_Udp6_RcvbufErrors[5m])) > 200
`,
			checker:    newComparisonCheck,
			prometheus: noProm,
		},
		{
			description: "deep level without comparison",
			content: `
- alert: High_UDP_Receive_Errors
  expr: quantile_over_time(0.7,(irate(udp_packets_drops[2m]))[10m:2m])
        AND ON (instance)
        rate(node_netstat_Udp_RcvbufErrors[5m])+rate(node_netstat_Udp6_RcvbufErrors[5m])
`,
			checker:    newComparisonCheck,
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "alert unless condition",
			content:     "- alert: Foo Is Down\n  for: 10m\n  expr: foo unless bar\n",
			checker:     newComparisonCheck,
			prometheus:  noProm,
		},
		{
			description: "alert expr with bool",
			content:     "- alert: Error rate is high\n  expr: rate(error_count[5m]) > bool 5\n",
			checker:     newComparisonCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "alert expr with bool and condition",
			content:     "- alert: Error rate is high\n  expr: rate(error_count[5m]) > bool 5 == 1\n",
			checker:     newComparisonCheck,
			prometheus:  noProm,
		},
		{
			description: "alert on absent",
			content:     "- alert: Foo Is Missing\n  expr: absent(foo)\n",
			checker:     newComparisonCheck,
			prometheus:  noProm,
		},
		{
			description: "absent or absent",
			content:     "- alert: Foo Is Missing\n  expr: absent(foo) or absent(bar)\n",
			checker:     newComparisonCheck,
			prometheus:  noProm,
		},
		{
			description: "absent or absent or absent",
			content:     "- alert: Foo Is Missing\n  expr: absent(foo) or absent(bar) or absent(bob{job=\"xx\"})\n",
			checker:     newComparisonCheck,
			prometheus:  noProm,
		},
		{
			description: "alert on absent_over_time",
			content:     "- alert: Foo Is Missing\n  expr: absent_over_time(foo[5m])\n",
			checker:     newComparisonCheck,
			prometheus:  noProm,
		},
		{
			description: "(foo > 1) > bool 1",
			content:     "- alert: Foo Is Missing\n  expr: (foo > 1) > bool 1\n",
			checker:     newComparisonCheck,
			prometheus:  noProm,
		},
		{
			description: "vector(0) or (foo > 0)",
			content:     "- alert: Foo Is Down\n  expr: (foo > 0) or vector(0)\n",
			checker:     newComparisonCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "(foo > 0) or vector(0)",
			content:     "- alert: Foo Is Down\n  expr: (foo > 0) or vector(0)\n",
			checker:     newComparisonCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "absent(foo) or vector(0)",
			content:     "- alert: Foo Is Down\n  expr: (foo > 0) or vector(0)\n",
			checker:     newComparisonCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "(foo or vector(0)) / bar > 0",
			content:     "- alert: Foo Is Missing\n  expr: (foo or vector(0)) / bar > 0\n",
			checker:     newComparisonCheck,
			prometheus:  noProm,
		},
		{
			// FIXME this should warn because missing foo makes this `0 / <something> <= 0`, so <something> > 0 makes it `0 <= 0`.
			description: "(foo or vector(0)) / bar <= 0",
			content:     "- alert: Foo Is Missing\n  expr: (foo or vector(0)) / bar <= 0\n",
			checker:     newComparisonCheck,
			prometheus:  noProm,
		},
		{
			description: "max() * group_right label_replace(...)",
			content: `
- alert: Foo Is Missing
  expr: |
    max(kernel_xfs_corruption_errors > 0) by (instance)
    * on (instance) group_right
    label_replace(salt_highstate_runner_configured_minions, "instance", "$1", "minion", "(.+)")
`,
			checker:    newComparisonCheck,
			prometheus: noProm,
		},
		{
			description: "vector(0)",
			content:     "- alert: Foo Is Down\n  expr: vector(0)\n",
			checker:     newComparisonCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "alert on absent(vector(1))",
			content:     "- alert: Foo Is Missing\n  expr: absent(vector(1))\n",
			checker:     newComparisonCheck,
			prometheus:  noProm,
		},
	}

	runTests(t, testCases)
}
