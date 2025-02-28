package checks_test

import (
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/prometheus/common/model"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
)

func newAlertsCheck(prom *promapi.FailoverGroup) checks.RuleChecker {
	return checks.NewAlertsCheck(prom, time.Hour*24, time.Minute, time.Minute*5, 0, "", checks.Information)
}

func alertsText(name, uri string, count int, since string) string {
	return fmt.Sprintf("`%s` Prometheus server at %s would trigger %d alert(s) in the last %s.", name, uri, count, since)
}

func alertsDetails(uri, query, since, comment string) string {
	details := fmt.Sprintf(
		`To get a preview of the alerts that would fire please [click here](%s/graph?g0.expr=%s&g0.tab=0&g0.range_input=%s).`,
		uri, url.QueryEscape(query), since,
	)
	if comment != "" {
		details = fmt.Sprintf("%s\nRule comment: %s", details, comment)
	}
	return details
}

func TestAlertsCountCheck(t *testing.T) {
	content := "- alert: Foo Is Down\n  expr: up{job=\"foo\"} == 0\n"

	testCases := []checkTest{
		{
			description: "ignores recording rules",
			content:     "- record: foo\n  expr: up == 0\n",
			checker:     newAlertsCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
		},
		{
			description: "ignores rules with syntax errors",
			content:     "- alert: Foo Is Down\n  expr: sum(\n",
			checker:     newAlertsCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
		},
		{
			description: "bad request",
			content:     content,
			checker:     newAlertsCheck,
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "alerts/count",
						Summary:  "unable to run checks",
						Severity: checks.Bug,
						Diagnostics: []output.Diagnostic{
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
			checker:     newAlertsCheck,
			prometheus: func(_ string) *promapi.FailoverGroup {
				return simpleProm("prom", "http://127.0.0.1:1111", time.Second*5, false)
			},
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "alerts/count",
						Summary:  "unable to run checks",
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Message: checkErrorUnableToRun(checks.AlertsCheckName, "prom", "http://127.0.0.1:1111", `connection refused`),
							},
						},
					},
				}
			},
		},
		{
			description: "empty response",
			content:     content,
			checker:     newAlertsCheck,
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "alerts/count",
						Summary:  "alert count estimate",
						Details:  alertsDetails(uri, `up{job="foo"} == 0`, "1d", ""),
						Severity: checks.Information,
						Diagnostics: []output.Diagnostic{
							{
								Message: alertsText("prom", uri, 0, "1d"),
							},
						},
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
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "alerts/count",
						Summary:  "alert count estimate",
						Details:  alertsDetails(uri, `up{job="foo"} == 0`, "1d", ""),
						Severity: checks.Information,
						Diagnostics: []output.Diagnostic{
							{
								Message: alertsText("prom", uri, 7, "1d"),
							},
						},
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
						formCond{key: "query", value: `count(up)`},
					},
					resp: respondWithSingleRangeVector1D(),
				},
			},
		},
		{
			description: "for: 10m",
			content:     "- alert: Foo Is Down\n  for: 10m\n  expr: up{job=\"foo\"} == 0\n",
			checker:     newAlertsCheck,
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 3,
							Last:  3,
						},
						Reporter: "alerts/count",
						Summary:  "alert count estimate",
						Details:  alertsDetails(uri, `up{job="foo"} == 0`, "1d", ""),
						Severity: checks.Information,
						Diagnostics: []output.Diagnostic{
							{
								Message: alertsText("prom", uri, 2, "1d"),
							},
						},
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
						formCond{key: "query", value: `count(up)`},
					},
					resp: respondWithSingleRangeVector1D(),
				},
			},
		},
		{
			description: "minCount=2",
			content:     "- alert: Foo Is Down\n  for: 10m\n  expr: up{job=\"foo\"} == 0\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAlertsCheck(prom, time.Hour*24, time.Minute, time.Minute*5, 2, "rule comment", checks.Information)
			},
			prometheus: newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 3,
							Last:  3,
						},
						Reporter: "alerts/count",
						Summary:  "alert count estimate",
						Details:  alertsDetails(uri, `up{job="foo"} == 0`, "1d", "rule comment"),
						Severity: checks.Information,
						Diagnostics: []output.Diagnostic{
							{
								Message: alertsText("prom", uri, 2, "1d"),
							},
						},
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
						formCond{key: "query", value: `count(up)`},
					},
					resp: respondWithSingleRangeVector1D(),
				},
			},
		},
		{
			description: "minCount=2 severity=bug",
			content:     "- alert: Foo Is Down\n  for: 10m\n  expr: up{job=\"foo\"} == 0\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAlertsCheck(prom, time.Hour*24, time.Minute, time.Minute*5, 2, "", checks.Bug)
			},
			prometheus: newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 3,
							Last:  3,
						},
						Reporter: "alerts/count",
						Summary:  "alert count estimate",
						Details:  alertsDetails(uri, `up{job="foo"} == 0`, "1d", ""),
						Severity: checks.Bug,
						Diagnostics: []output.Diagnostic{
							{
								Message: alertsText("prom", uri, 2, "1d"),
							},
						},
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
						formCond{key: "query", value: `count(up)`},
					},
					resp: respondWithSingleRangeVector1D(),
				},
			},
		},
		{
			description: "minCount=10",
			content:     "- alert: Foo Is Down\n  for: 10m\n  expr: up{job=\"foo\"} == 0\n",
			checker: func(prom *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAlertsCheck(prom, time.Hour*24, time.Minute, time.Minute*5, 10, "", checks.Information)
			},
			prometheus: newSimpleProm,
			problems:   noProblems,
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
						formCond{key: "query", value: `count(up)`},
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
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 3,
							Last:  3,
						},
						Reporter: "alerts/count",
						Summary:  "alert count estimate",
						Details:  alertsDetails(uri, `{__name__="up", job="foo"} == 0`, "1d", ""),
						Severity: checks.Information,
						Diagnostics: []output.Diagnostic{
							{
								Message: alertsText("prom", uri, 3, "1d"),
							},
						},
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
						formCond{key: "query", value: `count(up)`},
					},
					resp: respondWithSingleRangeVector1D(),
				},
			},
		},
		{
			description: "{__name__=~}",
			content: `
- alert: foo
  expr: '{__name__=~"(up|foo)", job="foo"} == 0'
`,
			checker:    newAlertsCheck,
			prometheus: newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 3,
							Last:  3,
						},
						Reporter: "alerts/count",
						Summary:  "alert count estimate",
						Details:  alertsDetails(uri, `{__name__=~"(up|foo)", job="foo"} == 0`, "1d", ""),
						Severity: checks.Information,
						Diagnostics: []output.Diagnostic{
							{
								Message: alertsText("prom", uri, 3, "1d"),
							},
						},
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
						formCond{key: "query", value: `count(up)`},
					},
					resp: respondWithSingleRangeVector1D(),
				},
			},
		},
		{
			description: "uptime query error",
			content:     "- alert: Foo Is Down\n  expr: up{job=\"foo\"} == 0\n",
			checker:     newAlertsCheck,
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "alerts/count",
						Summary:  "alert count estimate",
						Details:  alertsDetails(uri, `up{job="foo"} == 0`, "1d", ""),
						Severity: checks.Information,
						Diagnostics: []output.Diagnostic{
							{
								Message: alertsText("prom", uri, 3, "1d"),
							},
						},
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
						formCond{key: "query", value: `count(up)`},
					},
					resp: respondWithInternalError(),
				},
			},
		},
		{
			description: "keep_firing_for: 10m",
			content:     "- alert: Foo Is Down\n  keep_firing_for: 10m\n  expr: up{job=\"foo\"} == 0\n",
			checker:     newAlertsCheck,
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 3,
							Last:  3,
						},
						Reporter: "alerts/count",
						Summary:  "alert count estimate",
						Details:  alertsDetails(uri, `up{job="foo"} == 0`, "1d", ""),
						Severity: checks.Information,
						Diagnostics: []output.Diagnostic{
							{
								Message: alertsText("prom", uri, 2, "1d"),
							},
						},
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
						formCond{key: "query", value: `count(up)`},
					},
					resp: respondWithSingleRangeVector1D(),
				},
			},
		},
		{
			description: "for: 10m + keep_firing_for: 10m",
			content:     "- alert: Foo Is Down\n  for: 10m\n  keep_firing_for: 10m\n  expr: up{job=\"foo\"} == 0\n",
			checker:     newAlertsCheck,
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 4,
							Last:  4,
						},
						Reporter: "alerts/count",
						Summary:  "alert count estimate",
						Details:  alertsDetails(uri, `up{job="foo"} == 0`, "1d", ""),
						Severity: checks.Information,
						Diagnostics: []output.Diagnostic{
							{
								Message: alertsText("prom", uri, 1, "1d"),
							},
						},
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
						formCond{key: "query", value: `count(up)`},
					},
					resp: respondWithSingleRangeVector1D(),
				},
			},
		},
	}

	runTests(t, testCases)
}
