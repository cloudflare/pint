package checks_test

import (
	"errors"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
)

func textDuplicateRule(path string, line int) string {
	return fmt.Sprintf("Duplicated rule, identical rule found at %s:%d.", path, line)
}

func TestRuleDuplicateCheck(t *testing.T) {
	xxxEntries := mustParseContent("- record: foo\n  expr: up == 0\n")
	for i := range xxxEntries {
		xxxEntries[i].Path.SymlinkTarget = "xxx.yml"
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
			description: "ignores entries with path errors",
			content:     "- record: foo\n  expr: up == 0\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: newSimpleProm,
			problems:   noProblems,
			entries: []discovery.Entry{
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
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 1,
							Last:  2,
						},
						Reporter: checks.RuleDuplicateCheckName,
						Summary:  "duplicated recording rule",
						Severity: checks.Bug,
						Diagnostics: []output.Diagnostic{
							{
								Message: textDuplicateRule("fake.yml", 6),
							},
						},
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
					uri,
					[]*promapi.Prometheus{
						promapi.NewPrometheus("prom", uri, "", map[string]string{}, time.Second, 4, 100, nil),
					},
					true,
					"up",
					nil,
					[]*regexp.Regexp{regexp.MustCompile(".*")},
					nil,
				)
			},
			problems: noProblems,
			entries:  xxxEntries,
		},
		{
			description: "same expr but formatted differently",
			content:     "- record: job:up:sum\n  expr: sum(up) by(job)\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDuplicateCheck(prom)
			},
			prometheus: newSimpleProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 1,
							Last:  2,
						},
						Reporter: checks.RuleDuplicateCheckName,
						Summary:  "duplicated recording rule",
						Severity: checks.Bug,
						Diagnostics: []output.Diagnostic{
							{
								Message: textDuplicateRule("fake.yml", 6),
							},
						},
					},
				}
			},
			entries: mustParseContent(`
- record: job:up:sum
  expr: sum by(job) (up)
  labels:
    cluster: prod
- record: job:up:sum
  expr: sum by(job) (up)
`),
		},
	}

	runTests(t, testCases)
}
