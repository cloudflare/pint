package checks_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
)

func newRateCheck(prom *promapi.FailoverGroup) checks.RuleChecker {
	return checks.NewRateCheck(prom)
}

func durationMustText(name, uri, fun, multi, using string) string {
	return fmt.Sprintf("Duration for `%s()` must be at least %s x scrape_interval, `%s` Prometheus server at %s is using `%s` scrape_interval.", fun, multi, name, uri, using)
}

func notCounterText(name, uri, fun, metric, kind string) string {
	return fmt.Sprintf("`%s()` should only be used with counters but `%s` is a %s according to metrics metadata from `%s` Prometheus server at %s.", fun, metric, kind, name, uri)
}

func rateSumText(rateName, sumExpr string) string {
	return fmt.Sprintf("`rate(sum(counter))` chain detected, `rate(%s)` is called here on results of `%s`.", rateName, sumExpr)
}

func rateSumDetails() string {
	return "You can only calculate `rate()` directly from a counter metric. Calling `rate()` on `sum()` results will return bogus results because `sum()` will hide information on when each counter resets. You must first calculate `rate()` before calling any aggregation function. Always `sum(rate(counter))`, never `rate(sum(counter))`"
}

func TestRateCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "ignores rules with syntax errors",
			content:     "- record: foo\n  expr: sum(foo) without(\n",
			checker:     newRateCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
		},
		{
			description: "rate < 2x scrape_interval",
			content:     "- record: foo\n  expr: rate(foo[1m])\n",
			checker:     newRateCheck,
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "promql/rate",
						Text:     durationMustText("prom", uri, "rate", "2", "1m"),
						Details:  checks.RateCheckDetails,
						Severity: checks.Bug,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
				{
					conds: []requestCondition{requireMetadataPath},
					resp: metadataResponse{metadata: map[string][]v1.Metadata{
						"foo": {{Type: "counter"}},
					}},
				},
			},
		},
		{
			description: "rate < 4x scrape_interval",
			content:     "- record: foo\n  expr: rate(foo[3m])\n",
			checker:     newRateCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
				{
					conds: []requestCondition{requireMetadataPath},
					resp: metadataResponse{metadata: map[string][]v1.Metadata{
						"foo": {{Type: "counter"}},
					}},
				},
			},
		},
		{
			description: "rate == 4x scrape interval",
			content:     "- record: foo\n  expr: rate(foo[2m])\n",
			checker:     newRateCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 30s\n"},
				},
				{
					conds: []requestCondition{requireMetadataPath},
					resp: metadataResponse{metadata: map[string][]v1.Metadata{
						"foo": {{Type: "counter"}},
					}},
				},
			},
		},
		{
			description: "irate < 2x scrape_interval",
			content:     "- record: foo\n  expr: irate(foo[1m])\n",
			checker:     newRateCheck,
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "promql/rate",
						Text:     durationMustText("prom", uri, "irate", "2", "1m"),
						Details:  checks.RateCheckDetails,
						Severity: checks.Bug,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
				{
					conds: []requestCondition{requireMetadataPath},
					resp: metadataResponse{metadata: map[string][]v1.Metadata{
						"foo": {{Type: "counter"}},
					}},
				},
			},
		},
		{
			description: "deriv < 2x scrape_interval",
			content:     "- record: foo\n  expr: deriv(foo[1m])\n",
			checker:     newRateCheck,
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "promql/rate",
						Text:     durationMustText("prom", uri, "deriv", "2", "1m"),
						Details:  checks.RateCheckDetails,
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
			description: "deriv == 2x scrape_interval",
			content:     "- record: foo\n  expr: deriv(foo[2m])\n",
			checker:     newRateCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
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
			prometheus:  newSimpleProm,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
				{
					conds: []requestCondition{requireMetadataPath},
					resp: metadataResponse{metadata: map[string][]v1.Metadata{
						"foo": {{Type: "counter"}},
					}},
				},
			},
		},
		{
			description: "irate{__name__} > 3x scrape_interval",
			content: `
- record: foo
  expr: irate({__name__="foo"}[5m])
`,
			checker:    newRateCheck,
			prometheus: newSimpleProm,
			problems:   noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
				{
					conds: []requestCondition{requireMetadataPath},
					resp: metadataResponse{metadata: map[string][]v1.Metadata{
						"foo": {{Type: "counter"}},
					}},
				},
			},
		},
		{
			description: "irate{__name__=~} > 3x scrape_interval",
			content: `
- record: foo
  expr: irate({__name__=~"(foo|bar)_total"}[5m])
`,
			checker:    newRateCheck,
			prometheus: newSimpleProm,
			problems:   noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
				{
					conds: []requestCondition{requireMetadataPath},
					resp: metadataResponse{metadata: map[string][]v1.Metadata{
						"foo": {{Type: "counter"}},
					}},
				},
			},
		},
		{
			description: "irate{__name__} < 3x scrape_interval",
			content: `
- record: foo
  expr: irate({__name__="foo"}[2m])
`,
			checker:    newRateCheck,
			prometheus: newSimpleProm,
			problems:   noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
				{
					conds: []requestCondition{requireMetadataPath},
					resp: metadataResponse{metadata: map[string][]v1.Metadata{
						"foo": {{Type: "counter"}},
					}},
				},
			},
		},
		{
			description: "irate{__name__=~} < 3x scrape_interval",
			content: `
- record: foo
  expr: irate({__name__=~"(foo|bar)_total"}[2m])
`,
			checker:    newRateCheck,
			prometheus: newSimpleProm,
			problems:   noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
				{
					conds: []requestCondition{requireMetadataPath},
					resp: metadataResponse{metadata: map[string][]v1.Metadata{
						"foo": {{Type: "counter"}},
					}},
				},
			},
		},
		{
			description: "irate == 3x scrape interval",
			content:     "- record: foo\n  expr: irate(foo[3m])\n",
			checker:     newRateCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
				{
					conds: []requestCondition{requireMetadataPath},
					resp: metadataResponse{metadata: map[string][]v1.Metadata{
						"foo": {{Type: "counter"}},
					}},
				},
			},
		},
		{
			description: "valid range selector",
			content:     "- record: foo\n  expr: foo[1m]\n",
			checker:     newRateCheck,
			prometheus:  newSimpleProm,
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
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "promql/rate",
						Text:     durationMustText("prom", uri, "rate", "2", "1m"),
						Details:  checks.RateCheckDetails,
						Severity: checks.Bug,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
				{
					conds: []requestCondition{requireMetadataPath},
					resp: metadataResponse{metadata: map[string][]v1.Metadata{
						"foo": {{Type: "counter"}},
					}},
				},
			},
		},
		{
			description: "500 error from Prometheus API",
			content:     "- record: foo\n  expr: rate(foo[5m])\n",
			checker:     newRateCheck,
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "promql/rate",
						Text:     checkErrorUnableToRun(checks.RateCheckName, "prom", uri, "server_error: internal error"),
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
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "promql/rate",
						Text:     checkErrorBadData("prom", uri, "bad_data: bad input data"),
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
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
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
			checker:     newRateCheck,
			prometheus: func(_ string) *promapi.FailoverGroup {
				return simpleProm("prom", "http://127.0.0.1:1111", time.Second, true)
			},
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "promql/rate",
						Text:     checkErrorUnableToRun(checks.RateCheckName, "prom", "http://127.0.0.1:1111", "connection refused"),
						Severity: checks.Bug,
					},
				}
			},
		},
		{
			description: "irate == 3 x default 1m",
			content:     "- record: foo\n  expr: irate(foo[3m])\n",
			checker:     newRateCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global: {}\n"},
				},
				{
					conds: []requestCondition{requireMetadataPath},
					resp: metadataResponse{metadata: map[string][]v1.Metadata{
						"foo": {{Type: "counter"}},
					}},
				},
			},
		},
		{
			description: "metadata error",
			content:     "- record: foo\n  expr: rate(foo{job=\"xxx\"}[1m])\n",
			checker:     newRateCheck,
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "promql/rate",
						Text:     durationMustText("prom", uri, "rate", "2", "1m"),
						Details:  checks.RateCheckDetails,
						Severity: checks.Bug,
					},
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "promql/rate",
						Text:     checkErrorUnableToRun(checks.RateCheckName, "prom", uri, "server_error: internal error"),
						Severity: checks.Bug,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
				{
					conds: []requestCondition{requireMetadataPath},
					resp:  respondWithInternalError(),
				},
			},
		},
		{
			description: "empty metadata response",
			content:     "- record: foo\n  expr: rate(foo{job=\"xxx\"}[5m])\n",
			checker:     newRateCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
				{
					conds: []requestCondition{requireMetadataPath},
					resp:  metadataResponse{metadata: map[string][]v1.Metadata{}},
				},
			},
		},
		{
			description: "metadata unsupported",
			content:     "- record: foo\n  expr: rate(foo{job=\"xxx\"}[1m])\n",
			checker:     newRateCheck,
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "promql/rate",
						Text:     durationMustText("prom", uri, "rate", "2", "1m"),
						Details:  checks.RateCheckDetails,
						Severity: checks.Bug,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
				{
					conds: []requestCondition{requireMetadataPath},
					resp:  httpResponse{code: http.StatusNotFound, body: "Not Found"},
				},
			},
		},
		{
			description: "rate(gauge) < 2x scrape interval",
			content:     "- record: foo\n  expr: rate(foo{job=\"xxx\"}[1m])\n",
			checker:     newRateCheck,
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "promql/rate",
						Text:     durationMustText("prom", uri, "rate", "2", "1m"),
						Details:  checks.RateCheckDetails,
						Severity: checks.Bug,
					},
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "promql/rate",
						Text:     notCounterText("prom", uri, "rate", "foo", "gauge"),
						Details:  checks.RateCheckDetails,
						Severity: checks.Bug,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
				{
					conds: []requestCondition{requireMetadataPath},
					resp: metadataResponse{metadata: map[string][]v1.Metadata{
						"foo": {{Type: "gauge"}},
					}},
				},
			},
		},
		{
			description: "rate(counter)  / rate(gauge)",
			content:     "- record: foo\n  expr: rate(foo_c[2m]) / rate(bar_g[2m])\n",
			checker:     newRateCheck,
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "promql/rate",
						Text:     notCounterText("prom", uri, "rate", "bar_g", "gauge"),
						Details:  checks.RateCheckDetails,
						Severity: checks.Bug,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
				{
					conds: []requestCondition{
						requireMetadataPath,
						formCond{"metric", "foo_c"},
					},
					resp: metadataResponse{metadata: map[string][]v1.Metadata{
						"foo_c": {{Type: "counter"}},
					}},
				},
				{
					conds: []requestCondition{
						requireMetadataPath,
						formCond{"metric", "bar_g"},
					},
					resp: metadataResponse{metadata: map[string][]v1.Metadata{
						"bar_g": {{Type: "gauge"}},
					}},
				},
			},
		},
		{
			description: "rate(unknown)",
			content:     "- record: foo\n  expr: rate(foo[2m])\n",
			checker:     newRateCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
				{
					conds: []requestCondition{requireMetadataPath},
					resp: metadataResponse{metadata: map[string][]v1.Metadata{
						"foo": {{Type: "unknown"}},
					}},
				},
			},
		},
		{
			description: "rate(foo) / rate(foo) / sum(rate(foo))",
			content:     "- record: foo\n  expr: rate(foo[2m]) / rate(foo[2m]) / sum(rate(foo[2m]))\n",
			checker:     newRateCheck,
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "promql/rate",
						Text:     notCounterText("prom", uri, "rate", "foo", "gauge"),
						Details:  checks.RateCheckDetails,
						Severity: checks.Bug,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
				{
					conds: []requestCondition{
						requireMetadataPath,
						formCond{"metric", "foo"},
					},
					resp: metadataResponse{metadata: map[string][]v1.Metadata{
						"foo": {{Type: "gauge"}},
					}},
				},
			},
		},
		{
			description: "rate_over_sum",
			content:     "- alert: my alert\n  expr: rate(my:sum[5m])\n",
			entries:     mustParseContent("- record: my:sum\n  expr: sum(foo)\n"),
			checker:     newRateCheck,
			prometheus:  newSimpleProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "promql/rate",
						Text:     rateSumText("my:sum[5m]", "sum(foo)"),
						Details:  rateSumDetails(),
						Severity: checks.Bug,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
				{
					conds: []requestCondition{
						requireMetadataPath,
						formCond{"metric", "foo"},
					},
					resp: metadataResponse{metadata: map[string][]v1.Metadata{
						"foo": {{Type: "counter"}},
					}},
				},
				{
					conds: []requestCondition{
						requireMetadataPath,
						formCond{"metric", "my:sum"},
					},
					resp: metadataResponse{metadata: map[string][]v1.Metadata{}},
				},
			},
		},
		{
			description: "rate_over_sum_error",
			content:     "- alert: my alert\n  expr: rate(my:sum[5m])\n",
			entries:     mustParseContent("- record: my:sum\n  expr: sum(foo)\n"),
			checker:     newRateCheck,
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "promql/rate",
						Text:     checkErrorUnableToRun(checks.RateCheckName, "prom", uri, "server_error: internal error"),
						Severity: checks.Bug,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
				{
					conds: []requestCondition{
						requireMetadataPath,
						formCond{"metric", "foo"},
					},
					resp: respondWithInternalError(),
				},
				{
					conds: []requestCondition{
						requireMetadataPath,
						formCond{"metric", "my:sum"},
					},
					resp: metadataResponse{metadata: map[string][]v1.Metadata{}},
				},
			},
		},
		{
			description: "rate_over_sum_on_gauge",
			content:     "- alert: my alert\n  expr: rate(my:sum[5m])\n",
			entries:     mustParseContent("- record: my:sum\n  expr: sum(foo)\n"),
			checker:     newRateCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
				{
					conds: []requestCondition{
						requireMetadataPath,
						formCond{"metric", "foo"},
					},
					resp: metadataResponse{metadata: map[string][]v1.Metadata{
						"foo": {{Type: "gauge"}},
					}},
				},
				{
					conds: []requestCondition{
						requireMetadataPath,
						formCond{"metric", "my:sum"},
					},
					resp: metadataResponse{metadata: map[string][]v1.Metadata{}},
				},
			},
		},
		{
			description: "sum_over_rate",
			content:     "- alert: my alert\n  expr: sum(my:rate:5m)\n",
			entries:     mustParseContent("- record: my:rate:5m\n  expr: rate(foo[5m])\n"),
			checker:     newRateCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
			},
		},
		{
			description: "sum_over_rate / ignore entry with PathError",
			content:     "- alert: my alert\n  expr: rate(my:sum[5m])\n",
			entries:     []discovery.Entry{{PathError: errors.New("mock error")}},
			checker:     newRateCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
				{
					conds: []requestCondition{
						requireMetadataPath,
						formCond{"metric", "my:sum"},
					},
					resp: metadataResponse{metadata: map[string][]v1.Metadata{}},
				},
			},
		},
		{
			description: "sum_over_rate / ignore entry with rule error",
			content:     "- alert: my alert\n  expr: rate(my:sum[5m])\n",
			entries: []discovery.Entry{
				{
					Rule: parser.Rule{
						Error: parser.ParseError{
							Err: errors.New("mock error"),
						},
					},
				},
			},
			checker:    newRateCheck,
			prometheus: newSimpleProm,
			problems:   noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
				{
					conds: []requestCondition{
						requireMetadataPath,
						formCond{"metric", "my:sum"},
					},
					resp: metadataResponse{metadata: map[string][]v1.Metadata{}},
				},
			},
		},
		{
			description: "sum(rate(sum)) / sum(rate(sum))",
			content: `
- alert: Plexi_Worker_High_Signing_Latency
  expr: |
    sum(
      rate(global:response_time_sum{namespace!~"test[.].+"}[15m])
    ) by (environment, namespace)
    /
    sum(
      rate(global:response_time_count{namespace!~"test[.].+"}[15m])
    ) by (environment, namespace)
    > 3000
`,
			checker:    newRateCheck,
			prometheus: newSimpleProm,
			entries: mustParseContent(`
- record: global:response_time_sum
  expr: sum(response_time_sum:rate2m)
- record: response_time_sum:rate2m
  expr: rate(response_time_sum[2m])
`),
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 4,
							Last:  11,
						},
						Reporter: "promql/rate",
						Text:     rateSumText(`global:response_time_sum{namespace!~"test[.].+"}[15m]`, "sum(response_time_sum:rate2m)"),
						Details:  rateSumDetails(),
						Severity: checks.Warning,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 53s\n"},
				},
				{
					conds: []requestCondition{
						requireMetadataPath,
					},
					resp: metadataResponse{metadata: map[string][]v1.Metadata{}},
				},
			},
		},
		{
			description: "config 404",
			content:     "- alert: my alert\n  expr: rate(my:sum[5m])\n",
			entries:     mustParseContent("- record: my:sum\n  expr: sum(foo)\n"),
			checker:     newRateCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  httpResponse{code: http.StatusNotFound, body: "Not Found"},
				},
			},
		},
		{
			description: "metadata 404",
			content: `
- alert: Plexi_Worker_High_Signing_Latency
  expr: |
    sum(
      rate(global:response_time_sum{namespace!~"test[.].+"}[15m])
    ) by (environment, namespace)
    /
    sum(
      rate(global:response_time_count{namespace!~"test[.].+"}[15m])
    ) by (environment, namespace)
    > 3000
`,
			checker:    newRateCheck,
			prometheus: newSimpleProm,
			entries: mustParseContent(`
- record: global:response_time_sum
  expr: sum(response_time_sum:rate2m)
- record: response_time_sum:rate2m
  expr: rate(response_time_sum[2m])
`),
			problems: noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 53s\n"},
				},
				{
					conds: []requestCondition{
						requireMetadataPath,
						formCond{"metric", "global:response_time_sum"},
					},
					resp: metadataResponse{metadata: map[string][]v1.Metadata{}},
				},
				{
					conds: []requestCondition{
						requireMetadataPath,
						formCond{"metric", "response_time_sum:rate2m"},
					},
					resp: httpResponse{code: http.StatusNotFound, body: "Not Found"},
				},
			},
		},
		{
			description: "rate over non aggregate",
			content:     "- alert: my alert\n  expr: rate(my:foo[5m])\n",
			entries:     mustParseContent("- record: my:foo\n  expr: foo / foo\n"),
			checker:     newRateCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
				{
					conds: []requestCondition{
						requireMetadataPath,
						formCond{"metric", "my:foo"},
					},
					resp: metadataResponse{metadata: map[string][]v1.Metadata{}},
				},
			},
		},
	}
	runTests(t, testCases)
}
