package checks_test

import (
	"testing"
	"time"

	"github.com/prometheus/common/model"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/promapi"
)

func newVectorMatchingCheck(prom *promapi.FailoverGroup) checks.RuleChecker {
	return checks.NewVectorMatchingCheck(prom)
}

func TestVectorMatchingCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "ignores rules with syntax errors",
			content:     "- record: foo\n  expr: sum(foo) without(\n",
			checker:     newVectorMatchingCheck,
			prometheus:  newSimpleProm,
		},
		{
			description: "ignores rules with bogus calls",
			content:     "- record: foo\n  expr: sum(foo, 5) without(\n",
			checker:     newVectorMatchingCheck,
			prometheus:  newSimpleProm,
		},
		{
			description: "one to one matching",
			content:     "- record: foo\n  expr: foo_with_notfound / bar\n",
			checker:     newVectorMatchingCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo_with_notfound / bar\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo_with_notfound\n) without(__name__)"},
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
						formCond{key: "query", value: "count(\nbar\n) without(__name__)"},
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
			description: "one to one matching / match",
			content:     "- record: foo\n  expr: foo / bar\n",
			checker:     newVectorMatchingCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo / bar\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo\n) without(__name__)"},
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
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nbar\n) without(__name__)"},
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
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nxxx / foo\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nxxx\n) without(__name__)"},
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
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo / xxx\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo\n) without(__name__)"},
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
						formCond{key: "query", value: "count(\nxxx\n) without(__name__)"},
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
		},
		{
			description: "ignore present or vector",
			content:     "- record: foo\n  expr: sum(foo or vector(0))\n",
			checker:     newVectorMatchingCheck,
			prometheus:  newSimpleProm,
		},
		{
			description: "ignore with mismatched series",
			content:     "- record: foo\n  expr: foo / ignoring(xxx) app_registry\n",
			checker:     newVectorMatchingCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo / ignoring (xxx) app_registry\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo\n) without(__name__,xxx)"},
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
						formCond{key: "query", value: "count(\napp_registry\n) without(__name__,xxx)"},
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
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo / on (notfound) bar\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo\n) without(__name__)"},
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
						formCond{key: "query", value: "count(\nbar\n) without(__name__)"},
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
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo / ignoring (notfound) foo\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo\n) without(__name__,notfound)"},
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
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo_with_notfound / ignoring (notfound) foo_with_notfound\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo_with_notfound\n) without(__name__,notfound)"},
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
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo / ignoring (notfound) foo_with_notfound\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo\n) without(__name__,notfound)"},
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
						formCond{key: "query", value: "count(\nfoo_with_notfound\n) without(__name__,notfound)"},
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
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo_with_notfound / ignoring (notfound) foo\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo_with_notfound\n) without(__name__,notfound)"},
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
						formCond{key: "query", value: "count(\nfoo\n) without(__name__,notfound)"},
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
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo_with_notfound / ignoring (notfound) bar\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo_with_notfound\n) without(__name__,notfound)"},
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
						formCond{key: "query", value: "count(\nbar\n) without(__name__,notfound)"},
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
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo / on (notfound) bar_with_notfound\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo\n) without(__name__)"},
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
						formCond{key: "query", value: "count(\nbar_with_notfound\n) without(__name__)"},
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
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo_with_notfound / on (notfound) bar\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo_with_notfound\n) without(__name__)"},
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
						formCond{key: "query", value: "count(\nbar\n) without(__name__)"},
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
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\n(memory_bytes / ignoring (job) memory_limit) * on (app_name) group_left (a, b, c) app_registry\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nmemory_bytes / ignoring (job) memory_limit\n)"},
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
						formCond{key: "query", value: "count(\napp_registry\n) without(__name__)"},
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
						formCond{key: "query", value: "count(\n(memory_bytes / ignoring (job) memory_limit)\n) without(__name__)"},
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
		},
		{
			description: "skips number comparison on LHS",
			content:     "- record: foo\n  expr: 2 < foo / bar\n",
			checker:     newVectorMatchingCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo / bar\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo\n) without(__name__)"},
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
						formCond{key: "query", value: "count(\nbar\n) without(__name__)"},
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
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo / bar\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo\n) without(__name__)"},
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
						formCond{key: "query", value: "count(\nbar\n) without(__name__)"},
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
		},
		{
			description: "up == 0 AND foo > 0",
			content:     "- alert: foo\n  expr: up == 0 AND foo > 0\n",
			checker:     newVectorMatchingCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nup and foo\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nup\n) without(__name__)"},
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
						formCond{key: "query", value: "count(\nfoo\n) without(__name__)"},
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
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nmin_over_time(foo_with_notfound[30m:1m]) / bar\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nmin_over_time(foo_with_notfound[30m:1m])\n) without(__name__)"},
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
						formCond{key: "query", value: "count(\nbar\n) without(__name__)"},
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
		},
		{
			description: "binary expression on both sides / passing",
			content:     "- alert: foo\n  expr: (foo / ignoring(notfound) foo_with_notfound) / (foo / ignoring(notfound) foo_with_notfound)\n",
			checker:     newVectorMatchingCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\n(foo / ignoring (notfound) foo_with_notfound) / (foo / ignoring (notfound) foo_with_notfound)\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\n(foo / ignoring (notfound) foo_with_notfound)\n) without(__name__)"},
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
						formCond{key: "query", value: "count(\nfoo / ignoring (notfound) foo_with_notfound\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo\n) without(__name__,notfound)"},
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
						formCond{key: "query", value: "count(\nfoo_with_notfound\n) without(__name__,notfound)"},
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
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\n(foo / ignoring (notfound) foo_with_notfound) / (memory_bytes / ignoring (job) memory_limit)\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\n(foo / ignoring (notfound) foo_with_notfound)\n) without(__name__)"},
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
						formCond{key: "query", value: "count(\nfoo\n) without(__name__,notfound)"},
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
						formCond{key: "query", value: "count(\nfoo_with_notfound\n) without(__name__,notfound)"},
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
						formCond{key: "query", value: "count(\n(memory_bytes / ignoring (job) memory_limit)\n) without(__name__)"},
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
						formCond{key: "query", value: "count(\nfoo / ignoring (notfound) foo_with_notfound\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nmemory_bytes / ignoring (job) memory_limit\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nmemory_bytes\n) without(__name__,job)"},
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
						formCond{key: "query", value: "count(\nmemory_limit\n) without(__name__,job)"},
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
			problems: true,
		},
		{
			description: "connection refused / not required",
			content:     "- record: foo\n  expr: xxx/yyy\n",
			checker:     newVectorMatchingCheck,
			prometheus: func(_ string) *promapi.FailoverGroup {
				return simpleProm("prom", "http://127.0.0.1:1111", time.Second, false)
			},
			problems: true,
		},
		{
			description: "error on topk1() left side",
			content:     "- record: foo\n  expr: xxx/yyy\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewVectorMatchingCheck(prom)
			},
			prometheus: newSimpleProm,
			problems:   true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nxxx / yyy\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nxxx\n) without(__name__)"},
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
			problems:   true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nxxx / yyy\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nxxx\n) without(__name__)"},
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
						formCond{key: "query", value: "count(\nyyy\n) without(__name__)"},
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
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nup{job=\"a\"} / up{job=\"b\"}\n)"},
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
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nup{job=\"a\"} / on () up{job=\"b\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nup{job=\"a\"}\n) without(__name__)"},
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
						formCond{key: "query", value: "count(\nup{job=\"b\"}\n) without(__name__)"},
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
		{
			description: "max() by(a) and on(a) foo",
			content: `
- record: foo
  expr: |
    max by (cluster, env) (
      increase(error_total{}[10m])
    ) > 0
    and on (cluster)
    cluster_metadata{cluster="foo", environment="prod"} > 0
  `,
			checker:    newVectorMatchingCheck,
			prometheus: newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nmax by (cluster, env) (increase(error_total[10m])) and on (cluster) cluster_metadata{cluster=\"foo\",environment=\"prod\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nmax by (cluster, env) (increase(error_total[10m]))\n) without(__name__)"},
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
						formCond{key: "query", value: "count(\ncluster_metadata{cluster=\"foo\",environment=\"prod\"}\n) without(__name__)"},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"cluster":     "foo",
								"environment": "prod",
								"status":      "green",
								"job":         "b",
							}),
						},
					},
				},
			},
			problems: true,
		},
	}
	runTests(t, testCases)
}
