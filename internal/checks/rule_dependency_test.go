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

func TestRuleDependencyCheck(t *testing.T) {
	parseWithState := func(input string, state discovery.ChangeType, sp, rp string) []discovery.Entry {
		entries := mustParseContent(input)
		for i := range entries {
			entries[i].State = state
			entries[i].Path.Name = sp
			entries[i].Path.SymlinkTarget = rp

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
		},
		{
			description: "ignores rules with syntax errors",
			content:     "- record: foo\n  expr: sum(foo) without(\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDependencyCheck()
			},
			prometheus: newSimpleProm,
		},
		{
			description: "ignores alerts with expr errors",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDependencyCheck()
			},
			prometheus: newSimpleProm,
			entries: []discovery.Entry{
				parseWithState("- record: foo\n  expr: sum(foo)\n", discovery.Removed, "foo.yaml", "foo.yaml")[0],
				parseWithState("- alert: foo\n  expr: foo ==\n", discovery.Noop, "foo.yaml", "foo.yaml")[0],
			},
		},
		{
			description: "ignores alerts without dependencies",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDependencyCheck()
			},
			prometheus: newSimpleProm,
			entries: []discovery.Entry{
				parseWithState("- record: foo\n  expr: sum(foo)\n", discovery.Removed, "foo.yaml", "foo.yaml")[0],
				parseWithState("- alert: foo\n  expr: up == 0\n", discovery.Noop, "foo.yaml", "foo.yaml")[0],
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
						promapi.NewPrometheus("prom", uri, simplePromPublicURI, map[string]string{"X-Debug": "1"}, time.Second, 16, 1000, nil),
					},
					true,
					"up",
					[]*regexp.Regexp{},
					[]*regexp.Regexp{regexp.MustCompile("excluded.yml")},
					[]string{},
				)
			},
			problems: true,
			entries: []discovery.Entry{
				parseWithState("- record: foo\n  expr: sum(foo)\n", discovery.Removed, "foo.yaml", "foo.yaml")[0],
				parseWithState("- alert: alert\n  expr: foo == 0\n", discovery.Noop, "excluded.yaml", "excluded.yaml")[0],
			},
		},
		{
			description: "warns about removed dependency",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDependencyCheck()
			},
			prometheus: newSimpleProm,
			problems:   true,
			entries: []discovery.Entry{
				parseWithState("- record: foo\n  expr: sum(foo)\n", discovery.Removed, "foo.yaml", "foo.yaml")[0],
				parseWithState("- alert: alert\n  expr: foo == 0\n", discovery.Noop, "foo.yaml", "foo.yaml")[0],
			},
		},
		{
			description: "ignores unparsable files",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDependencyCheck()
			},
			prometheus: newSimpleProm,
			entries: []discovery.Entry{
				{
					Path: discovery.Path{
						Name:          "broken.yaml",
						SymlinkTarget: "broken.yaml",
					},
					PathError: errors.New("bad file"),
				},
				parseWithState("- alert: foo\n  expr: up == 0\n", discovery.Noop, "foo.yaml", "foo.yaml")[0],
			},
		},
		{
			description: "ignores rules with errors",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDependencyCheck()
			},
			prometheus: newSimpleProm,
			entries: []discovery.Entry{
				parseWithState("- recordx: foo\n  expr: sum(foo)\n", discovery.Noop, "foo.yaml", "foo.yaml")[0],
				parseWithState("- alert: foo\n  expr: up == 0\n", discovery.Noop, "foo.yaml", "foo.yaml")[0],
			},
		},
		{
			description: "deduplicates affected files",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDependencyCheck()
			},
			prometheus: newSimpleProm,
			problems:   true,
			entries: []discovery.Entry{
				parseWithState("\n\n- alert: alert\n  expr: (foo / foo) == 0\n- alert: alert\n  expr: (foo / foo) == 0\n", discovery.Noop, "alice.yaml", "alice.yaml")[1],
				parseWithState("\n\n- alert: alert\n  expr: (foo / foo) == 0\n- alert: alert\n  expr: (foo / foo) == 0\n", discovery.Noop, "alice.yaml", "alice.yaml")[0],
				parseWithState("- alert: alert\n  expr: (foo / foo) == 0\n", discovery.Noop, "symlink3.yaml", "bar.yaml")[0],
				parseWithState("- record: foo\n  expr: sum(foo)\n", discovery.Removed, "foo.yaml", "foo.yaml")[0],
				parseWithState("- alert: alert\n  expr: foo == 0\n", discovery.Noop, "foo.yaml", "foo.yaml")[0],
				parseWithState("- alert: xxx\n  expr: (foo / foo) == 0\n", discovery.Noop, "bar.yaml", "bar.yaml")[0],
				parseWithState("- alert: alert\n  expr: (foo / foo) == 0\n", discovery.Noop, "bar.yaml", "bar.yaml")[0],
				parseWithState("- alert: alert\n  expr: foo == 0\n", discovery.Noop, "symlink1.yaml", "foo.yaml")[0],
				parseWithState("- alert: alert\n  expr: foo == 0\n", discovery.Noop, "symlink2.yaml", "foo.yaml")[0],
			},
		},
		{
			description: "warns about removed alert used in ALERTS{}",
			content:     "- alert: TargetIsDown\n  expr: up == 0\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDependencyCheck()
			},
			prometheus: newSimpleProm,
			problems:   true,
			entries: []discovery.Entry{
				parseWithState(`
- record: alert:count
  expr: count(ALERTS{alertname="TargetIsDown"})
`, discovery.Noop, "foo.yaml", "foo.yaml")[0],
			},
		},
		{
			description: "warns about removed alert used in ALERTS_FOR_STATE{}",
			content:     "- alert: TargetIsDown\n  expr: up == 0\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDependencyCheck()
			},
			prometheus: newSimpleProm,
			problems:   true,
			entries: []discovery.Entry{
				parseWithState(`
- record: alert:count
  expr: count(ALERTS_FOR_STATE{alertname="TargetIsDown"})
`, discovery.Noop, "foo.yaml", "foo.yaml")[0],
			},
		},
		{
			description: "ignores removed unused alert",
			content:     "- alert: TargetIsDown\n  expr: up == 0\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDependencyCheck()
			},
			prometheus: newSimpleProm,
			entries: []discovery.Entry{
				parseWithState(`
- record: alert:count
  expr: count(ALERTS{alertname!="TargetIsDown"})
`, discovery.Noop, "foo.yaml", "foo.yaml")[0],
			},
		},
		{
			description: "warns about removed dependency without crashing if it is not the last rule",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDependencyCheck()
			},
			prometheus: newSimpleProm,
			problems:   true,
			entries: []discovery.Entry{
				parseWithState("- record: foo\n  expr: sum(foo)\n", discovery.Removed, "foo.yaml", "foo.yaml")[0],
				parseWithState("- alert: alert\n  expr: foo == 0\n", discovery.Noop, "foo.yaml", "foo.yaml")[0],
				parseWithState("- record: bar\n  expr: vector(0)\n", discovery.Noop, "foo.yaml", "foo.yaml")[0],
			},
		},
		{
			description: "ignores re-added rules",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDependencyCheck()
			},
			prometheus: newSimpleProm,
			entries: []discovery.Entry{
				parseWithState("- record: foo\n  expr: sum(foo)\n", discovery.Removed, "foo.yaml", "foo.yaml")[0],
				parseWithState("- alert: alert\n  expr: foo == 0\n", discovery.Noop, "foo.yaml", "foo.yaml")[0],
				parseWithState("- record: foo\n  expr: sum(foo)\n", discovery.Added, "bar.yaml", "foo.yaml")[0],
			},
		},
	}

	runTests(t, testCases)
}
