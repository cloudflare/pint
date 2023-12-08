package checks_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/common/model"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/promapi"
)

func newVectorMatchingCheck(prom *promapi.FailoverGroup) checks.RuleChecker {
	return checks.NewVectorMatchingCheck(prom)
}

func differentLabelsText(l, r string) string {
	return fmt.Sprintf("This query will never return anything because the right and the left hand side have different labels: `[%s]` != `[%s]`.", l, r)
}

func usingMismatchText(f, l, r string) string {
	return fmt.Sprintf("Using `%s` won't produce any results because both sides of the query have different labels: `[%s]` != `[%s]`.", f, l, r)
}

func differentFilters(k, lv, rv string) string {
	return fmt.Sprintf("The left hand side uses `{%s=%q}` while the right hand side uses `{%s=%q}`, this will never match.", k, lv, k, rv)
}

func TestVectorMatchingCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "ignores rules with syntax errors",
			content:     "- record: foo\n  expr: sum(foo) without(\n",
			checker:     newVectorMatchingCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
		},
		{
			description: "ignores rules with bogus calls",
			content:     "- record: foo\n  expr: sum(foo, 5) without(\n",
			checker:     newVectorMatchingCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
		},
		{
			description: "one to one matching",
			content:     "- record: foo\n  expr: foo_with_notfound / bar\n",
			checker:     newVectorMatchingCheck,
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines:    []int{2},
						Reporter: checks.VectorMatchingCheckName,
						Text:     differentLabelsText("instance, job, notfound", "instance, job"),
						Details:  checks.VectorMatchingCheckDetails,
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
						formCond{key: "query", value: "count(foo_with_notfound) without(__name__)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"instance": "aaa",
								"job":      "bbb",
								"notfound": "xxx",
							}),
							generateSample(map[string]string{
								"instance": "bbb",
								"job":      "bbb",
								"notfound": "xxx",
							}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(bar) without(__name__)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"instance": "aaa",
								"job":      "bbb",
							}),
							generateSample(map[string]string{
								"instance": "bbb",
								"job":      "bbb",
							}),
							generateSample(map[string]string{
								"instance": "ccc",
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
			prometheus:  newSimpleProm,
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
						formCond{key: "query", value: "count(xxx) without(__name__)"},
					},
					resp: respondWithEmptyVector(),
				},
			},
		},
		{
			description: "ignore missing right side",
			content:     "- record: foo\n  expr: foo / xxx\n",
			checker:     newVectorMatchingCheck,
			prometheus:  newSimpleProm,
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
						formCond{key: "query", value: "count(foo) without(__name__)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"instance": "aaa",
								"job":      "bbb",
							}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(xxx) without(__name__)"},
					},
					resp: respondWithEmptyVector(),
				},
			},
		},
		{
			description: "ignore missing or vector",
			content:     "- record: foo\n  expr: sum(missing or vector(0))\n",
			checker:     newVectorMatchingCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
		},
		{
			description: "ignore present or vector",
			content:     "- record: foo\n  expr: sum(foo or vector(0))\n",
			checker:     newVectorMatchingCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
		},
		{
			description: "ignore with mismatched series",
			content:     "- record: foo\n  expr: foo / ignoring(xxx) app_registry\n",
			checker:     newVectorMatchingCheck,
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines:    []int{2},
						Reporter: checks.VectorMatchingCheckName,
						Text:     usingMismatchText(`ignoring(xxx)`, "instance, job", "app_name"),
						Details:  checks.VectorMatchingCheckDetails,
						Severity: checks.Bug,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(foo / ignoring (xxx) app_registry)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(foo) without(__name__,xxx)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"instance": "aaa",
								"job":      "bbb",
							}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(app_registry) without(__name__,xxx)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"app_name": "aaa",
							}),
							generateSample(map[string]string{
								"app_name": "aaa",
								"cluster":  "dev",
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
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines:    []int{2},
						Reporter: checks.VectorMatchingCheckName,
						Text:     "Using `on(notfound)` won't produce any results because both sides of the query don't have this label.",
						Details:  checks.VectorMatchingCheckDetails,
						Severity: checks.Bug,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(foo / on (notfound) bar)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(foo) without(__name__)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"instance": "aaa",
								"job":      "bbb",
							}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(bar) without(__name__)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
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
			prometheus:  newSimpleProm,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(foo / ignoring (notfound) foo)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(foo) without(__name__,notfound)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
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
			prometheus:  newSimpleProm,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(foo_with_notfound / ignoring (notfound) foo_with_notfound)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(foo_with_notfound) without(__name__,notfound)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"instance": "aaa",
								"job":      "bbb",
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
			prometheus:  newSimpleProm,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(foo / ignoring (notfound) foo_with_notfound)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(foo) without(__name__,notfound)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"instance": "aaa",
								"job":      "bbb",
							}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(foo_with_notfound) without(__name__,notfound)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"instance": "aaa",
								"job":      "bbb",
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
			prometheus:  newSimpleProm,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(foo_with_notfound / ignoring (notfound) foo)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(foo_with_notfound) without(__name__,notfound)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"instance": "aaa",
								"job":      "bbb",
							}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(foo) without(__name__,notfound)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
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
			prometheus:  newSimpleProm,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(foo_with_notfound / ignoring (notfound) bar)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(foo_with_notfound) without(__name__,notfound)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"instance": "aaa",
								"job":      "bbb",
							}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(bar) without(__name__,notfound)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
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
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines:    []int{2},
						Reporter: checks.VectorMatchingCheckName,
						Text:     "Using `on(notfound)` won't produce any results because the left hand side of the query doesn't have this label: `foo`.",
						Details:  checks.VectorMatchingCheckDetails,
						Severity: checks.Bug,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(foo / on (notfound) bar_with_notfound)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(foo) without(__name__)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"instance": "aaa",
								"job":      "bbb",
							}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(bar_with_notfound) without(__name__)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
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
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines:    []int{2},
						Reporter: checks.VectorMatchingCheckName,
						Text:     "Using `on(notfound)` won't produce any results because the right hand side of the query doesn't have this label: `bar`.",
						Details:  checks.VectorMatchingCheckDetails,
						Severity: checks.Bug,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(foo_with_notfound / on (notfound) bar)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(foo_with_notfound) without(__name__)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
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
						formCond{key: "query", value: "count(bar) without(__name__)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
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
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines:    []int{2},
						Reporter: checks.VectorMatchingCheckName,
						Text:     "Using `on(app_name)` won't produce any results because the left hand side of the query doesn't have this label: `(memory_bytes / ignoring (job) (memory_limit > 0))`.",
						Details:  checks.VectorMatchingCheckDetails,
						Severity: checks.Bug,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count((memory_bytes / ignoring (job) memory_limit) * on (app_name) group_left (a, b, c) app_registry)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(memory_bytes / ignoring (job) memory_limit)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(app_registry) without(__name__)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"app_name": "foo",
							}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count((memory_bytes / ignoring (job) memory_limit)) without(__name__)"},
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
			checker:    newVectorMatchingCheck,
			prometheus: newSimpleProm,
			problems:   noProblems,
		},
		{
			description: "skips number comparison on LHS",
			content:     "- record: foo\n  expr: 2 < foo / bar\n",
			checker:     newVectorMatchingCheck,
			prometheus:  newSimpleProm,
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
						formCond{key: "query", value: "count(foo) without(__name__)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(bar) without(__name__)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{}),
						},
					},
				},
			},
		},
		{
			description: "skips number comparison on RHS",
			content:     "- record: foo\n  expr: foo / bar > 0\n",
			checker:     newVectorMatchingCheck,
			prometheus:  newSimpleProm,
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
						formCond{key: "query", value: "count(foo) without(__name__)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(bar) without(__name__)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{}),
						},
					},
				},
			},
		},
		{
			description: "skips number comparison on both sides",
			content:     "- record: foo\n  expr: 1 > bool 1\n",
			checker:     newVectorMatchingCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
		},
		{
			description: "up == 0 AND foo > 0",
			content:     "- alert: foo\n  expr: up == 0 AND foo > 0\n",
			checker:     newVectorMatchingCheck,
			prometheus:  newSimpleProm,
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
						formCond{key: "query", value: "count(up) without(__name__)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(foo) without(__name__)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{}),
						},
					},
				},
			},
		},
		{
			description: "subquery is trimmed",
			content:     "- alert: foo\n  expr: min_over_time((foo_with_notfound > 0)[30m:1m]) / bar\n",
			checker:     newVectorMatchingCheck,
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines:    []int{2},
						Reporter: checks.VectorMatchingCheckName,
						Text:     "This query will never return anything because the right and the left hand side have different labels: `[instance, job, notfound]` != `[instance, job]`.",
						Details:  checks.VectorMatchingCheckDetails,
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
						formCond{key: "query", value: "count(min_over_time(foo_with_notfound[30m:1m])) without(__name__)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
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
						formCond{key: "query", value: "count(bar) without(__name__)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
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
			prometheus:  newSimpleProm,
			problems:    noProblems,
		},
		{
			description: "binary expression on both sides / passing",
			content:     "- alert: foo\n  expr: (foo / ignoring(notfound) foo_with_notfound) / (foo / ignoring(notfound) foo_with_notfound)\n",
			checker:     newVectorMatchingCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count((foo / ignoring (notfound) foo_with_notfound) / (foo / ignoring (notfound) foo_with_notfound))"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count((foo / ignoring (notfound) foo_with_notfound)) without(__name__)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
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
						formCond{key: "query", value: "count(foo / ignoring (notfound) foo_with_notfound)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(foo) without(__name__,notfound)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"instance": "aaa",
								"job":      "bbb",
							}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(foo_with_notfound) without(__name__,notfound)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"instance": "aaa",
								"job":      "bbb",
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
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines:    []int{2},
						Reporter: checks.VectorMatchingCheckName,
						Text:     "This query will never return anything because the right and the left hand side have different labels: `[instance, job]` != `[dev, instance, job]`.",
						Details:  checks.VectorMatchingCheckDetails,
						Severity: checks.Bug,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count((foo / ignoring (notfound) foo_with_notfound) / (memory_bytes / ignoring (job) memory_limit))"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count((foo / ignoring (notfound) foo_with_notfound)) without(__name__)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"instance": "aaa",
								"job":      "bbb",
							}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(foo) without(__name__,notfound)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"instance": "aaa",
								"job":      "bbb",
							}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(foo_with_notfound) without(__name__,notfound)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"instance": "aaa",
								"job":      "bbb",
							}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count((memory_bytes / ignoring (job) memory_limit)) without(__name__)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
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
						formCond{key: "query", value: "count(foo / ignoring (notfound) foo_with_notfound)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(memory_bytes / ignoring (job) memory_limit)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(memory_bytes) without(__name__,job)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"instance": "aaa",
								"dev":      "ccc",
							}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(memory_limit) without(__name__,job)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"instance": "aaa",
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
			checker:     newVectorMatchingCheck,
			prometheus: func(_ string) *promapi.FailoverGroup {
				return simpleProm("prom", "http://127.0.0.1:1111", time.Second, true)
			},
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines:    []int{2},
						Reporter: checks.VectorMatchingCheckName,
						Text:     checkErrorUnableToRun(checks.VectorMatchingCheckName, "prom", "http://127.0.0.1:1111", "connection refused"),
						Severity: checks.Bug,
					},
				}
			},
		},
		{
			description: "connection refused / not required",
			content:     "- record: foo\n  expr: xxx/yyy\n",
			checker:     newVectorMatchingCheck,
			prometheus: func(_ string) *promapi.FailoverGroup {
				return simpleProm("prom", "http://127.0.0.1:1111", time.Second, false)
			},
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines:    []int{2},
						Reporter: checks.VectorMatchingCheckName,
						Text:     checkErrorUnableToRun(checks.VectorMatchingCheckName, "prom", "http://127.0.0.1:1111", "connection refused"),
						Severity: checks.Warning,
					},
				}
			},
		},
		{
			description: "error on topk1() left side",
			content:     "- record: foo\n  expr: xxx/yyy\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewVectorMatchingCheck(prom)
			},
			prometheus: newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines:    []int{2},
						Reporter: checks.VectorMatchingCheckName,
						Text:     checkErrorUnableToRun(checks.VectorMatchingCheckName, "prom", uri, `server_error: internal error`),
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
						formCond{key: "query", value: `count(xxx) without(__name__)`},
					},
					resp: respondWithInternalError(),
				},
			},
		},
		{
			description: "error on topk1() right side",
			content:     "- record: foo\n  expr: xxx/yyy\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewVectorMatchingCheck(prom)
			},
			prometheus: newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines:    []int{2},
						Reporter: checks.VectorMatchingCheckName,
						Text:     checkErrorUnableToRun(checks.VectorMatchingCheckName, "prom", uri, `server_error: internal error`),
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
						formCond{key: "query", value: `count(xxx) without(__name__)`},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"instance": "xx",
								"job":      "xx",
							}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: `count(yyy) without(__name__)`},
					},
					resp: respondWithInternalError(),
				},
			},
		},
		{
			description: `up{job="a"} / up{job="b"}`,
			content:     "- record: foo\n  expr: up{job=\"a\"} / up{job=\"b\"}\n",
			checker:     newVectorMatchingCheck,
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines:    []int{2},
						Reporter: checks.VectorMatchingCheckName,
						Text:     differentFilters("job", "a", "b"),
						Details:  checks.VectorMatchingCheckDetails,
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
			prometheus:  newSimpleProm,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: `count(up{job="a"} / on () up{job="b"})`},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: `count(up{job="a"}) without(__name__)`},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"instance": "a",
								"job":      "a",
							}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: `count(up{job="b"}) without(__name__)`},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
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
