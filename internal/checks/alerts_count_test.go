package checks_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/common/model"

	"github.com/cloudflare/pint/internal/checks"
)

func newAlertsCheck(uri string) checks.RuleChecker {
	return checks.NewAlertsCheck(simpleProm("prom", uri, time.Second*5, true), time.Hour*24, time.Minute, time.Minute*5)
}

func alertsText(name, uri string, count int, since string) string {
	return fmt.Sprintf(`prometheus %q at %s would trigger %d alert(s) in the last %s`, name, uri, count, since)
}

func TestAlertsCheck(t *testing.T) {
	content := "- alert: Foo Is Down\n  expr: up{job=\"foo\"} == 0\n"

	testCases := []checkTest{
		{
			description: "ignores recording rules",
			content:     "- record: foo\n  expr: up == 0\n",
			checker:     newAlertsCheck,
			problems:    noProblems,
		},
		{
			description: "ignores rules with syntax errors",
			content:     "- alert: Foo Is Down\n  expr: sum(\n",
			checker:     newAlertsCheck,
			problems:    noProblems,
		},
		{
			description: "bad request",
			content:     content,
			checker:     newAlertsCheck,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: `up{job="foo"} == 0`,
						Lines:    []int{2},
						Reporter: "alerts/count",
						Text:     checkErrorBadData("prom", uri, "bad_data: bad input data"),
						Severity: checks.Bug,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: `up{job="foo"} == 0`},
					},
					resp: respondWithBadData(),
				},
			},
		},
		{
			description: "connection refused / upstream not required / warning",
			content:     content,
			checker: func(s string) checks.RuleChecker {
				return checks.NewAlertsCheck(simpleProm("prom", "http://127.0.0.1:1111", time.Second*5, false), time.Hour*24, time.Minute, time.Minute*5)
			},
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: `up{job="foo"} == 0`,
						Lines:    []int{2},
						Reporter: "alerts/count",
						Text:     checkErrorUnableToRun(checks.AlertsCheckName, "prom", "http://127.0.0.1:1111", `Post "http://127.0.0.1:1111/api/v1/query_range": dial tcp 127.0.0.1:1111: connect: connection refused`),
						Severity: checks.Warning,
					},
				}
			},
		},
		{
			description: "empty response",
			content:     content,
			checker:     newAlertsCheck,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: `up{job="foo"} == 0`,
						Lines:    []int{2},
						Reporter: "alerts/count",
						Text:     alertsText("prom", uri, 0, "1d"),
						Severity: checks.Information,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: `up{job="foo"} == 0`},
					},
					resp: respondWithEmptyMatrix(),
				},
			},
		},
		{
			description: "multiple alerts",
			content:     content,
			checker:     newAlertsCheck,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: `up{job="foo"} == 0`,
						Lines:    []int{2},
						Reporter: "alerts/count",
						Text:     alertsText("prom", uri, 7, "1d"),
						Severity: checks.Information,
					},
				}
			},
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
								time.Now().Add(time.Hour*24),
								time.Now().Add(time.Hour*24).Add(time.Minute*6),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*23),
								time.Now().Add(time.Hour*23).Add(time.Minute*6),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*22),
								time.Now().Add(time.Hour*22).Add(time.Minute),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*21),
								time.Now().Add(time.Hour*21).Add(time.Minute*16),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*20),
								time.Now().Add(time.Hour*20).Add(time.Minute*36),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*19),
								time.Now().Add(time.Hour*19).Add(time.Minute*36),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*18),
								time.Now().Add(time.Hour*18).Add(time.Hour*2),
								time.Minute,
							),
						},
					},
				},
			},
		},
		{
			description: "for: 10m",
			content:     "- alert: Foo Is Down\n  for: 10m\n  expr: up{job=\"foo\"} == 0\n",
			checker:     newAlertsCheck,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: `up{job="foo"} == 0`,
						Lines:    []int{2, 3},
						Reporter: "alerts/count",
						Text:     alertsText("prom", uri, 2, "1d"),
						Severity: checks.Information,
					},
				}
			},
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
								time.Now().Add(time.Hour*24),
								time.Now().Add(time.Hour*24).Add(time.Minute*6),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*23),
								time.Now().Add(time.Hour*23).Add(time.Minute*6),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*22),
								time.Now().Add(time.Hour*22).Add(time.Minute),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*21),
								time.Now().Add(time.Hour*21).Add(time.Minute*16),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*20),
								time.Now().Add(time.Hour*20).Add(time.Minute*9).Add(time.Second*59),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*18),
								time.Now().Add(time.Hour*18).Add(time.Hour*2),
								time.Minute,
							),
						},
					},
				},
			},
		},
		{
			description: "{__name__=}",
			content: `
- alert: foo
  expr: '{__name__="up", job="foo"} == 0'
`,
			checker: newAlertsCheck,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: `{__name__="up", job="foo"} == 0`,
						Lines:    []int{3},
						Reporter: "alerts/count",
						Text:     alertsText("prom", uri, 3, "1d"),
						Severity: checks.Information,
					},
				}
			},
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
								time.Now().Add(time.Hour*24),
								time.Now().Add(time.Hour*24).Add(time.Minute*6),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*23),
								time.Now().Add(time.Hour*23).Add(time.Minute*6),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*22),
								time.Now().Add(time.Hour*22).Add(time.Minute),
								time.Minute,
							),
						},
					},
				},
			},
		},
		{
			description: "{__name__=~}",
			content: `
- alert: foo
  expr: '{__name__=~"(up|foo)", job="foo"} == 0'
`,
			checker: newAlertsCheck,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: `{__name__=~"(up|foo)", job="foo"} == 0`,
						Lines:    []int{3},
						Reporter: "alerts/count",
						Text:     alertsText("prom", uri, 3, "1d"),
						Severity: checks.Information,
					},
				}
			},
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
								time.Now().Add(time.Hour*21),
								time.Now().Add(time.Hour*21).Add(time.Minute*16),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*20),
								time.Now().Add(time.Hour*20).Add(time.Minute*9).Add(time.Second*59),
								time.Minute,
							),
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*18),
								time.Now().Add(time.Hour*18).Add(time.Hour*2),
								time.Minute,
							),
						},
					},
				},
			},
		},
	}

	runTests(t, testCases)
}
