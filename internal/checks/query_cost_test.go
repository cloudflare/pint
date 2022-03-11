package checks_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/common/model"

	"github.com/cloudflare/pint/internal/checks"
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
			checker: func(uri string) checks.RuleChecker {
				return checks.NewCostCheck(simpleProm("prom", uri, time.Second*5, true), 4096, 0, checks.Bug)
			},
			problems: noProblems,
		},
		{
			description: "empty response",
			content:     content,
			checker: func(uri string) checks.RuleChecker {
				return checks.NewCostCheck(simpleProm("prom", uri, time.Second*5, true), 4096, 0, checks.Bug)
			},
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
			checker: func(uri string) checks.RuleChecker {
				return checks.NewCostCheck(simpleProm("prom", uri, time.Millisecond*50, true), 4096, 0, checks.Bug)
			},
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "sum(foo)",
						Lines:    []int{2},
						Reporter: "query/cost",
						Text: checkErrorUnableToRun(checks.CostCheckName, "prom", uri,
							fmt.Sprintf(`Post "%s/api/v1/query": context deadline exceeded`, uri)),
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
					resp: sleepResponse{sleep: time.Millisecond * 100},
				},
			},
		},
		{
			description: "bad request",
			content:     content,
			checker: func(uri string) checks.RuleChecker {
				return checks.NewCostCheck(simpleProm("prom", uri, time.Second*5, true), 4096, 0, checks.Bug)
			},
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
			checker: func(uri string) checks.RuleChecker {
				return checks.NewCostCheck(simpleProm("prom", "http://127.0.0.1:1111", time.Second*5, false), 4096, 0, checks.Bug)
			},
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "sum(foo)",
						Lines:    []int{2},
						Reporter: "query/cost",
						Text:     checkErrorUnableToRun(checks.CostCheckName, "prom", "http://127.0.0.1:1111", `Post "http://127.0.0.1:1111/api/v1/query": dial tcp 127.0.0.1:1111: connect: connection refused`),
						Severity: checks.Warning,
					},
				}
			},
		},
		{
			description: "1 result",
			content:     content,
			checker: func(uri string) checks.RuleChecker {
				return checks.NewCostCheck(simpleProm("prom", uri, time.Second*5, true), 4096, 0, checks.Bug)
			},
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
			},
		},
		{
			description: "7 results",
			content:     content,
			checker: func(uri string) checks.RuleChecker {
				return checks.NewCostCheck(simpleProm("prom", uri, time.Second*5, true), 101, 0, checks.Bug)
			},
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
			},
		},
		{
			description: "7 result with MB",
			content:     content,
			checker: func(uri string) checks.RuleChecker {
				return checks.NewCostCheck(simpleProm("prom", uri, time.Second*5, true), 1024*1024, 0, checks.Bug)
			},
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
			},
		},
		{
			description: "7 results with 1 series max (1KB bps)",
			content:     content,
			checker: func(uri string) checks.RuleChecker {
				return checks.NewCostCheck(simpleProm("prom", uri, time.Second*5, true), 1024, 1, checks.Bug)
			},
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
			},
		},
		{
			description: "6 results with 5 series max",
			content:     content,
			checker: func(uri string) checks.RuleChecker {
				return checks.NewCostCheck(simpleProm("prom", uri, time.Second*5, true), 0, 5, checks.Bug)
			},
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
			},
		},
		{
			description: "7 results with 5 series max / infi",
			content:     content,
			checker: func(uri string) checks.RuleChecker {
				return checks.NewCostCheck(simpleProm("prom", uri, time.Second*5, true), 0, 5, checks.Information)
			},
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
			},
		},
		{
			description: "7 results",
			content: `
- record: foo
  expr: 'sum({__name__="foo"})'
`,
			checker: func(uri string) checks.RuleChecker {
				return checks.NewCostCheck(simpleProm("prom", uri, time.Second*5, true), 101, 0, checks.Bug)
			},
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
			},
		},
	}

	runTests(t, testCases)
}
