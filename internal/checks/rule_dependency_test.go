package checks_test

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/promapi"
)

func TestRuleDependencyCheck(t *testing.T) {
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
			entries: []*discovery.Entry{
				parseWithStatePath("- record: foo\n  expr: sum(foo)\n", discovery.Removed, "foo.yaml", "foo.yaml")[0],
				parseWithStatePath("- alert: foo\n  expr: foo ==\n", discovery.Noop, "foo.yaml", "foo.yaml")[0],
			},
		},
		{
			description: "ignores alerts without dependencies",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDependencyCheck()
			},
			prometheus: newSimpleProm,
			entries: []*discovery.Entry{
				parseWithStatePath("- record: foo\n  expr: sum(foo)\n", discovery.Removed, "foo.yaml", "foo.yaml")[0],
				parseWithStatePath("- alert: foo\n  expr: up == 0\n", discovery.Noop, "foo.yaml", "foo.yaml")[0],
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
			entries: []*discovery.Entry{
				parseWithStatePath("- record: foo\n  expr: sum(foo)\n", discovery.Removed, "foo.yaml", "foo.yaml")[0],
				parseWithStatePath("- alert: alert\n  expr: foo == 0\n", discovery.Noop, "excluded.yaml", "excluded.yaml")[0],
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
			entries: []*discovery.Entry{
				parseWithStatePath("- record: foo\n  expr: sum(foo)\n", discovery.Removed, "foo.yaml", "foo.yaml")[0],
				parseWithStatePath("- alert: alert\n  expr: foo == 0\n", discovery.Noop, "foo.yaml", "foo.yaml")[0],
			},
		},
		{
			description: "ignores unparsable files",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDependencyCheck()
			},
			prometheus: newSimpleProm,
			entries: []*discovery.Entry{
				{
					Path: discovery.Path{
						Name:          "broken.yaml",
						SymlinkTarget: "broken.yaml",
					},
					PathError: errors.New("bad file"),
				},
				parseWithStatePath("- alert: foo\n  expr: up == 0\n", discovery.Noop, "foo.yaml", "foo.yaml")[0],
			},
		},
		{
			description: "ignores rules with errors",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDependencyCheck()
			},
			prometheus: newSimpleProm,
			entries: []*discovery.Entry{
				parseWithStatePath("- recordx: foo\n  expr: sum(foo)\n", discovery.Noop, "foo.yaml", "foo.yaml")[0],
				parseWithStatePath("- alert: foo\n  expr: up == 0\n", discovery.Noop, "foo.yaml", "foo.yaml")[0],
			},
		},
		{
			description: "ignores rules with invalid queries",
			content:     "- alert: myalert\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDependencyCheck()
			},
			prometheus: newSimpleProm,
			entries: []*discovery.Entry{
				parseWithStatePath("- record: foo\n  expr: sum()\n", discovery.Noop, "foo.yaml", "foo.yaml")[0],
				parseWithStatePath("- alert: foo\n  expr: up +\n", discovery.Noop, "foo.yaml", "foo.yaml")[0],
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
			entries: []*discovery.Entry{
				parseWithStatePath("\n\n- alert: alert\n  expr: (foo / foo) == 0\n- alert: alert\n  expr: (foo / foo) == 0\n", discovery.Noop, "alice.yaml", "alice.yaml")[1],
				parseWithStatePath("\n\n- alert: alert\n  expr: (foo / foo) == 0\n- alert: alert\n  expr: (foo / foo) == 0\n", discovery.Noop, "alice.yaml", "alice.yaml")[0],
				parseWithStatePath("- alert: alert\n  expr: (foo / foo) == 0\n", discovery.Noop, "symlink3.yaml", "bar.yaml")[0],
				parseWithStatePath("- record: foo\n  expr: sum(foo)\n", discovery.Removed, "foo.yaml", "foo.yaml")[0],
				parseWithStatePath("- alert: alert\n  expr: foo == 0\n", discovery.Noop, "foo.yaml", "foo.yaml")[0],
				parseWithStatePath("- alert: xxx\n  expr: (foo / foo) == 0\n", discovery.Noop, "bar.yaml", "bar.yaml")[0],
				parseWithStatePath("- alert: alert\n  expr: (foo / foo) == 0\n", discovery.Noop, "bar.yaml", "bar.yaml")[0],
				parseWithStatePath("- alert: alert\n  expr: foo == 0\n", discovery.Noop, "symlink1.yaml", "foo.yaml")[0],
				parseWithStatePath("- alert: alert\n  expr: foo == 0\n", discovery.Noop, "symlink2.yaml", "foo.yaml")[0],
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
			entries: []*discovery.Entry{
				parseWithStatePath(`
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
			entries: []*discovery.Entry{
				parseWithStatePath(`
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
			entries: []*discovery.Entry{
				parseWithStatePath(`
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
			entries: []*discovery.Entry{
				parseWithStatePath("- record: foo\n  expr: sum(foo)\n", discovery.Removed, "foo.yaml", "foo.yaml")[0],
				parseWithStatePath("- alert: alert\n  expr: foo == 0\n", discovery.Noop, "foo.yaml", "foo.yaml")[0],
				parseWithStatePath("- record: bar\n  expr: vector(0)\n", discovery.Noop, "foo.yaml", "foo.yaml")[0],
			},
		},
		{
			description: "ignores re-added rules",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDependencyCheck()
			},
			prometheus: newSimpleProm,
			entries: []*discovery.Entry{
				parseWithStatePath("- record: foo\n  expr: sum(foo)\n", discovery.Removed, "foo.yaml", "foo.yaml")[0],
				parseWithStatePath("- alert: alert\n  expr: foo == 0\n", discovery.Noop, "foo.yaml", "foo.yaml")[0],
				parseWithStatePath("- record: foo\n  expr: sum(foo)\n", discovery.Added, "bar.yaml", "foo.yaml")[0],
			},
		},
		{
			// Alert uses metric from recording rule in a different group - no warning for alerting rules.
			description: "ignores cross-group dependency for alerting rules / different groups in same file",
			content: `groups:
- name: alerts
  rules:
  - alert: foo too high
    expr: foo:sum > 100
`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDependencyCheck()
			},
			prometheus: newSimpleProm,
			entries: parseWithStatePath(`groups:
- name: recordings
  rules:
  - record: foo:sum
    expr: sum(foo)
`, discovery.Noop, "rules.yaml", "rules.yaml"),
		},
		{
			// Alert uses metric from recording rule in the same group - no warning.
			description: "ignores same-group dependency",
			content: `groups:
- name: mygroup
  rules:
  - record: foo:sum
    expr: sum(foo)
  - alert: foo too high
    expr: foo:sum > 100
`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDependencyCheck()
			},
			prometheus: newSimpleProm,
			problems:   false,
		},
		{
			// Recording rule uses metric from another recording rule in a different group - should warn.
			description: "warns about cross-group dependency / recording rule depends on recording rule",
			content: `groups:
- name: aggregations
  rules:
  - record: foo:rate
    expr: rate(foo:sum[5m])
`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDependencyCheck()
			},
			prometheus: newSimpleProm,
			problems:   true,
			entries: parseWithStatePath(`groups:
- name: base
  rules:
  - record: foo:sum
    expr: sum(foo)
`, discovery.Noop, "base.yaml", "base.yaml"),
		},
		{
			// Recording rule uses multiple metrics from recording rules in a different group.
			description: "warns about multiple cross-group dependencies for recording rules",
			content: `groups:
- name: aggregations
  rules:
  - record: combined:sum
    expr: foo:sum + bar:sum
`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDependencyCheck()
			},
			prometheus: newSimpleProm,
			problems:   true,
			entries: parseWithStatePath(`groups:
- name: base
  rules:
  - record: bar:sum
    expr: sum(bar)
  - record: foo:sum
    expr: sum(foo)
`, discovery.Noop, "base.yaml", "base.yaml"),
		},
		{
			// Alert with cross-group dependency across different files - no warning for alerting rules.
			description: "ignores cross-group dependency for alerting rules / different files",
			content: `groups:
- name: alerts
  rules:
  - alert: high error rate
    expr: error:rate5m > 0.1
`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDependencyCheck()
			},
			prometheus: newSimpleProm,
			entries: parseWithStatePath(`groups:
- name: recordings
  rules:
  - record: error:rate5m
    expr: rate(errors_total[5m])
`, discovery.Noop, "other.yaml", "other.yaml"),
		},
		{
			// Alert with multiple cross-group dependencies - no warning for alerting rules.
			description: "ignores multiple cross-group dependencies for alerting rules",
			content: `groups:
- name: alerts
  rules:
  - alert: combined
    expr: foo:sum + bar:sum
`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDependencyCheck()
			},
			prometheus: newSimpleProm,
			entries: parseWithStatePath(`groups:
- name: recordings
  rules:
  - record: bar:sum
    expr: sum(bar)
  - record: foo:sum
    expr: sum(foo)
`, discovery.Noop, "other.yaml", "other.yaml"),
		},
		{
			// Rule uses metric that is not from any recording rule - no warning.
			description: "ignores metrics not from recording rules",
			content: `groups:
- name: alerts
  rules:
  - alert: high cpu
    expr: cpu_usage > 0.9
`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDependencyCheck()
			},
			prometheus: newSimpleProm,
			problems:   false,
		},
		{
			// Rule with syntax error - no warning.
			description: "ignores rules with syntax errors in cross-group check",
			content: `groups:
- name: alerts
  rules:
  - alert: broken
    expr: foo:sum >
`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDependencyCheck()
			},
			prometheus: newSimpleProm,
			problems:   false,
			entries: parseWithStatePath(`groups:
- name: recordings
  rules:
  - record: foo:sum
    expr: sum(foo)
`, discovery.Noop, "rules.yaml", "rules.yaml"),
		},
		{
			// Expression with scalar - no VectorSelector, should not crash.
			description: "handles expressions without vector selectors",
			content: `groups:
- name: alerts
  rules:
  - alert: always
    expr: vector(1) > 0
`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDependencyCheck()
			},
			prometheus: newSimpleProm,
			problems:   false,
			entries: parseWithStatePath(`groups:
- name: recordings
  rules:
  - record: foo:sum
    expr: sum(foo)
`, discovery.Noop, "rules.yaml", "rules.yaml"),
		},
		{
			description: "ignores cross-group dependency when other rule produces a static value",
			content: `groups:
- name: aggregations
  rules:
  - record: foo:rate
    expr: rate(foo:sum[5m])
`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDependencyCheck()
			},
			prometheus: newSimpleProm,
			entries: parseWithStatePath(`groups:
- name: base
  rules:
  - record: foo:sum
    expr: vector(1)
`, discovery.Noop, "base.yaml", "base.yaml"),
		},
		{
			description: "ignores cross-group dependency when other rule produces a computed static value",
			content: `groups:
- name: aggregations
  rules:
  - record: foo:rate
    expr: rate(foo:sum[5m])
`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDependencyCheck()
			},
			prometheus: newSimpleProm,
			entries: parseWithStatePath(`groups:
- name: base
  rules:
  - record: foo:sum
    expr: vector(1) + vector(2)
`, discovery.Noop, "base.yaml", "base.yaml"),
		},
		{
			description: "ignores cross-group dependency when other rule has syntax error",
			content: `groups:
- name: aggregations
  rules:
  - record: foo:rate
    expr: rate(foo:sum[5m])
`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDependencyCheck()
			},
			prometheus: newSimpleProm,
			entries: parseWithStatePath(`groups:
- name: base
  rules:
  - record: foo:sum
    expr: foo !=! bar
`, discovery.Noop, "base.yaml", "base.yaml"),
		},
		{
			description: "warns about non-static cross-group dependency but ignores static one",
			content: `groups:
- name: aggregations
  rules:
  - record: combined:rate
    expr: foo:sum + bar:sum
`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDependencyCheck()
			},
			prometheus: newSimpleProm,
			problems:   true,
			entries: parseWithStatePath(`groups:
- name: base
  rules:
  - record: bar:sum
    expr: vector(0)
  - record: foo:sum
    expr: sum(foo)
`, discovery.Noop, "base.yaml", "base.yaml"),
		},
		{
			description: "ignores foo:sum cross-group dependency when foo:sum is in ignoreGroupMismatch",
			content: `groups:
- name: aggregations
  rules:
  - record: foo:rate
    expr: rate(foo:sum[5m])
`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDependencyCheck()
			},
			ctx: func(ctx context.Context, _ string) context.Context {
				s := checks.RuleDependencySettings{
					IgnoreGroupMismatch: []string{"foo:sum"},
				}
				if err := s.Validate(); err != nil {
					t.Error(err)
					t.FailNow()
				}
				return context.WithValue(ctx, checks.SettingsKey(checks.RuleDependencyCheckName), &s)
			},
			prometheus: newSimpleProm,
			entries: parseWithStatePath(`groups:
- name: base
  rules:
  - record: foo:sum
    expr: sum(foo)
`, discovery.Noop, "base.yaml", "base.yaml"),
		},
		{
			description: "warns about bar:sum but ignores foo:sum when only foo:.* is in ignoreGroupMismatch",
			content: `groups:
- name: aggregations
  rules:
  - record: combined:rate
    expr: rate(foo:sum[5m]) + rate(bar:sum[5m])
`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDependencyCheck()
			},
			ctx: func(ctx context.Context, _ string) context.Context {
				s := checks.RuleDependencySettings{
					IgnoreGroupMismatch: []string{"foo:.*"},
				}
				if err := s.Validate(); err != nil {
					t.Error(err)
					t.FailNow()
				}
				return context.WithValue(ctx, checks.SettingsKey(checks.RuleDependencyCheckName), &s)
			},
			prometheus: newSimpleProm,
			problems:   true,
			entries: parseWithStatePath(`groups:
- name: base
  rules:
  - record: foo:sum
    expr: sum(foo)
  - record: bar:sum
    expr: sum(bar)
`, discovery.Noop, "base.yaml", "base.yaml"),
		},
		{
			description: "ignores foo:sum cross-group dependency when foo:.* is in ignoreGroupMismatch",
			content: `groups:
- name: aggregations
  rules:
  - record: foo:rate
    expr: rate(foo:sum[5m])
`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDependencyCheck()
			},
			ctx: func(ctx context.Context, _ string) context.Context {
				s := checks.RuleDependencySettings{
					IgnoreGroupMismatch: []string{"foo:.*"},
				}
				if err := s.Validate(); err != nil {
					t.Error(err)
					t.FailNow()
				}
				return context.WithValue(ctx, checks.SettingsKey(checks.RuleDependencyCheckName), &s)
			},
			prometheus: newSimpleProm,
			entries: parseWithStatePath(`groups:
- name: base
  rules:
  - record: foo:sum
    expr: sum(foo)
`, discovery.Noop, "base.yaml", "base.yaml"),
		},
		{
			description: "ignores recording rule with vector(1) - no VectorSelector",
			content: `groups:
- name: aggregations
  rules:
  - record: static:value
    expr: vector(1)
`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleDependencyCheck()
			},
			prometheus: newSimpleProm,
			entries: parseWithStatePath(`groups:
- name: base
  rules:
  - record: foo:sum
    expr: sum(foo)
`, discovery.Noop, "base.yaml", "base.yaml"),
		},
	}

	runTests(t, testCases)
}

func TestRuleDependencySettings(t *testing.T) {
	t.Run("valid regex", func(t *testing.T) {
		s := checks.RuleDependencySettings{
			IgnoreGroupMismatch: []string{"foo:.*", "bar:sum"},
		}
		err := s.Validate()
		if err != nil {
			t.Errorf("Validate() returned an error: %v", err)
		}
	})

	t.Run("invalid regex", func(t *testing.T) {
		s := checks.RuleDependencySettings{
			IgnoreGroupMismatch: []string{"foo:["},
		}
		err := s.Validate()
		if err == nil {
			t.Error("Validate() did not return an error for invalid regex")
		}
	})
}

func TestRuleDependencyCheck_Interface(t *testing.T) {
	c := checks.NewRuleDependencyCheck()
	if c.String() != checks.RuleDependencyCheckName {
		t.Errorf("String() = %q, want %q", c.String(), checks.RuleDependencyCheckName)
	}
	if c.Reporter() != checks.RuleDependencyCheckName {
		t.Errorf("Reporter() = %q, want %q", c.Reporter(), checks.RuleDependencyCheckName)
	}
	meta := c.Meta()
	if meta.Online {
		t.Error("Meta().Online = true, want false")
	}
	if meta.AlwaysEnabled {
		t.Error("Meta().AlwaysEnabled = true, want false")
	}
}
