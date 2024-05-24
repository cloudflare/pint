package checks_test

import (
	"testing"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
)

func topkProblem() string {
	return "usage of topk or bottomk in recording rules is discouraged and creates churn"
}

func TestRuleTopkCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "recording rule without topk or bottomk",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleTopkCheck()
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "recording rule with topk",
			content:     "- record: foo\n  expr: topk(5, up)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleTopkCheck()
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "rule/topk",
						Text:     topkProblem(),
						Severity: checks.Warning,
						Details:  checks.TopkCheckRuleDetails,
					},
				}
			},
		},
		{
			description: "recording rule with bottomk",
			content:     "- record: foo\n  expr: bottomk(5, up)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleTopkCheck()
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "rule/topk",
						Text:     topkProblem(),
						Severity: checks.Warning,
						Details:  checks.TopkCheckRuleDetails,
					},
				}
			},
		},
		{
			description: "recording rule with nested topk",
			content:     "- record: foo\n  expr: sum(topk(5, up))\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleTopkCheck()
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "rule/topk",
						Text:     topkProblem(),
						Severity: checks.Warning,
						Details:  checks.TopkCheckRuleDetails,
					},
				}
			},
		},
		{
			description: "alerting rule that shouldn't trigger",
			content:     "- alert: foo\n  expr: foo\n  for: abc\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleTopkCheck()
			},
			prometheus: noProm,
			problems:   noProblems,
		},
		{
			description: "syntax error that shouldn't trigger",
			content:     "- record: foo\n  expr: rate(topk(5, up)[5m])\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleTopkCheck()
			},
			prometheus: noProm,
			problems:   noProblems,
		},
	}

	runTests(t, testCases)
}
