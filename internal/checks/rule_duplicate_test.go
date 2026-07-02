package checks_test

import (
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/promapi"
)

func TestRuleDuplicateCheck(t *testing.T) {
	xxxEntries := mustParseContent("- record: foo\n  expr: up == 0\n")
	for i := range xxxEntries {
		xxxEntries[i].Path.Name = "xxx.yml"
	}

	xxxAlerts := mustParseContent("- alert: foo\n  expr: up == 0\n")
	for i := range xxxAlerts {
		xxxAlerts[i].Path.Name = "xxx.yml"
	}

	testCases := []checkTest{
		{
			description: "ignores rules with syntax errors",
			content:     "- record: foo\n  expr: sum(foo) without(\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: newSimpleProm,
		},
		{
			description: "ignores removed entries",
			content:     "- record: foo\n  expr: up == 0\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: newSimpleProm,
			entries: func() []*discovery.Entry {
				entries := mustParseContent("- record: foo\n  expr: up == 0\n")
				for i := range entries {
					entries[i].State = discovery.Removed
				}
				return entries
			}(),
		},
		{
			description: "ignores entries with path errors",
			content:     "- record: foo\n  expr: up == 0\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: newSimpleProm,
			entries: []*discovery.Entry{
				{PathError: errors.New("Mock error")},
			},
		},
		{
			description: "ignores self",
			content:     "- record: foo\n  expr: up == 0\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: newSimpleProm,
			entries:    mustParseContent("- record: foo\n  expr: up == 0\n"),
		},
		{
			description: "skip alerting entries",
			content:     "- record: foo\n  expr: up == 0\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: newSimpleProm,
			entries: mustParseContent(`
- alert: foo
  expr: up == 0
- record: baz
  expr: up == 0
  labels:
    cluster: prod
`),
		},
		{
			description: "skip broken entries",
			content:     "- record: foo\n  expr: up == 0\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: newSimpleProm,
			entries: mustParseContent(`
# foo
- record: foo
  expr: up == 
- record: foo
  exprx: up == 0
`),
		},
		{
			description: "multiple different rules",
			content:     "- record: foo\n  expr: up == 0\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: newSimpleProm,
			entries: mustParseContent(`
- record: bar
  expr: up == 0
  labels:
    cluster: dev
- record: baz
  expr: up == 0
  labels:
    cluster: prod
`),
		},
		{
			description: "multiple rules with different labels",
			content:     "- record: foo\n  expr: up == 0\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: newSimpleProm,
			entries: mustParseContent(`
- record: foo
  expr: up == 0
  labels:
    cluster: dev
- record: foo
  expr: up == 0
  labels:
    cluster: prod
`),
		},
		{
			description: "multiple rules with same labels",
			content:     "- record: foo\n  expr: up == 0\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: newSimpleProm,
			problems:   true,
			entries: mustParseContent(`
- record: foo
  expr: up == 0
  labels:
    cluster: prod
- record: foo
  expr: up == 0
`),
		},
		{
			description: "ignores different Prometheus servers",
			content:     "- record: foo\n  expr: up == 0\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: func(uri string) *promapi.FailoverGroup {
				return promapi.NewFailoverGroup(
					"prom",
					uri,
					[]*promapi.Prometheus{
						promapi.NewPrometheus("prom", uri, simplePromPublicURI, map[string]string{}, time.Second, 4, 100, nil),
					},
					true,
					"up",
					nil,
					[]*regexp.Regexp{regexp.MustCompile(".*")},
					nil,
				)
			},
			entries: xxxEntries,
		},
		{
			description: "same expr but formatted differently",
			content:     "- record: job:up:sum\n  expr: sum(up) by(job)\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: newSimpleProm,
			problems:   true,
			entries: mustParseContent(`
- record: job:up:sum
  expr: sum by(job) (up)
  labels:
    cluster: prod
- record: job:up:sum
  expr: sum by(job) (up)
`),
		},
		{
			description: "ignores rules for different Prometheus servers",
			content:     "- record: foo\n  expr: up == 0\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: func(uri string) *promapi.FailoverGroup {
				return promapi.NewFailoverGroup(
					"prom",
					uri,
					[]*promapi.Prometheus{
						promapi.NewPrometheus("prom", uri, simplePromPublicURI, map[string]string{}, time.Second, 4, 100, nil),
					},
					true,
					"up",
					[]*regexp.Regexp{regexp.MustCompile("fake.yml")},
					nil,
					nil,
				)
			},
			entries: xxxEntries,
		},
		{
			description: "identical alerting rules",
			content:     "- alert: foo\n  expr: up == 0\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: newSimpleProm,
			problems:   true,
			entries: mustParseContent(`
- alert: foo
  expr: up == 0
`),
		},
		{
			description: "identical alerting rules with same labels",
			content:     "- alert: foo\n  expr: up == 0\n  labels:\n    cluster: prod\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: newSimpleProm,
			problems:   true,
			entries: mustParseContent(`
- alert: foo
  expr: up == 0
  labels:
    cluster: prod
`),
		},
		{
			description: "alerting rules with same expr formatted differently",
			content:     "- alert: foo\n  expr: sum(up) by(job) == 0\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: newSimpleProm,
			problems:   true,
			entries: mustParseContent(`
- alert: foo
  expr: sum by(job) (up) == 0
`),
		},
		{
			description: "alerting rules with different labels",
			content:     "- alert: foo\n  expr: up == 0\n  labels:\n    cluster: dev\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: newSimpleProm,
			entries: mustParseContent(`
- alert: foo
  expr: up == 0
  labels:
    cluster: prod
`),
		},
		{
			description: "alerting rules with different alert names",
			content:     "- alert: foo\n  expr: up == 0\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: newSimpleProm,
			entries: mustParseContent(`
- alert: bar
  expr: up == 0
`),
		},
		{
			description: "alerting rules with overlapping selectors",
			content:     "- alert: foo\n  expr: salt_function_running_start_time_unix > 5400\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: newSimpleProm,
			problems:   true,
			entries: mustParseContent(`
- alert: foo
  expr: salt_function_running_start_time_unix{function!="state.highstate"} > 3600
`),
		},
		{
			description: "alerting rules with mismatched selectors",
			content:     "- alert: foo\n  expr: salt_function_running_start_time_unix{function=\"state.highstate\"} > 5400\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: newSimpleProm,
			entries: mustParseContent(`
- alert: foo
  expr: salt_function_running_start_time_unix{function!="state.highstate"} > 3600
`),
		},
		{
			description: "alerting rules with different metrics",
			content:     "- alert: foo\n  expr: up == 0\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: newSimpleProm,
			entries: mustParseContent(`
- alert: foo
  expr: node_up == 0
`),
		},
		{
			description: "alerting rules with same expr and labels but different for",
			content:     "- alert: foo\n  expr: up == 0\n  for: 5m\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: newSimpleProm,
			problems:   true,
			entries: mustParseContent(`
- alert: foo
  expr: up == 0
  for: 10m
`),
		},
		{
			description: "overlapping selectors with different thresholds",
			content:     "- alert: foo\n  expr: node_load1 > 5\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: newSimpleProm,
			problems:   true,
			entries: mustParseContent(`
- alert: foo
  expr: node_load1 > 10
`),
		},
		{
			description: "one selector is a subset of the other via added matcher",
			content:     "- alert: foo\n  expr: node_load1{job=\"node\"} > 5\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: newSimpleProm,
			problems:   true,
			entries: mustParseContent(`
- alert: foo
  expr: node_load1 > 5
`),
		},
		{
			description: "equal matchers with different values mismatch",
			content:     "- alert: foo\n  expr: up{job=\"a\"} == 0\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: newSimpleProm,
			entries: mustParseContent(`
- alert: foo
  expr: up{job="b"} == 0
`),
		},
		{
			description: "regexp matcher overlaps equal matcher",
			content:     "- alert: foo\n  expr: up{job=~\"api.*\"} == 0\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: newSimpleProm,
			problems:   true,
			entries: mustParseContent(`
- alert: foo
  expr: up{job="api-prod"} == 0
`),
		},
		{
			description: "regexp matcher mismatches equal matcher",
			content:     "- alert: foo\n  expr: up{job=~\"api.*\"} == 0\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: newSimpleProm,
			entries: mustParseContent(`
- alert: foo
  expr: up{job="web-prod"} == 0
`),
		},
		{
			description: "negative matcher mismatches equal matcher",
			content:     "- alert: foo\n  expr: up{env!=\"prod\"} == 0\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: newSimpleProm,
			entries: mustParseContent(`
- alert: foo
  expr: up{env="prod"} == 0
`),
		},
		{
			description: "two negative matchers can overlap",
			content:     "- alert: foo\n  expr: up{env!=\"prod\"} == 0\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: newSimpleProm,
			problems:   true,
			entries: mustParseContent(`
- alert: foo
  expr: up{env!="dev"} == 0
`),
		},
		{
			description: "mismatch on one label but overlapping on another is still a mismatch",
			content:     "- alert: foo\n  expr: up{job=\"a\",env=\"prod\"} == 0\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: newSimpleProm,
			entries: mustParseContent(`
- alert: foo
  expr: up{job="b",env="prod"} == 0
`),
		},
		{
			description: "different metric names never overlap",
			content:     "- alert: foo\n  expr: node_load1 > 5\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: newSimpleProm,
			entries: mustParseContent(`
- alert: foo
  expr: node_load5 > 5
`),
		},
		{
			description: "__name__ matcher equivalent to metric name overlaps",
			content:     "- alert: foo\n  expr: node_load1 > 5\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: newSimpleProm,
			problems:   true,
			entries: mustParseContent(`
- alert: foo
  expr: '{__name__="node_load1"} > 10'
`),
		},
		{
			description: "regexp only selectors are not compared",
			content:     "- alert: foo\n  expr: '{__name__=~\"node_load.*\"} > 5'\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: newSimpleProm,
			entries: mustParseContent(`
- alert: foo
  expr: '{__name__=~"node_cpu.*"} > 5'
`),
		},
		{
			description: "aggregations collapsing to a single series collide",
			content:     "- alert: foo\n  expr: sum(rate(errors[5m])) > 5\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: newSimpleProm,
			problems:   true,
			entries: mustParseContent(`
- alert: foo
  expr: sum(rate(errors[10m])) > 10
`),
		},
		{
			description: "binary op between two metrics with same output labels collides",
			content:     "- alert: foo\n  expr: errors / total > 0.1\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: newSimpleProm,
			problems:   true,
			entries: mustParseContent(`
- alert: foo
  expr: errors / total > 0.2
`),
		},
		{
			description: "rate on same metric but different window",
			content:     "- alert: foo\n  expr: rate(http_requests[5m]) > 5\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: newSimpleProm,
			problems:   true,
			entries: mustParseContent(`
- alert: foo
  expr: rate(http_requests[10m]) > 5
`),
		},
		{
			description: "overlapping selectors but different labels do not match",
			content:     "- alert: foo\n  expr: node_load1 > 5\n  labels:\n    team: a\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: newSimpleProm,
			entries: mustParseContent(`
- alert: foo
  expr: node_load1 > 10
  labels:
    team: b
`),
		},
		{
			description: "multi selector expression is not compared",
			content:     "- alert: foo\n  expr: up == 0 or node_up == 0\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: newSimpleProm,
			entries: mustParseContent(`
- alert: foo
  expr: up == 0
`),
		},
		{
			description: "same rule deployed via symlink to same Prometheus is a duplicate",
			content:     "- alert: foo\n  expr: up == 0\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: newSimpleProm,
			problems:   true,
			entries: func() []*discovery.Entry {
				entries := mustParseContent("- alert: foo\n  expr: up == 0\n")
				for i := range entries {
					entries[i].Path.Name = "symlink.yml"
				}
				return entries
			}(),
		},
		{
			description: "identical alerting rule on a different Prometheus is ignored",
			content:     "- alert: foo\n  expr: up == 0\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: func(uri string) *promapi.FailoverGroup {
				return promapi.NewFailoverGroup(
					"prom",
					uri,
					[]*promapi.Prometheus{
						promapi.NewPrometheus("prom", uri, simplePromPublicURI, map[string]string{}, time.Second, 4, 100, nil),
					},
					true,
					"up",
					nil,
					[]*regexp.Regexp{regexp.MustCompile(".*")},
					nil,
				)
			},
			entries: xxxAlerts,
		},
		{
			description: "identical alerting rule for a different Prometheus is ignored",
			content:     "- alert: foo\n  expr: up == 0\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: func(uri string) *promapi.FailoverGroup {
				return promapi.NewFailoverGroup(
					"prom",
					uri,
					[]*promapi.Prometheus{
						promapi.NewPrometheus("prom", uri, simplePromPublicURI, map[string]string{}, time.Second, 4, 100, nil),
					},
					true,
					"up",
					[]*regexp.Regexp{regexp.MustCompile("fake.yml")},
					nil,
					nil,
				)
			},
			entries: xxxAlerts,
		},
	}

	runTests(t, testCases)
}
