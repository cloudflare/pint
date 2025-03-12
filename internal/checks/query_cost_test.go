package checks_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/common/model"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/promapi"
)

func costText(name, uri string, count int) string {
	return fmt.Sprintf("`%s` Prometheus server at %s returned %d result(s)", name, uri, count)
}

func memUsageText(b string) string {
	return fmt.Sprintf(" with %s estimated memory usage", b)
}

func maxSeriesText(m int) string {
	return fmt.Sprintf(", maximum allowed series is %d", m)
}

func evalDurText(name, uri, dur, limit string) string {
	return fmt.Sprintf("`%s` Prometheus server at %s took %s when executing this query, which is more than the configured limit of %s.", name, uri, dur, limit)
}

func totalSamplesText(name, uri string, total, limit int) string {
	return fmt.Sprintf("`%s` Prometheus server at %s queried %d samples in total when executing this query, which is more than the configured limit of %d.", name, uri, total, limit)
}

func peakSamplesText(name, uri string, total, limit int) string {
	return fmt.Sprintf("`%s` Prometheus server at %s queried %d peak samples when executing this query, which is more than the configured limit of %d.", name, uri, total, limit)
}

func TestCostCheck(t *testing.T) {
	content := "- record: foo\n  expr: sum(foo)\n"

	testCases := []checkTest{
		{
			description: "ignores rules with syntax errors",
			content:     "- record: foo\n  expr: sum(foo) without(\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewCostCheck(prom, 0, 0, 0, 0, "", checks.Bug)
			},
			prometheus: newSimpleProm,
			problems:   noProblems,
		},
		{
			description: "empty response",
			content:     content,
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewCostCheck(prom, 0, 0, 0, 0, "", checks.Bug)
			},
			prometheus: newSimpleProm,
			problems:   noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: `count(sum(foo))`},
					},
					resp: respondWithEmptyVector(),
				},
			},
		},
		{
			description: "response timeout",
			content:     content,
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewCostCheck(prom, 0, 0, 0, 0, "", checks.Bug)
			},
			prometheus: func(uri string) *promapi.FailoverGroup {
				return simpleProm("prom", uri, time.Millisecond*50, true)
			},
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Reporter: "query/cost",
						Summary:  "unable to run checks",
						Severity: checks.Bug,
						Diagnostics: []diags.Diagnostic{
							{
								Message: checkErrorUnableToRun("prom", uri, "connection timeout"),
							},
						},
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: `count(sum(foo))`},
					},
					resp: sleepResponse{sleep: time.Millisecond * 1500},
				},
			},
		},
		{
			description: "bad request",
			content:     content,
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewCostCheck(prom, 0, 0, 0, 0, "", checks.Bug)
			},
			prometheus: newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Reporter: "query/cost",
						Summary:  "unable to run checks",
						Severity: checks.Bug,
						Diagnostics: []diags.Diagnostic{
							{
								Message: checkErrorBadData("prom", uri, "bad_data: bad input data"),
							},
						},
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: `count(sum(foo))`},
					},
					resp: respondWithBadData(),
				},
			},
		},
		{
			description: "connection refused",
			content:     content,
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewCostCheck(prom, 0, 0, 0, 0, "", checks.Bug)
			},
			prometheus: func(_ string) *promapi.FailoverGroup {
				return simpleProm("prom", "http://127.0.0.1:1111", time.Second*5, false)
			},
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Reporter: "query/cost",
						Summary:  "unable to run checks",
						Severity: checks.Warning,
						Diagnostics: []diags.Diagnostic{
							{
								Message: checkErrorUnableToRun("prom", "http://127.0.0.1:1111", "connection refused"),
							},
						},
					},
				}
			},
		},
		{
			description: "1 result",
			content:     content,
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewCostCheck(prom, 0, 0, 0, 0, "", checks.Bug)
			},
			prometheus: newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Reporter: "query/cost",
						Summary:  "query cost estimate",
						Severity: checks.Information,
						Diagnostics: []diags.Diagnostic{
							{
								Message: costText("prom", uri, 1) + memUsageText("4.0KiB") + ".",
							},
						},
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: `count(sum(foo))`},
					},
					resp: respondWithSingleInstantVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: checks.BytesPerSampleQuery},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSampleWithValue(map[string]string{}, 4096),
						},
					},
				},
			},
		},
		{
			description: "7 results",
			content:     content,
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewCostCheck(prom, 0, 0, 0, 0, "", checks.Bug)
			},
			prometheus: newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Reporter: "query/cost",
						Summary:  "query cost estimate",
						Severity: checks.Information,
						Diagnostics: []diags.Diagnostic{
							{
								Message: costText("prom", uri, 7) + memUsageText("707B") + ".",
							},
						},
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: `count(sum(foo))`},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{}),
							generateSample(map[string]string{}),
							generateSample(map[string]string{}),
							generateSample(map[string]string{}),
							generateSample(map[string]string{}),
							generateSample(map[string]string{}),
							generateSample(map[string]string{}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: checks.BytesPerSampleQuery},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSampleWithValue(map[string]string{}, 101),
						},
					},
				},
			},
		},
		{
			description: "7 result with MB",
			content:     content,
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewCostCheck(prom, 0, 0, 0, 0, "", checks.Bug)
			},
			prometheus: newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Reporter: "query/cost",
						Summary:  "query cost estimate",
						Severity: checks.Information,
						Diagnostics: []diags.Diagnostic{
							{
								Message: costText("prom", uri, 7) + memUsageText("7.0MiB") + ".",
							},
						},
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: `count(sum(foo))`},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{}),
							generateSample(map[string]string{}),
							generateSample(map[string]string{}),
							generateSample(map[string]string{}),
							generateSample(map[string]string{}),
							generateSample(map[string]string{}),
							generateSample(map[string]string{}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: checks.BytesPerSampleQuery},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSampleWithValue(map[string]string{}, 1024*1024),
						},
					},
				},
			},
		},
		{
			description: "7 results with 1 series max (1KB bps)",
			content:     content,
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewCostCheck(prom, 1, 0, 0, 0, "", checks.Bug)
			},
			prometheus: newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Reporter: "query/cost",
						Summary:  "query is too expensive",
						Severity: checks.Bug,
						Diagnostics: []diags.Diagnostic{
							{
								Message: costText("prom", uri, 7) + memUsageText("7.0KiB") + maxSeriesText(1) + ".",
							},
						},
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: `count(sum(foo))`},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{}),
							generateSample(map[string]string{}),
							generateSample(map[string]string{}),
							generateSample(map[string]string{}),
							generateSample(map[string]string{}),
							generateSample(map[string]string{}),
							generateSample(map[string]string{}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: checks.BytesPerSampleQuery},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSampleWithValue(map[string]string{}, 1024),
						},
					},
				},
			},
		},
		{
			description: "6 results with 5 series max",
			content:     content,
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewCostCheck(prom, 5, 0, 0, 0, "", checks.Bug)
			},
			prometheus: newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Reporter: "query/cost",
						Summary:  "query is too expensive",
						Severity: checks.Bug,
						Diagnostics: []diags.Diagnostic{
							{
								Message: costText("prom", uri, 6) + maxSeriesText(5) + ".",
							},
						},
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: `count(sum(foo))`},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{}),
							generateSample(map[string]string{}),
							generateSample(map[string]string{}),
							generateSample(map[string]string{}),
							generateSample(map[string]string{}),
							generateSample(map[string]string{}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: checks.BytesPerSampleQuery},
					},
					resp: respondWithEmptyVector(),
				},
			},
		},
		{
			description: "7 results with 5 series max / infi",
			content:     content,
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewCostCheck(prom, 5, 0, 0, 0, "rule comment", checks.Information)
			},
			prometheus: newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Reporter: "query/cost",
						Summary:  "query is too expensive",
						Details:  "Rule comment: rule comment",
						Severity: checks.Information,
						Diagnostics: []diags.Diagnostic{
							{
								Message: costText("prom", uri, 7) + maxSeriesText(5) + ".",
							},
						},
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: `count(sum(foo))`},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{}),
							generateSample(map[string]string{}),
							generateSample(map[string]string{}),
							generateSample(map[string]string{}),
							generateSample(map[string]string{}),
							generateSample(map[string]string{}),
							generateSample(map[string]string{}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: checks.BytesPerSampleQuery},
					},
					resp: respondWithInternalError(),
				},
			},
		},
		{
			description: "7 results",
			content: `
- record: foo
  expr: 'sum({__name__="foo"})'
`,
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewCostCheck(prom, 0, 0, 0, 0, "", checks.Bug)
			},
			prometheus: newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Reporter: "query/cost",
						Summary:  "query cost estimate",
						Severity: checks.Information,
						Diagnostics: []diags.Diagnostic{
							{
								Message: costText("prom", uri, 7) + memUsageText("707B") + ".",
							},
						},
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: `count(sum({__name__="foo"}))`},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{}),
							generateSample(map[string]string{}),
							generateSample(map[string]string{}),
							generateSample(map[string]string{}),
							generateSample(map[string]string{}),
							generateSample(map[string]string{}),
							generateSample(map[string]string{}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: checks.BytesPerSampleQuery},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSampleWithValue(map[string]string{}, 101),
						},
					},
				},
			},
		},
		{
			description: "1s eval, 5s limit",
			content:     content,
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewCostCheck(prom, 0, 0, 0, time.Second*5, "", checks.Bug)
			},
			prometheus: newSimpleProm,
			problems:   noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: `count(sum(foo))`},
					},
					resp: vectorResponse{
						samples: []*model.Sample{generateSample(map[string]string{})},
						stats: promapi.QueryStats{
							Timings: promapi.QueryTimings{
								EvalTotalTime: 1,
							},
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: checks.BytesPerSampleQuery},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSampleWithValue(map[string]string{}, 4096),
						},
					},
				},
			},
		},
		{
			description: "stats",
			content:     content,
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewCostCheck(prom, 0, 100, 10, time.Second*5, "", checks.Bug)
			},
			prometheus: newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Reporter: "query/cost",
						Summary:  "query is too expensive",
						Severity: checks.Bug,
						Diagnostics: []diags.Diagnostic{
							{
								Message: totalSamplesText("prom", uri, 200, 100),
							},
						},
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: `count(sum(foo))`},
					},
					resp: vectorResponse{
						samples: []*model.Sample{generateSample(map[string]string{})},
						stats: promapi.QueryStats{
							Timings: promapi.QueryTimings{
								EvalTotalTime: 5.1,
							},
							Samples: promapi.QuerySamples{
								TotalQueryableSamples: 200,
								PeakSamples:           20,
							},
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: checks.BytesPerSampleQuery},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSampleWithValue(map[string]string{}, 4096),
						},
					},
				},
			},
		},
		{
			description: "stats - peak samples",
			content:     content,
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewCostCheck(prom, 0, 300, 10, time.Second*5, "some text", checks.Information)
			},
			prometheus: newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Reporter: "query/cost",
						Summary:  "query is too expensive",
						Details:  "Rule comment: some text",
						Severity: checks.Information,
						Diagnostics: []diags.Diagnostic{
							{
								Message: peakSamplesText("prom", uri, 20, 10),
							},
						},
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: `count(sum(foo))`},
					},
					resp: vectorResponse{
						samples: []*model.Sample{generateSample(map[string]string{})},
						stats: promapi.QueryStats{
							Timings: promapi.QueryTimings{
								EvalTotalTime: 5.1,
							},
							Samples: promapi.QuerySamples{
								TotalQueryableSamples: 200,
								PeakSamples:           20,
							},
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: checks.BytesPerSampleQuery},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSampleWithValue(map[string]string{}, 4096),
						},
					},
				},
			},
		},
		{
			description: "stats - duration",
			content:     content,
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewCostCheck(prom, 0, 300, 30, time.Second*5, "some text", checks.Information)
			},
			prometheus: newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Reporter: "query/cost",
						Summary:  "query is too expensive",
						Details:  "Rule comment: some text",
						Severity: checks.Information,
						Diagnostics: []diags.Diagnostic{
							{
								Message: evalDurText("prom", uri, "5s100ms", "5s"),
							},
						},
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: `count(sum(foo))`},
					},
					resp: vectorResponse{
						samples: []*model.Sample{generateSample(map[string]string{})},
						stats: promapi.QueryStats{
							Timings: promapi.QueryTimings{
								EvalTotalTime: 5.1,
							},
							Samples: promapi.QuerySamples{
								TotalQueryableSamples: 200,
								PeakSamples:           20,
							},
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: checks.BytesPerSampleQuery},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSampleWithValue(map[string]string{}, 4096),
						},
					},
				},
			},
		},
	}

	runTests(t, testCases)
}
