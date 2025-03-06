package checks_test

import (
	"testing"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/promapi"
)

func TestRuleName(t *testing.T) {
	testCases := []checkTest{
		{
			description: "recording rule name doesn't match",
			content:     "- record: foo\n  expr: sum(foo) without(\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleNameCheck(checks.MustTemplatedRegexp("total:.+"), "some text", checks.Warning)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Reporter: checks.RuleNameCheckName,
						Summary:  "name not allowed",
						Details:  "Rule comment: some text",
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Message: "recording rule name must match `^total:.+$`.",
							},
						},
					},
				}
			},
		},
		{
			description: "recording rule name match",
			content:     "- record: total:foo\n  expr: sum(foo) without(\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleNameCheck(checks.MustTemplatedRegexp("total:.+"), "some text", checks.Warning)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "alerting rule name doesn't match",
			content:     "- alert: foo\n  expr: sum(foo) without(\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleNameCheck(checks.MustTemplatedRegexp("total:.+"), "some text", checks.Warning)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Reporter: checks.RuleNameCheckName,
						Summary:  "name not allowed",
						Details:  "Rule comment: some text",
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Message: "alerting rule name must match `^total:.+$`.",
							},
						},
					},
				}
			},
		},
		{
			description: "alerting rule name match",
			content:     "- alert: total:foo\n  expr: sum(foo) without(\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleNameCheck(checks.MustTemplatedRegexp("total:.+"), "some text", checks.Warning)
			},
			prometheus: noProm,
			problems:   noProblems,
		},
	}
	runTests(t, testCases)
}
