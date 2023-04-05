package checks_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/common/model"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/promapi"
)

func costText(name, uri string, count int) string {
	return fmt.Sprintf(`prometheus %q at %s returned %d result(s)`, name, uri, count)
}

func memUsageText(b string) string {
	return fmt.Sprintf(" with %s estimated memory usage", b)
}

func maxSeriesText(m int) string {
	return fmt.Sprintf(", maximum allowed series is %d", m)
}

func TestCostCheck(t *testing.T) {
	content := "- record: foo\n  expr: sum(foo)\n"

	testCases := []checkTest{
		{
			description: "ignores rules with syntax errors",
			content:     "- record: foo\n  expr: sum(foo) without(\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewCostCheck(prom, 0, checks.Bug)
			},
			prometheus: newSimpleProm,
			problems:   noProblems,
		},
		{
			description: "empty response",
			content:     content,
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewCostCheck(prom, 0, checks.Bug)
			},
			prometheus: newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "sum(foo)",
						Lines:    []int{2},
						Reporter: "query/cost",
						Text:     costText("prom", uri, 0),
						Severity: checks.Information,
					},
				}
			},
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
				return checks.NewCostCheck(prom, 0, checks.Bug)
			},
			prometheus: func(uri string) *promapi.FailoverGroup {
				return simpleProm("prom", uri, time.Millisecond*50, true)
			},
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "sum(foo)",
						Lines:    []int{2},
						Reporter: "query/cost",
						Text:     checkErrorUnableToRun(checks.CostCheckName, "prom", uri, "connection timeout"),
						Severity: checks.Bug,
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
				return checks.NewCostCheck(prom, 0, checks.Bug)
			},
			prometheus: newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "sum(foo)",
						Lines:    []int{2},
						Reporter: "query/cost",
						Text:     checkErrorBadData("prom", uri, "bad_data: bad input data"),
						Severity: checks.Bug,
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
				return checks.NewCostCheck(prom, 0, checks.Bug)
			},
			prometheus: func(s string) *promapi.FailoverGroup {
				return simpleProm("prom", "http://127.0.0.1:1111", time.Second*5, false)
			},
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "sum(foo)",
						Lines:    []int{2},
						Reporter: "query/cost",
						Text:     checkErrorUnableToRun(checks.CostCheckName, "prom", "http://127.0.0.1:1111", "connection refused"),
						Severity: checks.Warning,
					},
				}
			},
		},
		{
			description: "1 result",
			content:     content,
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewCostCheck(prom, 0, checks.Bug)
			},
			prometheus: newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "sum(foo)",
						Lines:    []int{2},
						Reporter: "query/cost",
						Text:     costText("prom", uri, 1) + memUsageText("4.0KiB"),
						Severity: checks.Information,
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
				return checks.NewCostCheck(prom, 0, checks.Bug)
			},
			prometheus: newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "sum(foo)",
						Lines:    []int{2},
						Reporter: "query/cost",
						Text:     costText("prom", uri, 7) + memUsageText("707B"),
						Severity: checks.Information,
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
				return checks.NewCostCheck(prom, 0, checks.Bug)
			},
			prometheus: newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "sum(foo)",
						Lines:    []int{2},
						Reporter: "query/cost",
						Text:     costText("prom", uri, 7) + memUsageText("7.0MiB"),
						Severity: checks.Information,
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
				return checks.NewCostCheck(prom, 1, checks.Bug)
			},
			prometheus: newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "sum(foo)",
						Lines:    []int{2},
						Reporter: "query/cost",
						Text:     costText("prom", uri, 7) + memUsageText("7.0KiB") + maxSeriesText(1),
						Severity: checks.Bug,
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
				return checks.NewCostCheck(prom, 5, checks.Bug)
			},
			prometheus: newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "sum(foo)",
						Lines:    []int{2},
						Reporter: "query/cost",
						Text:     costText("prom", uri, 6) + maxSeriesText(5),
						Severity: checks.Bug,
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
				return checks.NewCostCheck(prom, 5, checks.Information)
			},
			prometheus: newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "sum(foo)",
						Lines:    []int{2},
						Reporter: "query/cost",
						Text:     costText("prom", uri, 7) + maxSeriesText(5),
						Severity: checks.Information,
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
				return checks.NewCostCheck(prom, 0, checks.Bug)
			},
			prometheus: newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: `sum({__name__="foo"})`,
						Lines:    []int{3},
						Reporter: "query/cost",
						Text:     costText("prom", uri, 7) + memUsageText("707B"),
						Severity: checks.Information,
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
	}

	runTests(t, testCases)
}
