package checks_test

import (
	"testing"
	"time"

	"github.com/prometheus/common/model"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/promapi"
)

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
		},
		{
			description: "empty response",
			content:     content,
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewCostCheck(prom, 0, 0, 0, 0, "", checks.Bug)
			},
			prometheus: newSimpleProm,
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
			problems: true,
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
			problems:   true,
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
			problems: true,
		},
		{
			description: "1 result",
			content:     content,
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewCostCheck(prom, 0, 0, 0, 0, "", checks.Bug)
			},
			prometheus: newSimpleProm,
			problems:   true,
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
			problems:   true,
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
			problems:   true,
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
			problems:   true,
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
				return checks.NewCostCheck(prom, 5, 0, 0, 0, "Rule comment", checks.Bug)
			},
			prometheus: newSimpleProm,
			problems:   true,
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
			problems:   true,
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
			problems:   true,
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
			problems:   true,
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
			problems:   true,
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
			problems:   true,
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
			description: "ignores self",
			content:     "- record: foo\n  expr: up == 0\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewCostCheck(prom, 100, 100, 0, 0, "check comment", checks.Warning)
			},
			prometheus: newSimpleProm,
			entries:    mustParseContent("- record: foo\n  expr: up == 0\n"),
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
					},
					resp: vectorResponse{
						samples: []*model.Sample{},
					},
				},
			},
		},
		{
			description: "suggest recording rule / aggregation",
			content:     "- alert: foo\n  expr: sum(rate(foo_total[5m])) without(instance) > 10\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewCostCheck(prom, 100, 100, 0, 0, "check comment", checks.Warning)
			},
			prometheus: newSimpleProm,
			entries: mustParseContent(`
- alert: foo
  expr: vector(1)
- record: colo:foo
  expr: sum(rate(foo_total[5m])) without(instance)
`),
			problems: true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: `count(sum(rate(foo_total[5m])) without(instance) > 10)`},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{}),
						},
						stats: promapi.QueryStats{
							Samples: promapi.QuerySamples{
								TotalQueryableSamples: 100,
								PeakSamples:           50,
							},
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: `count(colo:foo > 10)`},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{}),
						},
						stats: promapi.QueryStats{
							Samples: promapi.QuerySamples{
								TotalQueryableSamples: 10,
								PeakSamples:           9,
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
			description: "suggest recording rule / rate",
			content:     "- alert: foo\n  expr: sum(rate(foo_total[5m])) without(instance) > 10\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewCostCheck(prom, 100, 100, 0, 0, "check comment", checks.Warning)
			},
			prometheus: newSimpleProm,
			entries: mustParseContent(`
- record: foo:rate5m
  expr: rate(foo_total[5m])
`),
			problems: true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: `count(sum(rate(foo_total[5m])) without(instance) > 10)`},
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
						stats: promapi.QueryStats{
							Samples: promapi.QuerySamples{
								TotalQueryableSamples: 100,
								PeakSamples:           50,
							},
							Timings: promapi.QueryTimings{
								EvalTotalTime: 30.3,
							},
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: `count(sum(foo:rate5m) without(instance) > 10)`},
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
						stats: promapi.QueryStats{
							Samples: promapi.QuerySamples{
								TotalQueryableSamples: 10,
								PeakSamples:           50,
							},
							Timings: promapi.QueryTimings{
								EvalTotalTime: 30.3,
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
			description: "suggest recording rule / ignore vector",
			content:     "- alert: foo\n  expr: sum(rate(foo_total[5m]) or vector(0)) without(instance) > 10\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewCostCheck(prom, 100, 100, 0, 0, "check comment", checks.Warning)
			},
			prometheus: newSimpleProm,
			entries: mustParseContent(`
- record: colo:foo
  expr: vector(0)
`),
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
					},
					resp: vectorResponse{
						samples: []*model.Sample{},
					},
				},
			},
		},
		{
			description: "suggest recording rule / ignore selector",
			content:     "- alert: foo\n  expr: sum(foo == 1) without(instance) > 10\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewCostCheck(prom, 100, 100, 0, 0, "check comment", checks.Warning)
			},
			prometheus: newSimpleProm,
			entries: mustParseContent(`
- record: colo:foo
  expr: foo == 1
`),
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
					},
					resp: vectorResponse{
						samples: []*model.Sample{},
					},
				},
			},
		},
		{
			description: "suggest recording rule / ignore multi-source",
			content:     "- alert: foo\n  expr: sum(foo == 1) without(instance) > 10\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewCostCheck(prom, 100, 100, 0, 0, "check comment", checks.Warning)
			},
			prometheus: newSimpleProm,
			entries: mustParseContent(`
- record: colo:foo
  expr: foo == 1 or bar == 1
`),
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
					},
					resp: vectorResponse{
						samples: []*model.Sample{},
					},
				},
			},
		},
		{
			description: "suggest recording rule / irate vs rate",
			content: `
- alert: Host_CPU_Utilization_High
  expr: |
    server_role{role="foo"}
    and on(instance)
    sum by (instance) (irate(node_cpu_seconds_total{job="foo", mode!="idle"}[5m])) > 20
`,
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewCostCheck(prom, 100, 100, 0, 0, "check comment", checks.Warning)
			},
			prometheus: newSimpleProm,
			entries: mustParseContent(`
- record: instance_mode:node_cpu:sum
  expr: sum(node_cpu_seconds_total) without (cpu)

- record: instance_mode:node_cpu:rate2m
  expr: sum(rate(node_cpu_seconds_total[2m])) without (cpu)

- record: colo:node_cpu:rate2m:by_mode
  expr: sum(instance_mode:node_cpu:rate2m{mode=~"user|system|nice|softirq"}) without (instance)

- record: colo_instance:node_cpu:count
  expr: count(node_cpu_seconds_total{mode="idle"}) without (cpu, mode)
`),
			problems: true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(server_role{role=\"foo\"}\nand on(instance)\nsum by (instance) (irate(node_cpu_seconds_total{job=\"foo\", mode!=\"idle\"}[5m])) > 20\n)"},
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
						stats: promapi.QueryStats{
							Samples: promapi.QuerySamples{
								TotalQueryableSamples: 99,
								PeakSamples:           19,
							},
							Timings: promapi.QueryTimings{
								EvalTotalTime: 60.3,
							},
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(server_role{role=\"foo\"}\nand on(instance)\ninstance_mode:node_cpu:rate2m{job=\"foo\", mode!=\"idle\"} > 20\n)"},
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
						stats: promapi.QueryStats{
							Samples: promapi.QuerySamples{
								TotalQueryableSamples: 29,
								PeakSamples:           11,
							},
							Timings: promapi.QueryTimings{
								EvalTotalTime: 21.3,
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
							generateSampleWithValue(map[string]string{}, 2048),
						},
					},
				},
			},
		},
		{
			description: "suggest recording rule / replacement with different number of series",
			content:     "- alert: foo\n  expr: sum(rate(foo_total[5m])) without(instance) > 10\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewCostCheck(prom, 100, 100, 0, 0, "check comment", checks.Warning)
			},
			prometheus: newSimpleProm,
			entries: mustParseContent(`
- record: foo:rate5m
  expr: rate(foo_total[5m])
`),
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: `count(sum(rate(foo_total[5m])) without(instance) > 10)`},
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
						stats: promapi.QueryStats{
							Samples: promapi.QuerySamples{
								TotalQueryableSamples: 100,
								PeakSamples:           50,
							},
							Timings: promapi.QueryTimings{
								EvalTotalTime: 30.3,
							},
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: `count(sum(foo:rate5m) without(instance) > 10)`},
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
						stats: promapi.QueryStats{
							Samples: promapi.QuerySamples{
								TotalQueryableSamples: 10,
								PeakSamples:           50,
							},
							Timings: promapi.QueryTimings{
								EvalTotalTime: 30.3,
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
			description: "suggest recording rule / replacement is more expensive",
			content:     "- alert: foo\n  expr: sum(rate(foo_total[5m])) without(instance) > 10\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewCostCheck(prom, 100, 100, 0, 0, "check comment", checks.Warning)
			},
			prometheus: newSimpleProm,
			entries: mustParseContent(`
- record: foo:rate5m
  expr: rate(foo_total[5m])
`),
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: `count(sum(rate(foo_total[5m])) without(instance) > 10)`},
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
						stats: promapi.QueryStats{
							Samples: promapi.QuerySamples{
								TotalQueryableSamples: 100,
								PeakSamples:           50,
							},
							Timings: promapi.QueryTimings{
								EvalTotalTime: 30.3,
							},
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: `count(sum(foo:rate5m) without(instance) > 10)`},
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
						stats: promapi.QueryStats{
							Samples: promapi.QuerySamples{
								TotalQueryableSamples: 101,
								PeakSamples:           50,
							},
							Timings: promapi.QueryTimings{
								EvalTotalTime: 30.3,
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
			description: "suggest recording rule / rule mismatch",
			content: `
- alert: Host_CPU_Utilization_High
  expr: |
    server_role{role="foo"}
    and on(instance)
    sum by (instance) (irate(node_cpu_seconds_total{job="foo", mode!="idle"}[5m])) > 20
`,
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewCostCheck(prom, 100, 100, 0, 0, "check comment", checks.Warning)
			},
			prometheus: newSimpleProm,
			entries: mustParseContent(`
- record: scalar
  expr: scalar(foo)

- record: time
  expr: time()

- record: wrong_metric
  expr: sum(rate(node_foo_seconds_total[2m])) without (cpu)

- record: no_name
  expr: sum(rate({job="foo"}[2m])) without (cpu)

- record: colo:node_cpu:rate2m:nojob
  expr: sum(rate(node_cpu_seconds_total[2m])) without (cpu, instance, job)

- record: colo:node_cpu:rate2m
  expr: sum(rate(node_cpu_seconds_total[2m])) without (cpu, instance)
`),
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(server_role{role=\"foo\"}\nand on(instance)\nsum by (instance) (irate(node_cpu_seconds_total{job=\"foo\", mode!=\"idle\"}[5m])) > 20\n)"},
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
						stats: promapi.QueryStats{
							Samples: promapi.QuerySamples{
								TotalQueryableSamples: 99,
								PeakSamples:           19,
							},
							Timings: promapi.QueryTimings{
								EvalTotalTime: 60.3,
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
							generateSampleWithValue(map[string]string{}, 2048),
						},
					},
				},
			},
		},
		{
			description: "suggest recording rule / no matchers",
			content: `- alert: Host_CPU_Utilization_High
  expr: |
    server_role{role="foo"}
    and on(instance)
    sum by (instance) (irate(node_cpu_seconds_total[5m])) > 20
`,
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewCostCheck(prom, 100, 100, 0, 0, "check comment", checks.Warning)
			},
			prometheus: newSimpleProm,
			entries: mustParseContent(`

- record: colo:node_cpu:rate2m
  expr: rate(node_cpu_seconds_total[2m])
`),
			problems: true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(server_role{role=\"foo\"}\nand on(instance)\nsum by (instance) (irate(node_cpu_seconds_total[5m])) > 20\n)"},
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
						stats: promapi.QueryStats{
							Samples: promapi.QuerySamples{
								TotalQueryableSamples: 99,
								PeakSamples:           19,
							},
							Timings: promapi.QueryTimings{
								EvalTotalTime: 60.3,
							},
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(server_role{role=\"foo\"}\nand on(instance)\nsum by (instance) (colo:node_cpu:rate2m) > 20\n)"},
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
						stats: promapi.QueryStats{
							Samples: promapi.QuerySamples{
								TotalQueryableSamples: 10,
								PeakSamples:           10,
							},
							Timings: promapi.QueryTimings{
								EvalTotalTime: 10,
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
							generateSampleWithValue(map[string]string{}, 2048),
						},
					},
				},
			},
		},
	}

	runTests(t, testCases)
}
