package checks_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/cloudflare/pint/internal/checks"
)

func newRateCheck(uri string) checks.RuleChecker {
	return checks.NewRateCheck(simpleProm("prom", uri, time.Second, true))
}

func durationMustText(name, uri, fun, multi, using string) string {
	return fmt.Sprintf(`duration for %s() must be at least %s x scrape_interval, prometheus %q at %s is using %s scrape_interval`, fun, multi, name, uri, using)
}

func durationRecommenedText(name, uri, fun, multi, using string) string {
	return fmt.Sprintf("duration for %s() is recommended to be at least %s x scrape_interval, prometheus %q at %s is using %s scrape_interval", fun, multi, name, uri, using)
}

func TestRateCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "ignores rules with syntax errors",
			content:     "- record: foo\n  expr: sum(foo) without(\n",
			checker:     newRateCheck,
			problems:    noProblems,
		},
		{
			description: "rate < 2x scrape_interval",
			content:     "- record: foo\n  expr: rate(foo[1m])\n",
			checker:     newRateCheck,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "rate(foo[1m])",
						Lines:    []int{2},
						Reporter: "promql/rate",
						Text:     durationMustText("prom", uri, "rate", "2", "1m"),
						Severity: checks.Bug,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
			},
		},
		{
			description: "rate < 4x scrape_interval",
			content:     "- record: foo\n  expr: rate(foo[3m])\n",
			checker:     newRateCheck,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "rate(foo[3m])",
						Lines:    []int{2},
						Reporter: "promql/rate",
						Text:     durationRecommenedText("prom", uri, "rate", "4", "1m"),
						Severity: checks.Warning,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
			},
		},
		{
			description: "rate == 4x scrape interval",
			content:     "- record: foo\n  expr: rate(foo[2m])\n",
			checker:     newRateCheck,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 30s\n"},
				},
			},
		},
		{
			description: "irate < 2x scrape_interval",
			content:     "- record: foo\n  expr: irate(foo[1m])\n",
			checker:     newRateCheck,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "irate(foo[1m])",
						Lines:    []int{2},
						Reporter: "promql/rate",
						Text:     durationMustText("prom", uri, "irate", "2", "1m"),
						Severity: checks.Bug,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
			},
		},
		{
			description: "irate < 3x scrape_interval",
			content:     "- record: foo\n  expr: irate(foo[2m])\n",
			checker:     newRateCheck,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "irate(foo[2m])",
						Lines:    []int{2},
						Reporter: "promql/rate",
						Text:     durationRecommenedText("prom", uri, "irate", "3", "1m"),
						Severity: checks.Warning,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
			},
		},
		{
			description: "irate{__name__} > 3x scrape_interval",
			content: `
- record: foo
  expr: irate({__name__="foo"}[5m])
`,
			checker:  newRateCheck,
			problems: noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
			},
		},
		{
			description: "irate{__name__=~} > 3x scrape_interval",
			content: `
- record: foo
  expr: irate({__name__=~"(foo|bar)_total"}[5m])
`,
			checker:  newRateCheck,
			problems: noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
			},
		},
		{
			description: "irate{__name__} < 3x scrape_interval",
			content: `
- record: foo
  expr: irate({__name__="foo"}[2m])
`,
			checker: newRateCheck,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: `irate({__name__="foo"}[2m])`,
						Lines:    []int{3},
						Reporter: "promql/rate",
						Text:     durationRecommenedText("prom", uri, "irate", "3", "1m"),
						Severity: checks.Warning,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
			},
		},
		{
			description: "irate{__name__=~} < 3x scrape_interval",
			content: `
- record: foo
  expr: irate({__name__=~"(foo|bar)_total"}[2m])
`,
			checker: newRateCheck,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: `irate({__name__=~"(foo|bar)_total"}[2m])`,
						Lines:    []int{3},
						Reporter: "promql/rate",
						Text:     durationRecommenedText("prom", uri, "irate", "3", "1m"),
						Severity: checks.Warning,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
			},
		},
		{
			description: "irate == 3x scrape interval",
			content:     "- record: foo\n  expr: irate(foo[3m])\n",
			checker:     newRateCheck,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
			},
		},
		{
			description: "valid range selector",
			content:     "- record: foo\n  expr: foo[1m]\n",
			checker:     newRateCheck,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
			},
		},
		{
			description: "nested invalid rate",
			content:     "- record: foo\n  expr: sum(rate(foo[3m])) / sum(rate(bar[1m]))\n",
			checker:     newRateCheck,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "rate(foo[3m])",
						Lines:    []int{2},
						Reporter: "promql/rate",
						Text:     durationRecommenedText("prom", uri, "rate", "4", "1m"),
						Severity: checks.Warning,
					},
					{
						Fragment: "rate(bar[1m])",
						Lines:    []int{2},
						Reporter: "promql/rate",
						Text:     durationMustText("prom", uri, "rate", "2", "1m"),
						Severity: checks.Bug,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
			},
		},
		{
			description: "500 error from Prometheus API",
			content:     "- record: foo\n  expr: rate(foo[5m])\n",
			checker:     newRateCheck,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "rate(foo[5m])",
						Lines:    []int{2},
						Reporter: "promql/rate",
						Text:     checkErrorUnableToRun(checks.RateCheckName, "prom", uri, "failed to query Prometheus config: server_error: server error: 500"),
						Severity: checks.Bug,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  respondWithInternalError(),
				},
			},
		},
		{
			description: "invalid status",
			content:     "- record: foo\n  expr: rate(foo[5m])\n",
			checker:     newRateCheck,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "rate(foo[5m])",
						Lines:    []int{2},
						Reporter: "promql/rate",
						Text:     checkErrorBadData("prom", uri, "failed to query Prometheus config: bad_data: bad input data"),
						Severity: checks.Bug,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  respondWithBadData(),
				},
			},
		},
		{
			description: "invalid YAML",
			content:     "- record: foo\n  expr: rate(foo[5m])\n",
			checker:     newRateCheck,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "rate(foo[5m])",
						Lines:    []int{2},
						Reporter: "promql/rate",
						Text: checkErrorUnableToRun(checks.RateCheckName, "prom", uri,
							fmt.Sprintf("failed to decode config data in %s response: yaml: line 2: could not find expected ':'", uri)),
						Severity: checks.Bug,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:::\nglobal:{}{}{}\n"},
				},
			},
		},
		{
			description: "connection refused",
			content:     "- record: foo\n  expr: rate(foo[5m])\n",
			checker: func(_ string) checks.RuleChecker {
				return checks.NewRateCheck(simpleProm("prom", "http://127.0.0.1:1111", time.Second, true))
			},
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "rate(foo[5m])",
						Lines:    []int{2},
						Reporter: "promql/rate",
						Text:     checkErrorUnableToRun(checks.RateCheckName, "prom", "http://127.0.0.1:1111", `failed to query Prometheus config: Get "http://127.0.0.1:1111/api/v1/status/config": dial tcp 127.0.0.1:1111: connect: connection refused`),
						Severity: checks.Bug,
					},
				}
			},
		},
		{
			description: "irate == 3 x default 1m",
			content:     "- record: foo\n  expr: irate(foo[3m])\n",
			checker:     newRateCheck,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global: {}\n"},
				},
			},
		},
		{
			description: "irate < 3 x default 1m",
			content:     "- record: foo\n  expr: irate(foo[2m])\n",
			checker:     newRateCheck,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "irate(foo[2m])",
						Lines:    []int{2},
						Reporter: "promql/rate",
						Text:     durationRecommenedText("prom", uri, "irate", "3", "1m"),
						Severity: checks.Warning,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global: {}\n"},
				},
			},
		},
	}
	runTests(t, testCases)
}
