package checks_test

import (
	"testing"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/promapi"
)

func newFragileCheck(_ *promapi.FailoverGroup) checks.RuleChecker {
	return checks.NewFragileCheck()
}

func TestFragileCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "ignores syntax errors",
			content:     "- record: foo\n  expr: up ==\n",
			checker:     newFragileCheck,
			prometheus:  noProm,
		},
		{
			description: "ignores recording rules",
			content:     "- record: foo\n  expr: up == 0\n",
			checker:     newFragileCheck,
			prometheus:  noProm,
		},
		{
			description: "warns about topk() as source of series",
			content:     "- alert: foo\n  expr: topk(10, foo)\n",
			checker:     newFragileCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "warns about topk() as source of series (or)",
			content:     "- alert: foo\n  expr: bar or topk(10, foo)\n",
			checker:     newFragileCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "warns about topk() as source of series (multiple)",
			content:     "- alert: foo\n  expr: bar or topk(10, foo) or bottomk(10, foo)\n",
			checker:     newFragileCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "ignores aggregated topk()",
			content:     "- alert: foo\n  expr: min(topk(10, foo)) > 5000\n",
			checker:     newFragileCheck,
			prometheus:  noProm,
		},
	}

	runTests(t, testCases)
}
