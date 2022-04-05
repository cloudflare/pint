package checks_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/common/model"

	"github.com/cloudflare/pint/internal/checks"
)

func newVectorMatchingCheck(uri string) checks.RuleChecker {
	return checks.NewVectorMatchingCheck(simpleProm("prom", uri, time.Second, true))
}

func differentLabelsText(l, r string) string {
	return fmt.Sprintf(`both sides of the query have different labels: [%s] != [%s]`, l, r)
}

func usingMismatchText(f, l, r string) string {
	return fmt.Sprintf(`using %s won't produce any results because both sides of the query have different labels: [%s] != [%s]`, f, l, r)
}

func differentFilters(k, lv, rv string) string {
	return fmt.Sprintf("left hand side uses {%s=%q} while right hand side uses {%s=%q}, this will never match", k, lv, k, rv)
}

func TestVectorMatchingCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "ignores rules with syntax errors",
			content:     "- record: foo\n  expr: sum(foo) without(\n",
			checker:     newVectorMatchingCheck,
			problems:    noProblems,
		},
		{
			description: "one to one matching",
			content:     "- record: foo\n  expr: foo_with_notfound / bar\n",
			checker:     newVectorMatchingCheck,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "foo_with_notfound / bar",
						Lines:    []int{2},
						Reporter: checks.VectorMatchingCheckName,
						Text:     differentLabelsText("instance job notfound", "instance job"),
						Severity: checks.Bug,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(foo_with_notfound / bar)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "topk(1, foo_with_notfound)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"__name__": "foo",
								"instance": "aaa",
								"job":      "bbb",
								"notfound": "xxx",
							}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "topk(1, bar)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"__name__": "bar",
								"instance": "aaa",
								"job":      "bbb",
							}),
						},
					},
				},
			},
		},
		{
			description: "ignore missing left side",
			content:     "- record: foo\n  expr: xxx / foo\n",
			checker:     newVectorMatchingCheck,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(xxx / foo)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "topk(1, xxx)"},
					},
					resp: respondWithEmptyVector(),
				},
			},
		},
		{
			description: "ignore missing right side",
			content:     "- record: foo\n  expr: foo / xxx\n",
			checker:     newVectorMatchingCheck,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(foo / xxx)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "topk(1, foo)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"__name__": "foo",
								"instance": "aaa",
								"job":      "bbb",
							}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "topk(1, xxx)"},
					},
					resp: respondWithEmptyVector(),
				},
			},
		},
		{
			description: "ignore missing or vector",
			content:     "- record: foo\n  expr: sum(missing or vector(0))\n",
			checker:     newVectorMatchingCheck,
			problems:    noProblems,
		},
		{
			description: "ignore present or vector",
			content:     "- record: foo\n  expr: sum(foo or vector(0))\n",
			checker:     newVectorMatchingCheck,
			problems:    noProblems,
		},
		{
			description: "ignore with mismatched series",
			content:     "- record: foo\n  expr: foo / ignoring(xxx) app_registry\n",
			checker:     newVectorMatchingCheck,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "foo / ignoring(xxx) app_registry",
						Lines:    []int{2},
						Reporter: checks.VectorMatchingCheckName,
						Text:     usingMismatchText(`ignoring("xxx")`, "instance job", "app_name"),
						Severity: checks.Bug,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(foo / ignoring(xxx) app_registry)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "topk(1, foo)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"__name__": "foo",
								"instance": "aaa",
								"job":      "bbb",
							}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "topk(1, app_registry)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"__name__": "app_registry",
								"app_name": "aaa",
							}),
						},
					},
				},
			},
		},
		{
			description: "one to one matching with on() - both missing",
			content:     "- record: foo\n  expr: foo / on(notfound) bar\n",
			checker:     newVectorMatchingCheck,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "foo / on(notfound) bar",
						Lines:    []int{2},
						Reporter: checks.VectorMatchingCheckName,
						Text:     `using on("notfound") won't produce any results because both sides of the query don't have this label`,
						Severity: checks.Bug,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(foo / on(notfound) bar)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "topk(1, foo)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"__name__": "foo",
								"instance": "aaa",
								"job":      "bbb",
							}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "topk(1, bar)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"__name__": "bar",
								"instance": "aaa",
								"job":      "bbb",
							}),
						},
					},
				},
			},
		},
		{
			description: "one to one matching with ignoring() - both missing",
			content:     "- record: foo\n  expr: foo / ignoring(notfound) foo\n",
			checker:     newVectorMatchingCheck,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(foo / ignoring(notfound) foo)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "topk(1, foo)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"__name__": "foo",
								"instance": "aaa",
								"job":      "bbb",
							}),
						},
					},
				},
			},
		},
		{
			description: "one to one matching with ignoring() - both present",
			content:     "- record: foo\n  expr: foo_with_notfound / ignoring(notfound) foo_with_notfound\n",
			checker:     newVectorMatchingCheck,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(foo_with_notfound / ignoring(notfound) foo_with_notfound)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "topk(1, foo_with_notfound)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"__name__": "foo_with_notfound",
								"instance": "aaa",
								"job":      "bbb",
								"notfound": "ccc",
							}),
						},
					},
				},
			},
		},
		{
			description: "one to one matching with ignoring() - left missing",
			content:     "- record: foo\n  expr: foo / ignoring(notfound) foo_with_notfound\n",
			checker:     newVectorMatchingCheck,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(foo / ignoring(notfound) foo_with_notfound)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "topk(1, foo)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"__name__": "foo",
								"instance": "aaa",
								"job":      "bbb",
							}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "topk(1, foo_with_notfound)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"__name__": "foo_with_notfound",
								"instance": "aaa",
								"job":      "bbb",
								"notfound": "ccc",
							}),
						},
					},
				},
			},
		},
		{
			description: "one to one matching with ignoring() - right missing",
			content:     "- record: foo\n  expr: foo_with_notfound / ignoring(notfound) foo\n",
			checker:     newVectorMatchingCheck,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(foo_with_notfound / ignoring(notfound) foo)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "topk(1, foo_with_notfound)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"__name__": "foo_with_notfound",
								"instance": "aaa",
								"job":      "bbb",
								"notfound": "ccc",
							}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "topk(1, foo)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"__name__": "foo",
								"instance": "aaa",
								"job":      "bbb",
							}),
						},
					},
				},
			},
		},
		{
			description: "one to one matching with ignoring() - mismatch",
			content:     "- record: foo\n  expr: foo_with_notfound / ignoring(notfound) bar\n",
			checker:     newVectorMatchingCheck,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(foo_with_notfound / ignoring(notfound) bar)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "topk(1, foo_with_notfound)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"__name__": "foo_with_notfound",
								"instance": "aaa",
								"job":      "bbb",
								"notfound": "ccc",
							}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "topk(1, bar)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"__name__": "bar",
								"instance": "aaa",
								"job":      "bbb",
							}),
						},
					},
				},
			},
		},
		{
			description: "one to one matching with on() - left missing",
			content:     "- record: foo\n  expr: foo / on(notfound) bar_with_notfound\n",
			checker:     newVectorMatchingCheck,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "foo / on(notfound) bar_with_notfound",
						Lines:    []int{2},
						Reporter: checks.VectorMatchingCheckName,
						Text:     `using on("notfound") won't produce any results because left hand side of the query doesn't have this label: "foo"`,
						Severity: checks.Bug,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(foo / on(notfound) bar_with_notfound)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "topk(1, foo)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"__name__": "foo",
								"instance": "aaa",
								"job":      "bbb",
							}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "topk(1, bar_with_notfound)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"__name__": "bar_with_notfound",
								"instance": "aaa",
								"job":      "bbb",
								"notfound": "ccc",
							}),
						},
					},
				},
			},
		},
		{
			description: "one to one matching with on() - right missing",
			content:     "- record: foo\n  expr: foo_with_notfound / on(notfound) bar\n",
			checker:     newVectorMatchingCheck,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "foo_with_notfound / on(notfound) bar",
						Lines:    []int{2},
						Reporter: checks.VectorMatchingCheckName,
						Text:     `using on("notfound") won't produce any results because right hand side of the query doesn't have this label: "bar"`,
						Severity: checks.Bug,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(foo_with_notfound / on(notfound) bar)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "topk(1, foo_with_notfound)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"__name__": "foo",
								"instance": "aaa",
								"job":      "bbb",
								"notfound": "ccc",
							}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "topk(1, bar)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"__name__": "bar",
								"instance": "aaa",
								"job":      "bbb",
							}),
						},
					},
				},
			},
		},
		{
			description: "nested query",
			content:     "- alert: foo\n  expr: (memory_bytes / ignoring(job) (memory_limit > 0)) * on(app_name) group_left(a,b,c) app_registry\n",
			checker:     newVectorMatchingCheck,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "(memory_bytes / ignoring(job) (memory_limit > 0)) * on(app_name) group_left(a,b,c) app_registry",
						Lines:    []int{2},
						Reporter: checks.VectorMatchingCheckName,
						Text:     `using on("app_name") won't produce any results because left hand side of the query doesn't have this label: "(memory_bytes / ignoring(job) (memory_limit > 0))"`,
						Severity: checks.Bug,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count((memory_bytes / ignoring(job) memory_limit) * on(app_name) group_left(a, b, c) app_registry)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "topk(1, (memory_bytes / ignoring(job) memory_limit))"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"__name__": "memory_bytes",
								"instance": "instance1",
								"job":      "foo_job",
								"dev":      "mem",
							}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "topk(1, app_registry)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"__name__": "app_registry",
								"app_name": "foo",
							}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(memory_bytes / ignoring(job) memory_limit)"},
					},
					resp: respondWithSingleInstantVector(),
				},
			},
		},
		{
			description: "one to one matching with ignoring() - both present - {__name__=}",
			content: `
- record: foo
  expr: '{__name__="foo_with_notfound"} / ignoring(notfound) foo_with_notfound'
`,
			checker:  newVectorMatchingCheck,
			problems: noProblems,
		},
		{
			description: "skips number comparison on LHS",
			content:     "- record: foo\n  expr: 2 < foo / bar\n",
			checker:     newVectorMatchingCheck,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(foo / bar)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "topk(1, foo)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"__name__": "foo",
							}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "topk(1, bar)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"__name__": "bar",
							}),
						},
					},
				},
			},
		},
		{
			description: "skips number comparison on RHS",
			content:     "- record: foo\n  expr: foo / bar > 0\n",
			checker:     newVectorMatchingCheck,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(foo / bar)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "topk(1, foo)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"__name__": "foo",
							}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "topk(1, bar)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"__name__": "bar",
							}),
						},
					},
				},
			},
		},
		{
			description: "skips number comparison on both sides",
			content:     "- record: foo\n  expr: 1 > bool 1\n",
			checker:     newVectorMatchingCheck,
			problems:    noProblems,
		},
		{
			description: "up == 0 AND foo > 0",
			content:     "- alert: foo\n  expr: up == 0 AND foo > 0\n",
			checker:     newVectorMatchingCheck,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(up and foo)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "topk(1, up)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"__name__": "up",
							}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "topk(1, foo)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"__name__": "foo",
							}),
						},
					},
				},
			},
		},
		{
			description: "subquery is trimmed",
			content:     "- alert: foo\n  expr: min_over_time((foo_with_notfound > 0)[30m:1m]) / bar\n",
			checker:     newVectorMatchingCheck,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "min_over_time((foo_with_notfound > 0)[30m:1m]) / bar",
						Lines:    []int{2},
						Reporter: checks.VectorMatchingCheckName,
						Text:     `both sides of the query have different labels: [instance job notfound] != [instance job]`,
						Severity: checks.Bug,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(min_over_time(foo_with_notfound[30m:1m]) / bar)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "topk(1, min_over_time(foo_with_notfound[30m:1m]))"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"__name__": "foo_with_notfound",
								"instance": "aaa",
								"job":      "bbb",
								"notfound": "ccc",
							}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "topk(1, bar)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"__name__": "bar",
								"instance": "aaa",
								"job":      "bbb",
							}),
						},
					},
				},
			},
		},
		{
			description: "scalar",
			content:     "- alert: foo\n  expr: (100*(1024^2))\n",
			checker:     newVectorMatchingCheck,
			problems:    noProblems,
		},
		{
			description: "binary expression on both sides / passing",
			content:     "- alert: foo\n  expr: (foo / ignoring(notfound) foo_with_notfound) / (foo / ignoring(notfound) foo_with_notfound)\n",
			checker:     newVectorMatchingCheck,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count((foo / ignoring(notfound) foo_with_notfound) / (foo / ignoring(notfound) foo_with_notfound))"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "topk(1, (foo / ignoring(notfound) foo_with_notfound))"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"__name__": "foo",
								"instance": "aaa",
								"job":      "bbb",
								"notfound": "ccc",
							}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(foo / ignoring(notfound) foo_with_notfound)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "topk(1, foo)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"__name__": "foo",
								"instance": "aaa",
								"job":      "bbb",
							}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "topk(1, foo_with_notfound)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"__name__": "foo_with_notfound",
								"instance": "aaa",
								"job":      "bbb",
								"notfound": "ccc",
							}),
						},
					},
				},
			},
		},
		{
			description: "binary expression on both sides / mismatch",
			content:     "- alert: foo\n  expr: (foo / ignoring(notfound) foo_with_notfound) / (memory_bytes / ignoring(job) memory_limit)\n",
			checker:     newVectorMatchingCheck,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "(foo / ignoring(notfound) foo_with_notfound) / (memory_bytes / ignoring(job) memory_limit)",
						Lines:    []int{2},
						Reporter: checks.VectorMatchingCheckName,
						Text:     "both sides of the query have different labels: [instance job] != [dev instance job]",
						Severity: checks.Bug,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count((foo / ignoring(notfound) foo_with_notfound) / (memory_bytes / ignoring(job) memory_limit))"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "topk(1, (foo / ignoring(notfound) foo_with_notfound))"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"__name__": "foo",
								"instance": "aaa",
								"job":      "bbb",
							}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "topk(1, foo)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"__name__": "foo",
								"instance": "aaa",
								"job":      "bbb",
							}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "topk(1, foo_with_notfound)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"__name__": "foo_with_notfound",
								"instance": "aaa",
								"job":      "bbb",
								"notfound": "ccc",
							}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "topk(1, (memory_bytes / ignoring(job) memory_limit))"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"__name__": "memory_bytes",
								"instance": "aaa",
								"job":      "bbb",
								"dev":      "ccc",
							}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(foo / ignoring(notfound) foo_with_notfound)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(memory_bytes / ignoring(job) memory_limit)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "topk(1, memory_bytes)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"__name__": "memory_bytes",
								"instance": "aaa",
								"job":      "bbb",
								"dev":      "ccc",
							}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "topk(1, memory_limit)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"__name__": "memory_limit",
								"instance": "aaa",
								"job":      "xxx",
								"dev":      "ccc",
							}),
						},
					},
				},
			},
		},
		{
			description: "connection refused / required",
			content:     "- record: foo\n  expr: xxx/yyy\n",
			checker: func(s string) checks.RuleChecker {
				return checks.NewVectorMatchingCheck(simpleProm("prom", "http://127.0.0.1:1111", time.Second, true))
			},
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "xxx/yyy",
						Lines:    []int{2},
						Reporter: checks.VectorMatchingCheckName,
						Text:     checkErrorUnableToRun(checks.VectorMatchingCheckName, "prom", "http://127.0.0.1:1111", `Post "http://127.0.0.1:1111/api/v1/query": dial tcp 127.0.0.1:1111: connect: connection refused`),
						Severity: checks.Bug,
					},
				}
			},
		},
		{
			description: "connection refused / not required",
			content:     "- record: foo\n  expr: xxx/yyy\n",
			checker: func(s string) checks.RuleChecker {
				return checks.NewVectorMatchingCheck(simpleProm("prom", "http://127.0.0.1:1111", time.Second, false))
			},
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "xxx/yyy",
						Lines:    []int{2},
						Reporter: checks.VectorMatchingCheckName,
						Text:     checkErrorUnableToRun(checks.VectorMatchingCheckName, "prom", "http://127.0.0.1:1111", `Post "http://127.0.0.1:1111/api/v1/query": dial tcp 127.0.0.1:1111: connect: connection refused`),
						Severity: checks.Warning,
					},
				}
			},
		},
		{
			description: "error on topk1() left side",
			content:     "- record: foo\n  expr: xxx/yyy\n",
			checker: func(uri string) checks.RuleChecker {
				return checks.NewVectorMatchingCheck(simpleProm("prom", uri, time.Second, true))
			},
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "xxx/yyy",
						Lines:    []int{2},
						Reporter: checks.VectorMatchingCheckName,
						Text:     checkErrorUnableToRun(checks.VectorMatchingCheckName, "prom", uri, `server_error: server error: 500`),
						Severity: checks.Bug,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: `count(xxx / yyy)`},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: `topk(1, xxx)`},
					},
					resp: respondWithInternalError(),
				},
			},
		},
		{
			description: "error on topk1() right side",
			content:     "- record: foo\n  expr: xxx/yyy\n",
			checker: func(uri string) checks.RuleChecker {
				return checks.NewVectorMatchingCheck(simpleProm("prom", uri, time.Second, true))
			},
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "xxx/yyy",
						Lines:    []int{2},
						Reporter: checks.VectorMatchingCheckName,
						Text:     checkErrorUnableToRun(checks.VectorMatchingCheckName, "prom", uri, `server_error: server error: 500`),
						Severity: checks.Bug,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: `count(xxx / yyy)`},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: `topk(1, xxx)`},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"__name__": "xxx",
								"instance": "xx",
								"job":      "xx",
							}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: `topk(1, yyy)`},
					},
					resp: respondWithInternalError(),
				},
			},
		},
		{
			description: `up{job="a"} / up{job="b"}`,
			content:     "- record: foo\n  expr: up{job=\"a\"} / up{job=\"b\"}\n",
			checker:     newVectorMatchingCheck,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: `up{job="a"} / up{job="b"}`,
						Lines:    []int{2},
						Reporter: checks.VectorMatchingCheckName,
						Text:     differentFilters("job", "a", "b"),
						Severity: checks.Bug,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: `count(up{job="a"} / up{job="b"})`},
					},
					resp: respondWithEmptyVector(),
				},
			},
		},
		{
			description: `up{job="a"} / on() up{job="b"}`,
			content:     "- record: foo\n  expr: up{job=\"a\"} / on() up{job=\"b\"}\n",
			checker:     newVectorMatchingCheck,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: `count(up{job="a"} / on() up{job="b"})`},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: `topk(1, up{job="a"})`},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"__name__": "up",
								"instance": "a",
								"job":      "a",
							}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: `topk(1, up{job="b"})`},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"__name__": "up",
								"instance": "b",
								"job":      "b",
							}),
						},
					},
				},
			},
		},
	}
	runTests(t, testCases)
}
