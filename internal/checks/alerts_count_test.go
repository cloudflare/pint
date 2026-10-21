package checks_test

import (
	"testing"
	"time"

	"github.com/prometheus/common/model"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/promapi"
)

func newAlertsCheck(prom *promapi.FailoverGroup) checks.RuleChecker {
	return checks.NewAlertsCheck(prom, time.Hour*24, time.Minute, time.Minute*5, 0, "", checks.Information)
}

func TestAlertsCountCheck(t *testing.T) {
	content := "- alert: Foo Is Down\n  expr: up{job=\"foo\"} == 0\n"
	now := time.Now()

	testCases := []checkTest{
		{
			// A recording rule is not checked, so no problems are reported.
			description: "ignores recording rules",
			content:     "- record: foo\n  expr: up == 0\n",
			checker:     newAlertsCheck,
			prometheus:  newSimpleProm,
		},
		{
			// The expression does not parse, so no problems are reported.
			description: "ignores rules with syntax errors",
			content:     "- alert: Foo Is Down\n  expr: sum(\n",
			checker:     newAlertsCheck,
			prometheus:  newSimpleProm,
		},
		{
			// The server rejects the query, so the check reports a problem with the error.
			description: "bad request",
			content:     content,
			checker:     newAlertsCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: `up{job="foo"} == 0`},
					},
					resp: respondWithBadData(),
				},
			},
			problems: true,
		},
		{
			// The optional server is unreachable, so queries report a warning problem.
			description: "connection refused / upstream not required / warning",
			content:     content,
			checker:     newAlertsCheck,
			prometheus: func(_ string) *promapi.FailoverGroup {
				return simpleProm("prom", "http://127.0.0.1:1111", time.Second*5, false)
			},
			problems: true,
		},
		{
			description: "empty response / 0 alerts",
			content:     content,
			checker:     newAlertsCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: `up{job="foo"} == 0`},
					},
					resp: respondWithEmptyMatrix(),
				},
			},
			problems: true,
		},
		{
			description: "multiple alerts / seven ranges for one series / 7 alerts",
			content:     content,
			checker:     newAlertsCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: `up{job="foo"} == 0`},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							// 7m
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-24),
								time.Now().Add(time.Hour*-24).Add(time.Minute*6),
								time.Minute,
							),
							// 7m
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-23),
								time.Now().Add(time.Hour*-23).Add(time.Minute*6),
								time.Minute,
							),
							// 2m
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-22),
								time.Now().Add(time.Hour*-22).Add(time.Minute),
								time.Minute,
							),
							// 17m
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-21),
								time.Now().Add(time.Hour*-21).Add(time.Minute*16),
								time.Minute,
							),
							// 37m
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-20),
								time.Now().Add(time.Hour*-20).Add(time.Minute*36),
								time.Minute,
							),
							// 37m
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-19),
								time.Now().Add(time.Hour*-19).Add(time.Minute*36),
								time.Minute,
							),
							// 2h1m
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-10),
								time.Now().Add(time.Hour*-10).Add(time.Hour*2),
								time.Minute,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1D(),
				},
			},
			problems: true,
		},
		{
			description: "for: 10m / 2 ranges over 10m / 2 alerts",
			content:     "- alert: Foo Is Down\n  for: 10m\n  expr: up{job=\"foo\"} == 0\n",
			checker:     newAlertsCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: `up{job="foo"} == 0`},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-24),
								time.Now().Add(time.Hour*-24).Add(time.Minute*6),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-23),
								time.Now().Add(time.Hour*-23).Add(time.Minute*6),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-22),
								time.Now().Add(time.Hour*-22).Add(time.Minute),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-21),
								time.Now().Add(time.Hour*-21).Add(time.Minute*16),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-20),
								time.Now().Add(time.Hour*-20).Add(time.Minute*9).Add(time.Second*59),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-18),
								time.Now().Add(time.Hour*-18).Add(time.Hour*2),
								time.Minute,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1D(),
				},
			},
			problems: true,
		},
		{
			description: "minCount=2 / 2 alerts reported",
			content:     "- alert: Foo Is Down\n  for: 10m\n  expr: up{job=\"foo\"} == 0\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAlertsCheck(prom, time.Hour*24, time.Minute, time.Minute*5, 2, "rule comment", checks.Information)
			},
			prometheus: newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: `up{job="foo"} == 0`},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-24),
								time.Now().Add(time.Hour*-24).Add(time.Minute*6),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-23),
								time.Now().Add(time.Hour*-23).Add(time.Minute*6),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-22),
								time.Now().Add(time.Hour*-22).Add(time.Minute),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-21),
								time.Now().Add(time.Hour*-21).Add(time.Minute*16),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-20),
								time.Now().Add(time.Hour*-20).Add(time.Minute*9).Add(time.Second*59),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-18),
								time.Now().Add(time.Hour*-18).Add(time.Hour*2),
								time.Minute,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1D(),
				},
			},
			problems: true,
		},
		{
			description: "minCount=2 severity=bug / 2 alerts reported as bug",
			content:     "- alert: Foo Is Down\n  for: 10m\n  expr: up{job=\"foo\"} == 0\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAlertsCheck(prom, time.Hour*24, time.Minute, time.Minute*5, 2, "", checks.Bug)
			},
			prometheus: newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: `up{job="foo"} == 0`},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-24),
								time.Now().Add(time.Hour*-24).Add(time.Minute*6),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-23),
								time.Now().Add(time.Hour*-23).Add(time.Minute*6),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-22),
								time.Now().Add(time.Hour*-22).Add(time.Minute),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-21),
								time.Now().Add(time.Hour*-21).Add(time.Minute*16),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-20),
								time.Now().Add(time.Hour*-20).Add(time.Minute*9).Add(time.Second*59),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-18),
								time.Now().Add(time.Hour*-18).Add(time.Hour*2),
								time.Minute,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1D(),
				},
			},
			problems: true,
		},
		{
			description: "minCount=10 / 2 alerts below minCount, nothing reported",
			content:     "- alert: Foo Is Down\n  for: 10m\n  expr: up{job=\"foo\"} == 0\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAlertsCheck(prom, time.Hour*24, time.Minute, time.Minute*5, 10, "", checks.Information)
			},
			prometheus: newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: `up{job="foo"} == 0`},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-24),
								time.Now().Add(time.Hour*-24).Add(time.Minute*6),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-23),
								time.Now().Add(time.Hour*-23).Add(time.Minute*6),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-22),
								time.Now().Add(time.Hour*-22).Add(time.Minute),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-21),
								time.Now().Add(time.Hour*-21).Add(time.Minute*16),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-20),
								time.Now().Add(time.Hour*-20).Add(time.Minute*9).Add(time.Second*59),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-18),
								time.Now().Add(time.Hour*-18).Add(time.Hour*2),
								time.Minute,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1D(),
				},
			},
		},
		{
			description: "{__name__=} / three ranges / 3 alerts",
			content: `
- alert: foo
  expr: '{__name__="up", job="foo"} == 0'
`,
			checker:    newAlertsCheck,
			prometheus: newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: `{__name__="up", job="foo"} == 0`},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-23),
								time.Now().Add(time.Hour*-23).Add(time.Minute*6),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-22),
								time.Now().Add(time.Hour*-22).Add(time.Minute*6),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-21),
								time.Now().Add(time.Hour*-21).Add(time.Minute),
								time.Minute,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1D(),
				},
			},
			problems: true,
		},
		{
			description: "{__name__=~} / three ranges / 3 alerts",
			content: `
- alert: foo
  expr: '{__name__=~"(up|foo)", job="foo"} == 0'
`,
			checker:    newAlertsCheck,
			prometheus: newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: `{__name__=~"(up|foo)", job="foo"} == 0`},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-21),
								time.Now().Add(time.Hour*-21).Add(time.Minute*16),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-20),
								time.Now().Add(time.Hour*-20).Add(time.Minute*9).Add(time.Second*59),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-10),
								time.Now().Add(time.Hour*-10).Add(time.Hour*2),
								time.Minute,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1D(),
				},
			},
			problems: true,
		},
		{
			description: "uptime query error / gap detection skipped / 3 alerts",
			content:     "- alert: Foo Is Down\n  expr: up{job=\"foo\"} == 0\n",
			checker:     newAlertsCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: `up{job="foo"} == 0`},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-23),
								time.Now().Add(time.Hour*-23).Add(time.Minute*6),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-22),
								time.Now().Add(time.Hour*-22).Add(time.Minute*6),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-21),
								time.Now().Add(time.Hour*-21).Add(time.Minute),
								time.Minute,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithInternalError(),
				},
			},
			problems: true,
		},
		{
			description: "keep_firing_for: 10m / 6 ranges 1h apart / 6 alerts",
			content:     "- alert: Foo Is Down\n  keep_firing_for: 10m\n  expr: up{job=\"foo\"} == 0\n",
			checker:     newAlertsCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: `up{job="foo"} == 0`},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-24),
								time.Now().Add(time.Hour*-24).Add(time.Minute*6),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-23),
								time.Now().Add(time.Hour*-23).Add(time.Minute*6),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-22),
								time.Now().Add(time.Hour*-22).Add(time.Minute),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-21),
								time.Now().Add(time.Hour*-21).Add(time.Minute*16),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-20),
								time.Now().Add(time.Hour*-20).Add(time.Minute*9).Add(time.Second*59),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-18),
								time.Now().Add(time.Hour*-18).Add(time.Hour*2),
								time.Minute,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1D(),
				},
			},
			problems: true,
		},
		{
			description: "for: 10m + keep_firing_for: 10m / 2 ranges over 10m / 2 alerts",
			content:     "- alert: Foo Is Down\n  for: 10m\n  keep_firing_for: 10m\n  expr: up{job=\"foo\"} == 0\n",
			checker:     newAlertsCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: `up{job="foo"} == 0`},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-24),
								time.Now().Add(time.Hour*-24).Add(time.Minute*6),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-23),
								time.Now().Add(time.Hour*-23).Add(time.Minute*6),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-22),
								time.Now().Add(time.Hour*-22).Add(time.Minute),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-21),
								time.Now().Add(time.Hour*-21).Add(time.Minute*16),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-20),
								time.Now().Add(time.Hour*-20).Add(time.Minute*9).Add(time.Second*59),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-18),
								time.Now().Add(time.Hour*-18).Add(time.Hour*2),
								time.Minute,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1D(),
				},
			},
			problems: true,
		},
		{
			description: "keep_firing_for=30m merges 15m gaps between ranges / 2 alerts",
			content:     "- alert: Foo Is Down\n  keep_firing_for: 30m\n  expr: up{job=\"foo\"} == 0\n",
			checker:     newAlertsCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: `up{job="foo"} == 0`},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{"job": "foo"},
								now.Add(time.Hour*-20),
								now.Add(time.Hour*-20).Add(time.Minute*5),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								now.Add(time.Hour*-20).Add(time.Minute*20),
								now.Add(time.Hour*-20).Add(time.Minute*25),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								now.Add(time.Hour*-20).Add(time.Minute*40),
								now.Add(time.Hour*-20).Add(time.Minute*45),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								now.Add(time.Hour*-16),
								now.Add(time.Hour*-16).Add(time.Minute*5),
								time.Minute,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1D(),
				},
			},
			problems: true,
		},
		{
			description: "keep_firing_for=2m covers a 2m gap but not wider gaps / 3 alerts",
			content:     "- alert: Foo Is Down\n  keep_firing_for: 2m\n  expr: up{job=\"foo\"} == 0\n",
			checker:     newAlertsCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: `up{job="foo"} == 0`},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{"job": "foo"},
								now.Add(time.Hour*-20),
								now.Add(time.Hour*-20),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								now.Add(time.Hour*-20).Add(time.Minute*2),
								now.Add(time.Hour*-20).Add(time.Minute*2),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								now.Add(time.Hour*-20).Add(time.Minute*8),
								now.Add(time.Hour*-20).Add(time.Minute*8),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								now.Add(time.Hour*-20).Add(time.Minute*12),
								now.Add(time.Hour*-20).Add(time.Minute*12),
								time.Minute,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					// Prometheus is up the whole window, so any gap is real.
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								now.Add(time.Hour*-24),
								now,
								time.Minute,
							),
						},
					},
				},
			},
			problems: true,
		},
		{
			// The ranges at +40m and +70m start before keep_firing_for: 30m
			// runs out, so foo has one alert. The last 2m range is too short
			// for `for`. job=bar fires its own alert.
			description: "for=10m + keep_firing_for=30m gives one alert per series / 2 alerts",
			content:     "- alert: Foo Is Down\n  for: 10m\n  keep_firing_for: 30m\n  expr: up{job=\"foo\"} == 0\n",
			checker:     newAlertsCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: `up{job="foo"} == 0`},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{"job": "bar"},
								now.Add(time.Hour*-20),
								now.Add(time.Hour*-20).Add(time.Minute*15),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								now.Add(time.Hour*-20),
								now.Add(time.Hour*-20).Add(time.Minute*15),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								now.Add(time.Hour*-20).Add(time.Minute*40),
								now.Add(time.Hour*-20).Add(time.Minute*42),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								now.Add(time.Hour*-20).Add(time.Minute*70),
								now.Add(time.Hour*-20).Add(time.Minute*85),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								now.Add(time.Hour*-16),
								now.Add(time.Hour*-16).Add(time.Minute*2),
								time.Minute,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					// Prometheus is up the whole window, so any gap is real.
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								now.Add(time.Hour*-24),
								now,
								time.Minute,
							),
						},
					},
				},
			},
			problems: true,
		},
		{
			// Two 7m ranges separated by 3m, Prometheus was down during those 3m.
			// Since Prometheus was down there's no gap, so ranges merge into ~17m, exceeding for: 10m.
			description: "Prometheus down during gap / ranges merge / 1 alert",
			content:     "- alert: Foo Is Down\n  for: 10m\n  expr: up{job=\"foo\"} == 0\n",
			checker:     newAlertsCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: `up{job="foo"} == 0`},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							// 7m range starting at -20h
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-20),
								time.Now().Add(time.Hour*-20).Add(time.Minute*6),
								time.Minute,
							),
							// 7m range starting at -20h+10m (3m gap between ranges)
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-20).Add(time.Minute*10),
								time.Now().Add(time.Hour*-20).Add(time.Minute*16),
								time.Minute,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							// Uptime from -24h to -20h+6m (covers first range)
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*-24),
								time.Now().Add(time.Hour*-20).Add(time.Minute*6),
								time.Minute,
							),
							// Uptime from -20h+10m to now (covers second range)
							// Prometheus was down from -20h+6m to -20h+10m
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*-20).Add(time.Minute*10),
								time.Now(),
								time.Minute,
							),
						},
					},
				},
			},
			problems: true,
		},
		{
			// Two 7m ranges but Prometheus was up the entire time.
			// There's a gap between them, so they don't merge.
			description: "Prometheus up during gap / ranges stay separate / 0 alerts",
			content:     "- alert: Foo Is Down\n  for: 10m\n  expr: up{job=\"foo\"} == 0\n",
			checker:     newAlertsCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: `up{job="foo"} == 0`},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							// 7m range starting at -20h
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-20),
								time.Now().Add(time.Hour*-20).Add(time.Minute*6),
								time.Minute,
							),
							// 7m range starting at -20h+10m (3m gap between ranges)
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*-20).Add(time.Minute*10),
								time.Now().Add(time.Hour*-20).Add(time.Minute*16),
								time.Minute,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*-24),
								time.Now(),
								time.Minute,
							),
						},
					},
				},
			},
			problems: true,
		},
	}

	runTests(t, testCases)
}
