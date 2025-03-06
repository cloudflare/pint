package checks_test

import (
	"fmt"
	"testing"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/promapi"
)

func newFragileCheck(_ *promapi.FailoverGroup) checks.RuleChecker {
	return checks.NewFragileCheck()
}

func fragileSampleFunc(s string) string {
	return fmt.Sprintf("Using `%s` to select time series might return different set of time series on every query, which would cause flapping alerts.", s)
}

func TestFragileCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "ignores syntax errors",
			content:     "- record: foo\n  expr: up ==\n",
			checker:     newFragileCheck,
			prometheus:  noProm,
			problems:    noProblems,
		},
		{
			description: "ignores recording rules",
			content:     "- record: foo\n  expr: up == 0\n",
			checker:     newFragileCheck,
			prometheus:  noProm,
			problems:    noProblems,
		},
		{
			description: "warns about topk() as source of series",
			content:     "- alert: foo\n  expr: topk(10, foo)\n",
			checker:     newFragileCheck,
			prometheus:  noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Reporter: checks.FragileCheckName,
						Summary:  "fragile query",
						Details:  checks.FragileCheckSamplingDetails,
						Severity: checks.Warning,
						Diagnostics: []diags.Diagnostic{
							{
								Message: fragileSampleFunc("topk"),
							},
						},
					},
				}
			},
		},
		{
			description: "warns about topk() as source of series (or)",
			content:     "- alert: foo\n  expr: bar or topk(10, foo)\n",
			checker:     newFragileCheck,
			prometheus:  noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Reporter: checks.FragileCheckName,
						Summary:  "fragile query",
						Details:  checks.FragileCheckSamplingDetails,
						Severity: checks.Warning,
						Diagnostics: []diags.Diagnostic{
							{
								Message: fragileSampleFunc("topk"),
							},
						},
					},
				}
			},
		},
		{
			description: "warns about topk() as source of series (multiple)",
			content:     "- alert: foo\n  expr: bar or topk(10, foo) or bottomk(10, foo)\n",
			checker:     newFragileCheck,
			prometheus:  noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Reporter: checks.FragileCheckName,
						Summary:  "fragile query",
						Details:  checks.FragileCheckSamplingDetails,
						Severity: checks.Warning,
						Diagnostics: []diags.Diagnostic{
							{
								Message: fragileSampleFunc("topk"),
							},
						},
					},
					{
						Reporter: checks.FragileCheckName,
						Summary:  "fragile query",
						Details:  checks.FragileCheckSamplingDetails,
						Severity: checks.Warning,
						Diagnostics: []diags.Diagnostic{
							{
								Message: fragileSampleFunc("bottomk"),
							},
						},
					},
				}
			},
		},
		{
			description: "ignores aggregated topk()",
			content:     "- alert: foo\n  expr: min(topk(10, foo)) > 5000\n",
			checker:     newFragileCheck,
			prometheus:  noProm,
			problems:    noProblems,
		},
	}

	runTests(t, testCases)
}
