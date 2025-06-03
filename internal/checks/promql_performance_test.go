package checks_test

import (
	"errors"
	"testing"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/promapi"
)

func TestPerformanceCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "ignores rules with syntax errors",
			content:     "- record: foo\n  expr: sum(foo) without(\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewPerformanceCheck(prom)
			},
			prometheus: newSimpleProm,
		},
		{
			description: "ignores entries with path errors",
			content:     "- record: foo\n  expr: up == 0\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewPerformanceCheck(prom)
			},
			prometheus: newSimpleProm,
			entries: []discovery.Entry{
				{PathError: errors.New("Mock error")},
			},
		},
		{
			description: "ignores self",
			content:     "- record: foo\n  expr: up == 0\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewPerformanceCheck(prom)
			},
			prometheus: newSimpleProm,
			entries:    mustParseContent("- record: foo\n  expr: up == 0\n"),
		},
		{
			description: "suggest recording rule / aggregation",
			content:     "- alert: foo\n  expr: sum(rate(foo_total[5m])) without(instance) > 10\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewPerformanceCheck(prom)
			},
			prometheus: newSimpleProm,
			entries: mustParseContent(`
- alert: foo
  expr: vector(1)
- record: colo:foo
  expr: sum(rate(foo_total[5m])) without(instance)
`),
			problems: true,
		},
		{
			description: "suggest recording rule / rate",
			content:     "- alert: foo\n  expr: sum(rate(foo_total[5m])) without(instance) > 10\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewPerformanceCheck(prom)
			},
			prometheus: newSimpleProm,
			entries: mustParseContent(`
- record: foo:rate5m
  expr: rate(foo_total[5m])
`),
			problems: true,
		},
		{
			description: "suggest recording rule / ignore vector",
			content:     "- alert: foo\n  expr: sum(rate(foo_total[5m]) or vector(0)) without(instance) > 10\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewPerformanceCheck(prom)
			},
			prometheus: newSimpleProm,
			entries: mustParseContent(`
- record: colo:foo
  expr: vector(0)
`),
		},
		{
			description: "suggest recording rule / ignore selector",
			content:     "- alert: foo\n  expr: sum(foo == 1) without(instance) > 10\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewPerformanceCheck(prom)
			},
			prometheus: newSimpleProm,
			entries: mustParseContent(`
- record: colo:foo
  expr: foo == 1
`),
		},
		{
			description: "suggest recording rule / ignore multi-source",
			content:     "- alert: foo\n  expr: sum(foo == 1) without(instance) > 10\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewPerformanceCheck(prom)
			},
			prometheus: newSimpleProm,
			entries: mustParseContent(`
- record: colo:foo
  expr: foo == 1 or bar == 1
`),
		},
	}
	runTests(t, testCases)
}
