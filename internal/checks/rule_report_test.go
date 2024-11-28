package checks_test

import (
	"testing"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
)

func TestReportCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "report passed problem / warning",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    alert: foo\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewReportCheck("problem reported", checks.Warning)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 1,
							Last:  4,
						},
						Reporter: "rule/report",
						Text:     "problem reported",
						Severity: checks.Warning,
					},
				}
			},
		},
		{
			description: "report passed problem / info",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewReportCheck("problem reported", checks.Information)
			},
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 1,
							Last:  2,
						},
						Reporter: "rule/report",
						Text:     "problem reported",
						Severity: checks.Information,
					},
				}
			},
		},
	}
	runTests(t, testCases)
}
