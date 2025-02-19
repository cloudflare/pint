package checks_test

import (
	"context"
	"testing"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/parser"
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
			problems:    noProblems,
		},
		{
			description: "static match",
			content:     "- record: foo\n  expr: foo{job=\"bar\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
			problems:    noProblems,
		},
		{
			description: "valid regexp",
			content:     "- record: foo\n  expr: foo{job=~\"bar|foo\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
			problems:    noProblems,
		},
		{
			description: "valid regexp",
			content:     "- record: foo\n  expr: foo{job!~\"(.*)\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
			problems:    noProblems,
		},
		{
			description: "valid regexp",
			content:     "- record: foo\n  expr: foo{job=~\"a|b|c\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
			problems:    noProblems,
		},
		{
			description: "valid partial regexp",
			content:     "- record: foo\n  expr: foo{job=~\"prefix(a|b)\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
			problems:    noProblems,
		},
		{
			description: "unnecessary regexp",
			content:     "- record: foo\n  expr: foo{job=~\"bar\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: checks.RegexpCheckName,
						Text:     "Unnecessary regexp match on static string `job=~\"bar\"`, use `job=\"bar\"` instead.",
						Details:  checks.RegexpCheckDetails,
						Severity: checks.Bug,
					},
				}
			},
		},
		{
			description: "unnecessary negative regexp",
			content:     "- record: foo\n  expr: foo{job!~\"bar\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: checks.RegexpCheckName,
						Text:     "Unnecessary regexp match on static string `job!~\"bar\"`, use `job!=\"bar\"` instead.",
						Details:  checks.RegexpCheckDetails,
						Severity: checks.Bug,
					},
				}
			},
		},
		{
			description: "empty regexp",
			content:     "- record: foo\n  expr: foo{job=~\"\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: checks.RegexpCheckName,
						Text:     "Unnecessary regexp match on static string `job=~\"\"`, use `job=\"\"` instead.",
						Details:  checks.RegexpCheckDetails,
						Severity: checks.Bug,
					},
				}
			},
		},
		{
			description: "unnecessary regexp anchor",
			content:     "- record: foo\n  expr: foo{job=~\"^.+$\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: checks.RegexpCheckName,
						Text:     "Prometheus regexp matchers are automatically fully anchored so match for `job=~\"^.+$\"` will result in `job=~\"^^.+$$\"`, remove regexp anchors `^` and/or `$`.",
						Details:  checks.RegexpCheckDetails,
						Severity: checks.Bug,
					},
				}
			},
		},
		{
			description: "unnecessary regexp anchor",
			content:     "- record: foo\n  expr: foo{job=~\"(foo|^.+)$\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: checks.RegexpCheckName,
						Text:     "Prometheus regexp matchers are automatically fully anchored so match for `job=~\"(foo|^.+)$\"` will result in `job=~\"^(foo|^.+)$$\"`, remove regexp anchors `^` and/or `$`.",
						Details:  checks.RegexpCheckDetails,
						Severity: checks.Bug,
					},
				}
			},
		},
		{
			description: "duplicated unnecessary regexp",
			content:     "- record: foo\n  expr: foo{job=~\"bar\"} / foo{job=~\"bar\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: checks.RegexpCheckName,
						Text:     "Unnecessary regexp match on static string `job=~\"bar\"`, use `job=\"bar\"` instead.",
						Details:  checks.RegexpCheckDetails,
						Severity: checks.Bug,
					},
				}
			},
		},
		{
			description: "duplicated unnecessary regexp",
			content:     "- record: foo\n  expr: foo{job=~\"bar\"} / foo{job=~\"bar\", level=\"total\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: checks.RegexpCheckName,
						Text:     "Unnecessary regexp match on static string `job=~\"bar\"`, use `job=\"bar\"` instead.",
						Details:  checks.RegexpCheckDetails,
						Severity: checks.Bug,
					},
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: checks.RegexpCheckName,
						Text:     "Unnecessary regexp match on static string `job=~\"bar\"`, use `job=\"bar\"` instead.",
						Details:  checks.RegexpCheckDetails,
						Severity: checks.Bug,
					},
				}
			},
		},
		{
			description: "regexp with a modifier",
			content:     "- record: foo\n  expr: foo{job=~\"(?i)someone\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
			problems:    noProblems,
		},
		{
			description: "unnecessary wildcard regexp",
			content:     "- record: foo\n  expr: foo{job=~\".*\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: checks.RegexpCheckName,
						Text:     "Unnecessary wildcard regexp, simply use `foo` if you want to match on all `job` values.",
						Details:  checks.RegexpCheckDetails,
						Severity: checks.Bug,
					},
				}
			},
		},
		{
			description: "unnecessary wildcard regexp / many filters",
			content:     "- record: foo\n  expr: foo{job=~\".*\", cluster=~\".*\", instance=\"bob\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: checks.RegexpCheckName,
						Text:     "Unnecessary wildcard regexp, simply use `foo{instance=\"bob\"}` if you want to match on all `job` values.",
						Details:  checks.RegexpCheckDetails,
						Severity: checks.Bug,
					},
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: checks.RegexpCheckName,
						Text:     "Unnecessary wildcard regexp, simply use `foo{instance=\"bob\"}` if you want to match on all `cluster` values.",
						Details:  checks.RegexpCheckDetails,
						Severity: checks.Bug,
					},
				}
			},
		},
		{
			description: "unnecessary wildcard regexp / many filters / no name",
			content: `- record: foo
  expr: |
    {job=~".*", cluster=~".*", instance="bob"}
`,
			checker:    newRegexpCheck,
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  3,
						},
						Reporter: checks.RegexpCheckName,
						Text:     "Unnecessary wildcard regexp, simply use `{instance=\"bob\"}` if you want to match on all `job` values.",
						Details:  checks.RegexpCheckDetails,
						Severity: checks.Bug,
					},
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  3,
						},
						Reporter: checks.RegexpCheckName,
						Text:     "Unnecessary wildcard regexp, simply use `{instance=\"bob\"}` if you want to match on all `cluster` values.",
						Details:  checks.RegexpCheckDetails,
						Severity: checks.Bug,
					},
				}
			},
		},
		{
			description: "unnecessary wildcard regexp / many filters / regexp name",
			content: `- record: foo
  expr: |
    {job=~".*", __name__=~"foo|bar", cluster=~".*", instance="bob"}
`,
			checker:    newRegexpCheck,
			prometheus: noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  3,
						},
						Reporter: checks.RegexpCheckName,
						Text:     "Unnecessary wildcard regexp, simply use `{__name__=~\"foo|bar\", instance=\"bob\"}` if you want to match on all `job` values.",
						Details:  checks.RegexpCheckDetails,
						Severity: checks.Bug,
					},
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  3,
						},
						Reporter: checks.RegexpCheckName,
						Text:     "Unnecessary wildcard regexp, simply use `{__name__=~\"foo|bar\", instance=\"bob\"}` if you want to match on all `cluster` values.",
						Details:  checks.RegexpCheckDetails,
						Severity: checks.Bug,
					},
				}
			},
		},
		{
			description: "greedy wildcard regexp",
			content:     "- record: foo\n  expr: foo{job=~\".+\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
			problems:    noProblems,
		},
		{
			description: "unnecessary negative wildcard regexp",
			content:     "- record: foo\n  expr: foo{job!~\".*\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: checks.RegexpCheckName,
						Text:     "Unnecessary wildcard regexp, simply use `foo{job=\"\"}` if you want to match on all time series for `foo` without the `job` label.",
						Details:  checks.RegexpCheckDetails,
						Severity: checks.Bug,
					},
				}
			},
		},
		{
			description: "unnecessary negative greedy wildcard regexp",
			content:     "- record: foo\n  expr: foo{job!~\".+\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: checks.RegexpCheckName,
						Text:     "Unnecessary wildcard regexp, simply use `foo{job=\"\"}` if you want to match on all time series for `foo` without the `job` label.",
						Details:  checks.RegexpCheckDetails,
						Severity: checks.Bug,
					},
				}
			},
		},
		{
			description: "needed wildcard regexp",
			content:     "- record: foo\n  expr: count({__name__=~\".+\"})\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
			problems:    noProblems,
		},
		{
			description: "empty match",
			content:     "- record: foo\n  expr: foo{bar=\"\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
			problems:    noProblems,
		},
		{
			description: "empty negative match",
			content:     "- record: foo\n  expr: foo{bar!=\"\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
			problems:    noProblems,
		},
		{
			description: "positive wildcard regexp",
			content:     "- record: foo\n  expr: count({name=~\".+\", job=\"foo\"})\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
			problems:    noProblems,
		},
		{
			description: "smelly selector",
			content:     "- record: foo\n  expr: foo{job=~\"service_.*_prod\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: checks.RegexpCheckName,
						Text:     "`{job=~\"service_.*_prod\"}` looks like a smelly selector that tries to extract substrings from the value, please consider breaking down the value of this label into multiple smaller labels",
						Details:  checks.RegexpCheckDetails,
						Severity: checks.Warning,
					},
				}
			},
		},
		{
			description: "non-smelly selector",
			content:     "- record: foo\n  expr: rate(http_requests_total{job=\"foo\", code=~\"5..\"}[5m])\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
			problems:    noProblems,
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
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: checks.RegexpCheckName,
						Text:     "`{job=~\"service_.*_prod\"}` looks like a smelly selector that tries to extract substrings from the value, please consider breaking down the value of this label into multiple smaller labels",
						Details:  checks.RegexpCheckDetails,
						Severity: checks.Warning,
					},
				}
			},
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
			problems:   noProblems,
		},
		{
			description: "code=~5.*",
			content:     "- record: foo\n  expr: foo{code=~\"5.*\"}\n",
			checker:     newRegexpCheck,
			prometheus:  noProm,
			problems:    noProblems,
		},
	}
	runTests(t, testCases)
}
