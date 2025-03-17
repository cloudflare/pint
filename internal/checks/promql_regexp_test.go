package checks_test

import (
	"context"
	"testing"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/promapi"
)

func newRegexpCheck(_ *promapi.FailoverGroup) checks.RuleChecker {
	return checks.NewRegexpCheck()
}

func TestRegexpCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "ignores rules with syntax errors",
			content:     "- alert: foo\n  expr: sum(foo) without(\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
		},
		{
			description: "static match",
			content:     "- record: foo\n  expr: foo{job=\"bar\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
		},
		{
			description: "valid regexp",
			content:     "- record: foo\n  expr: foo{job=~\"bar|foo\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
		},
		{
			description: "valid regexp",
			content:     "- record: foo\n  expr: foo{job!~\"(.*)\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
		},
		{
			description: "valid regexp",
			content:     "- record: foo\n  expr: foo{job=~\"a|b|c\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
		},
		{
			description: "valid partial regexp",
			content:     "- record: foo\n  expr: foo{job=~\"prefix(a|b)\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
		},
		{
			description: "unnecessary regexp",
			content:     "- record: foo\n  expr: foo{job=~\"bar\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "unnecessary negative regexp",
			content:     "- record: foo\n  expr: foo{job!~\"bar\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "empty regexp",
			content:     "- record: foo\n  expr: foo{job=~\"\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "unnecessary regexp anchor",
			content:     "- record: foo\n  expr: foo{job=~\"^.+$\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "unnecessary regexp anchor",
			content:     "- record: foo\n  expr: foo{job=~\"(foo|^.+)$\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "duplicated unnecessary regexp",
			content:     "- record: foo\n  expr: foo{job=~\"bar\"} / foo{job=~\"bar\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "duplicated unnecessary regexp",
			content:     "- record: foo\n  expr: foo{job=~\"bar\"} / foo{job=~\"bar\", level=\"total\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "regexp with a modifier",
			content:     "- record: foo\n  expr: foo{job=~\"(?i)someone\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
		},
		{
			description: "unnecessary wildcard regexp",
			content:     "- record: foo\n  expr: foo{job=~\".*\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "unnecessary wildcard regexp / many filters",
			content:     "- record: foo\n  expr: foo{job=~\".*\", cluster=~\".*\", instance=\"bob\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "unnecessary wildcard regexp / many filters / no name",
			content: `- record: foo
  expr: |
    {job=~".*", cluster=~".*", instance="bob"}
`,
			checker:    newRegexpCheck,
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "unnecessary wildcard regexp / many filters / regexp name",
			content: `- record: foo
  expr: |
    {job=~".*", __name__=~"foo|bar", cluster=~".*", instance="bob"}
`,
			checker:    newRegexpCheck,
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "greedy wildcard regexp",
			content:     "- record: foo\n  expr: foo{job=~\".+\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
		},
		{
			description: "unnecessary negative wildcard regexp",
			content:     "- record: foo\n  expr: foo{job!~\".*\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "unnecessary negative greedy wildcard regexp",
			content:     "- record: foo\n  expr: foo{job!~\".+\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "needed wildcard regexp",
			content:     "- record: foo\n  expr: count({__name__=~\".+\"})\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
		},
		{
			description: "empty match",
			content:     "- record: foo\n  expr: foo{bar=\"\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
		},
		{
			description: "empty negative match",
			content:     "- record: foo\n  expr: foo{bar!=\"\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
		},
		{
			description: "positive wildcard regexp",
			content:     "- record: foo\n  expr: count({name=~\".+\", job=\"foo\"})\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
		},
		{
			description: "smelly selector",
			content:     "- record: foo\n  expr: foo{job=~\"service_.*_prod\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "non-smelly selector",
			content:     "- record: foo\n  expr: rate(http_requests_total{job=\"foo\", code=~\"5..\"}[5m])\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
		},
		{
			description: "smelly selector / enabled",
			content:     "- record: foo\n  expr: foo{job=~\"service_.*_prod\"}\n",
			checker:     newRegexpCheck,
			ctx: func(ctx context.Context, _ string) context.Context {
				yes := true
				s := checks.PromqlRegexpSettings{
					Smelly: &yes,
				}
				if err := s.Validate(); err != nil {
					t.Error(err)
					t.FailNow()
				}
				return context.WithValue(ctx, checks.SettingsKey(checks.RegexpCheckName), &s)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "smelly selector / disabled",
			content:     "- record: foo\n  expr: foo{job=~\"service_.*_prod\"}\n",
			checker:     newRegexpCheck,
			ctx: func(ctx context.Context, _ string) context.Context {
				no := false
				s := checks.PromqlRegexpSettings{
					Smelly: &no,
				}
				if err := s.Validate(); err != nil {
					t.Error(err)
					t.FailNow()
				}
				return context.WithValue(ctx, checks.SettingsKey(checks.RegexpCheckName), &s)
			},
			prometheus: noProm,
		},
		{
			description: "code=~5.*",
			content:     "- record: foo\n  expr: foo{code=~\"5.*\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
		},
		{
			description: "smelly selector / multiple",
			content: `
- record: foo
  expr: |
    sum by (instance, type) (rate(requests_total{env=~"production.*", status="failed"}[5m])) / (1 + sum by (instance, type) (rate(request_total{env=~"production.*"}[5m]))) > 0.1
`,
			checker:    newRegexpCheck,
			prometheus: noProm,
			problems:   true,
		},
	}
	runTests(t, testCases)
}
