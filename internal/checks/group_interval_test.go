package checks_test

import (
	"testing"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/promapi"
)

func newGroupIntervalCheck(_ *promapi.FailoverGroup) checks.RuleChecker {
	return checks.NewGroupIntervalCheck()
}

func TestGroupIntervalCheck(t *testing.T) {
	testCases := []checkTest{
		{
			// Rules with syntax errors should be silently skipped.
			description: "ignores rules with syntax errors",
			content: `
- alert: foo
  expr: sum(foo) without(
`,
			checker:    newGroupIntervalCheck,
			prometheus: noProm,
		},
		{
			// Recording rule with no group interval set should not report.
			description: "no interval set on recording rule",
			content: `
- record: foo
  expr: sum(foo) without(job)
`,
			checker:    newGroupIntervalCheck,
			prometheus: noProm,
		},
		{
			// Alerting rule with no group interval set should not report.
			description: "no interval set on alerting rule",
			content: `
- alert: foo
  expr: foo > 0
`,
			checker:    newGroupIntervalCheck,
			prometheus: noProm,
		},
		{
			// Group interval exactly at the 5m limit should not report.
			description: "interval equal to 5m",
			content: `
- name: test
  interval: 5m
  rules:
  - alert: foo
    expr: foo > 0
`,
			checker:    newGroupIntervalCheck,
			prometheus: noProm,
		},
		{
			// Group interval below the 5m limit should not report.
			description: "interval below 5m",
			content: `
- name: test
  interval: 1m
  rules:
  - alert: foo
    expr: foo > 0
`,
			checker:    newGroupIntervalCheck,
			prometheus: noProm,
		},
		{
			// Group interval above the 5m limit should report a warning on alerting rules.
			description: "interval above 5m on alerting rule",
			content: `
- name: test
  interval: 9m
  rules:
  - alert: foo
    expr: foo > 0
`,
			checker:    newGroupIntervalCheck,
			prometheus: noProm,
			problems:   true,
		},
		{
			// Group interval above the 5m limit should report a warning on recording rules.
			description: "interval above 5m on recording rule",
			content: `
- name: test
  interval: 9m
  rules:
  - record: foo
    expr: sum(foo) without(job)
`,
			checker:    newGroupIntervalCheck,
			prometheus: noProm,
			problems:   true,
		},
		{
			// Alerting rule with keep_firing_for >= interval should not report.
			description: "interval above 5m but keep_firing_for covers it",
			content: `
- name: test
  interval: 10m
  rules:
  - alert: foo
    expr: foo > 0
    keep_firing_for: 10m
`,
			checker:    newGroupIntervalCheck,
			prometheus: noProm,
		},
		{
			// Alerting rule with keep_firing_for > interval should not report.
			description: "interval above 5m but keep_firing_for exceeds it",
			content: `
- name: test
  interval: 10m
  rules:
  - alert: foo
    expr: foo > 0
    keep_firing_for: 15m
`,
			checker:    newGroupIntervalCheck,
			prometheus: noProm,
		},
		{
			// Alerting rule with keep_firing_for < interval should still report.
			description: "interval above 5m and keep_firing_for too short",
			content: `
- name: test
  interval: 10m
  rules:
  - alert: foo
    expr: foo > 0
    keep_firing_for: 5m
`,
			checker:    newGroupIntervalCheck,
			prometheus: noProm,
			problems:   true,
		},
		{
			// Recording rule with keep_firing_for is not valid syntax, interval still reports.
			description: "interval above 5m on recording rule ignores keep_firing_for",
			content: `
- name: test
  interval: 10m
  rules:
  - record: foo
    expr: sum(foo) without(job)
`,
			checker:    newGroupIntervalCheck,
			prometheus: noProm,
			problems:   true,
		},
	}
	runTests(t, testCases)
}
