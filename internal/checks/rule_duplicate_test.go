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
	}

	runTests(t, testCases)
}
