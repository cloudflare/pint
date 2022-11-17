package checks_test

import (
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/promapi"
)

func textDuplicateRule(path string, line int) string {
	return fmt.Sprintf("duplicated rule, identical rule found at %s:%d", path, line)
}

func TestRuleDuplicateCheck(t *testing.T) {
	xxxEntries := mustParseContent("- record: foo\n  expr: up == 0\n")
	for i := range xxxEntries {
		xxxEntries[i].ReportedPath = "xxx.yml"
	}

	testCases := []checkTest{
		{
			description: "ignores rules with syntax errors",
			content:     "- record: foo\n  expr: sum(foo) without(\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: newSimpleProm,
			problems:   noProblems,
		},
		{
			description: "ignores alerting rules",
			content:     "- alert: foo\n  expr: up == 0\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: newSimpleProm,
			problems:   noProblems,
		},
		{
			description: "ignores self",
			content:     "- record: foo\n  expr: up == 0\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: newSimpleProm,
			problems:   noProblems,
			entries:    mustParseContent("- record: foo\n  expr: up == 0\n"),
		},
		{
			description: "skip alerting entries",
			content:     "- record: foo\n  expr: up == 0\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: newSimpleProm,
			problems:   noProblems,
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
			problems:   noProblems,
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
			problems:   noProblems,
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
			problems:   noProblems,
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
			problems: func(s string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "record: foo",
						Lines:    []int{1, 2},
						Reporter: checks.RuleDuplicateCheckName,
						Text:     textDuplicateRule("fake.yml", 6),
						Severity: checks.Bug,
					},
				}
			},
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
					[]*promapi.Prometheus{
						promapi.NewPrometheus("prom", uri, map[string]string{}, time.Second, 4, 100),
					},
					1000,
					true,
					"up",
					nil,
					[]*regexp.Regexp{regexp.MustCompile(".*")},
				)
			},
			problems: noProblems,
			entries:  xxxEntries,
		},
	}

	runTests(t, testCases)
}
