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

	testCases := []checkTest{
		{
			description: "ignores recording rules",
			content:     "- record: foo\n  expr: up == 0\n",
			checker:     newAlertsCheck,
			prometheus:  newSimpleProm,
		},
		{
			description: "ignores rules with syntax errors",
			content:     "- alert: Foo Is Down\n  expr: sum(\n",
			checker:     newAlertsCheck,
			prometheus:  newSimpleProm,
		},
		{
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
			description: "connection refused / upstream not required / warning",
			content:     content,
			checker:     newAlertsCheck,
			prometheus: func(_ string) *promapi.FailoverGroup {
				return simpleProm("prom", "http://127.0.0.1:1111", time.Second*5, false)
			},
			problems: true,
		},
		{
			description: "empty response",
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
			description: "multiple alerts",
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
			description: "for: 10m",
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
			description: "minCount=2",
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
			description: "minCount=2 severity=bug",
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
			description: "minCount=10",
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
			description: "{__name__=}",
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
			description: "{__name__=~}",
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
			description: "uptime query error",
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
			description: "keep_firing_for: 10m",
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
			description: "for: 10m + keep_firing_for: 10m",
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
	}

	runTests(t, testCases)
}
