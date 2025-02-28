package checks_test

import (
	"testing"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/parser"
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
			problems:    noProblems,
		},
		{
			description: "ignores rules with syntax errors",
			content:     "- alert: Foo Is Down\n  expr: sum(\n",
			checker:     newComparisonCheck,
			prometheus:  noProm,
			problems:    noProblems,
		},
		{
			description: "alert expr with > condition",
			content:     "- alert: Foo Is Down\n  for: 10m\n  expr: up{job=\"foo\"} > 0\n",
			checker:     newComparisonCheck,
			prometheus:  noProm,
			problems:    noProblems,
		},
		{
			description: "alert expr with >= condition",
			content:     "- alert: Foo Is Down\n  for: 10m\n  expr: up{job=\"foo\"} >= 1\n",
			checker:     newComparisonCheck,
			prometheus:  noProm,
			problems:    noProblems,
		},
		{
			description: "alert expr with == condition",
			content:     "- alert: Foo Is Down\n  for: 10m\n  expr: up{job=\"foo\"} == 1\n",
			checker:     newComparisonCheck,
			prometheus:  noProm,
			problems:    noProblems,
		},
		{
			description: "alert expr without any condition",
			content:     "- alert: Foo Is Down\n  expr: up{job=\"foo\"}\n",
			checker:     newComparisonCheck,
			prometheus:  noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: checks.ComparisonCheckName,
						Summary:  "always firing alert",
						Details:  checks.ComparisonCheckDetails,
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Message: "Alert query doesn't have any condition, it will always fire if the metric exists.",
							},
						},
					},
				}
			},
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
			problems:   noProblems,
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
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 3,
							Last:  5,
						},
						Reporter: checks.ComparisonCheckName,
						Summary:  "always firing alert",
						Details:  checks.ComparisonCheckDetails,
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Message: "Alert query doesn't have any condition, it will always fire if the metric exists.",
							},
						},
					},
				}
			},
		},
		{
			description: "alert unless condition",
			content:     "- alert: Foo Is Down\n  for: 10m\n  expr: foo unless bar\n",
			checker:     newComparisonCheck,
			prometheus:  noProm,
			problems:    noProblems,
		},
		{
			description: "alert expr with bool",
			content:     "- alert: Error rate is high\n  expr: rate(error_count[5m]) > bool 5\n",
			checker:     newComparisonCheck,
			prometheus:  noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: checks.ComparisonCheckName,
						Summary:  "always firing alert",
						Details:  checks.ComparisonCheckDetails,
						Severity: checks.Bug,
						Diagnostics: []output.Diagnostic{
							{
								Message: "Alert query uses `bool` modifier for comparison, this means it will always return a result and the alert will always fire.",
							},
						},
					},
				}
			},
		},
		{
			description: "alert expr with bool and condition",
			content:     "- alert: Error rate is high\n  expr: rate(error_count[5m]) > bool 5 == 1\n",
			checker:     newComparisonCheck,
			prometheus:  noProm,
			problems:    noProblems,
		},
		{
			description: "alert on absent",
			content:     "- alert: Foo Is Missing\n  expr: absent(foo)\n",
			checker:     newComparisonCheck,
			prometheus:  noProm,
			problems:    noProblems,
		},
		{
			description: "absent or absent",
			content:     "- alert: Foo Is Missing\n  expr: absent(foo) or absent(bar)\n",
			checker:     newComparisonCheck,
			prometheus:  noProm,
			problems:    noProblems,
		},
		{
			description: "absent or absent or absent",
			content:     "- alert: Foo Is Missing\n  expr: absent(foo) or absent(bar) or absent(bob{job=\"xx\"})\n",
			checker:     newComparisonCheck,
			prometheus:  noProm,
			problems:    noProblems,
		},
		{
			description: "alert on absent_over_time",
			content:     "- alert: Foo Is Missing\n  expr: absent_over_time(foo[5m])\n",
			checker:     newComparisonCheck,
			prometheus:  noProm,
			problems:    noProblems,
		},
		{
			description: "(foo > 1) > bool 1",
			content:     "- alert: Foo Is Missing\n  expr: (foo > 1) > bool 1\n",
			checker:     newComparisonCheck,
			prometheus:  noProm,
			problems:    noProblems,
		},
		{
			description: "vector(0) or (foo > 0)",
			content:     "- alert: Foo Is Down\n  expr: (foo > 0) or vector(0)\n",
			checker:     newComparisonCheck,
			prometheus:  noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: checks.ComparisonCheckName,
						Summary:  "always firing alert",
						Details:  checks.ComparisonCheckDetails,
						Severity: checks.Bug,
						Diagnostics: []output.Diagnostic{
							{
								Message: "Alert query uses `or` operator with one side of the query that will always return a result, this alert will always fire.",
							},
						},
					},
				}
			},
		},
		{
			description: "(foo > 0) or vector(0)",
			content:     "- alert: Foo Is Down\n  expr: (foo > 0) or vector(0)\n",
			checker:     newComparisonCheck,
			prometheus:  noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: checks.ComparisonCheckName,
						Summary:  "always firing alert",
						Details:  checks.ComparisonCheckDetails,
						Severity: checks.Bug,
						Diagnostics: []output.Diagnostic{
							{
								Message: "Alert query uses `or` operator with one side of the query that will always return a result, this alert will always fire.",
							},
						},
					},
				}
			},
		},
		{
			description: "absent(foo) or vector(0)",
			content:     "- alert: Foo Is Down\n  expr: (foo > 0) or vector(0)\n",
			checker:     newComparisonCheck,
			prometheus:  noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: checks.ComparisonCheckName,
						Summary:  "always firing alert",
						Details:  checks.ComparisonCheckDetails,
						Severity: checks.Bug,
						Diagnostics: []output.Diagnostic{
							{
								Message: "Alert query uses `or` operator with one side of the query that will always return a result, this alert will always fire.",
							},
						},
					},
				}
			},
		},
	}

	runTests(t, testCases)
}
