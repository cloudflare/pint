package checks_test

import (
	"testing"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/promapi"
)

func newAlertsForCheck(_ *promapi.FailoverGroup) checks.RuleChecker {
	return checks.NewAlertsForCheck()
}

func TestAlertsForCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "ignores rules with syntax errors",
			content:     "- alert: foo\n  expr: sum(foo) without(\n",
			checker:     newAlertsForCheck,
			prometheus:  noProm,
		},
		{
			description: "ignores recording rules",
			content:     "- record: foo\n  expr: sum(foo) without(job)\n",
			checker:     newAlertsForCheck,
			prometheus:  noProm,
		},
		{
			description: "invalid for value",
			content:     "- alert: foo\n  expr: foo\n  for: abc\n",
			checker:     newAlertsForCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "negative for value",
			content:     "- alert: foo\n  expr: foo\n  for: -5m\n",
			checker:     newAlertsForCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "default for value",
			content:     "- alert: foo\n  expr: foo\n  for: 0h\n",
			checker:     newAlertsForCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "invalid keep_firing_for value",
			content:     "- alert: foo\n  expr: foo\n  keep_firing_for: abc\n",
			checker:     newAlertsForCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "negative keep_firing_for value",
			content:     "- alert: foo\n  expr: foo\n  keep_firing_for: -5m\n",
			checker:     newAlertsForCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "default for value",
			content:     "- alert: foo\n  expr: foo\n  keep_firing_for: 0h\n",
			checker:     newAlertsForCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "valid for value",
			content:     "- alert: foo\n  expr: foo\n  for: 5m\n",
			checker:     newAlertsForCheck,
			prometheus:  noProm,
		},
		{
			description: "valid keep_firing_for value",
			content:     "- alert: foo\n  expr: foo\n  keep_firing_for: 10m\n",
			checker:     newAlertsForCheck,
			prometheus:  noProm,
		},
	}
	runTests(t, testCases)
}
