package checks_test

import (
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/promapi"
)

func textDependencyRule(path, name string) string {
	return fmt.Sprintf("This rule uses a metric produced by recording rule `%s` which was removed from %s.", name, path)
}

func detailsDependencyRule(name string) string {
	return fmt.Sprintf("If you remove the recording rule generating `%s` and there is no other source of `%s` metric, then this and other rule depending on `%s` will break.", name, name, name)
}

func TestRuleDependencyCheck(t *testing.T) {
	parseWithState := func(input string, state discovery.ChangeType, path string) []discovery.Entry {
		entries := mustParseContent(input)
		for i := range entries {
			entries[i].State = state
			entries[i].ReportedPath = path
			entries[i].SourcePath = path
		}
		return entries
	}

	testCases := []checkTest{
		{
			description: "ignores alerting rules",
			content:     "- alert: foo\n  expr: up == 0\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDependencyCheck()
			},
			prometheus: newSimpleProm,
			problems:   noProblems,
		},
		{
			description: "ignores rules with syntax errors",
			content:     "- record: foo\n  expr: sum(foo) without(\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDependencyCheck()
			},
			prometheus: newSimpleProm,
			problems:   noProblems,
		},
		{
			description: "ignores alerts with expr errors",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDependencyCheck()
			},
			prometheus: newSimpleProm,
			problems:   noProblems,
			entries: []discovery.Entry{
				parseWithState("- record: foo\n  expr: sum(foo)\n", discovery.Removed, "foo.yaml")[0],
				parseWithState("- alert: foo\n  expr: foo ==\n", discovery.Noop, "foo.yaml")[0],
			},
		},
		{
			description: "ignores alerts without dependencies",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDependencyCheck()
			},
			prometheus: newSimpleProm,
			problems:   noProblems,
			entries: []discovery.Entry{
				parseWithState("- record: foo\n  expr: sum(foo)\n", discovery.Removed, "foo.yaml")[0],
				parseWithState("- alert: foo\n  expr: up == 0\n", discovery.Noop, "foo.yaml")[0],
			},
		},
		{
			description: "includes alerts on other prometheus servers",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDependencyCheck()
			},
			prometheus: func(uri string) *promapi.FailoverGroup {
				return promapi.NewFailoverGroup(
					"prom",
					uri,
					[]*promapi.Prometheus{
						promapi.NewPrometheus("prom", uri, "", map[string]string{"X-Debug": "1"}, time.Second, 16, 1000, nil),
					},
					true,
					"up",
					[]*regexp.Regexp{},
					[]*regexp.Regexp{regexp.MustCompile("excluded.yml")},
					[]string{},
				)
			},
			problems: func(s string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "expr: foo == 0",
						Lines:    []int{2},
						Reporter: checks.RuleDependencyCheckName,
						Text:     textDependencyRule("fake.yml", "foo"),
						Details:  detailsDependencyRule("foo"),
						Severity: checks.Warning,
					},
				}
			},
			entries: []discovery.Entry{
				parseWithState("- record: foo\n  expr: sum(foo)\n", discovery.Removed, "foo.yaml")[0],
				parseWithState("- alert: foo\n  expr: foo == 0\n", discovery.Noop, "excluded.yaml")[0],
			},
		},
		{
			description: "warns about removed dependency",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDependencyCheck()
			},
			prometheus: newSimpleProm,
			problems: func(s string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "expr: foo == 0",
						Lines:    []int{2},
						Reporter: checks.RuleDependencyCheckName,
						Text:     textDependencyRule("fake.yml", "foo"),
						Details:  detailsDependencyRule("foo"),
						Severity: checks.Warning,
					},
				}
			},
			entries: []discovery.Entry{
				parseWithState("- record: foo\n  expr: sum(foo)\n", discovery.Removed, "foo.yaml")[0],
				parseWithState("- alert: foo\n  expr: foo == 0\n", discovery.Noop, "foo.yaml")[0],
			},
		},
	}

	runTests(t, testCases)
}
