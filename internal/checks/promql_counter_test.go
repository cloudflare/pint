package checks_test

import (
	"fmt"
	"testing"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
)

func newCounterCheck(prom *promapi.FailoverGroup) checks.RuleChecker {
	return checks.NewCounterCheck(prom)
}

func counterText(name, uri, metric string) string {
	return fmt.Sprintf("`%s` is a counter according to metrics metadata from `%s` Prometheus server at %s, it can be dangarous to use its value directly.", metric, name, uri)
}

func TestCounterCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "ignores rules with syntax errors",
			content:     "- record: foo\n  expr: sum(foo) without(\n",
			checker:     newCounterCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
		},
		{
			description: "500 error from Prometheus API",
			content:     "- record: foo\n  expr: http_requests_total > 1\n",
			checker:     newCounterCheck,
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "promql/counter",
						Text:     checkErrorUnableToRun(checks.CounterCheckName, "prom", uri, "server_error: internal error"),
						Severity: checks.Bug,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireMetadataPath},
					resp:  respondWithInternalError(),
				},
			},
		},
		{
			description: "invalid status",
			content:     "- record: foo\n  expr: http_requests_total > 1\n",
			checker:     newCounterCheck,
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "promql/counter",
						Text:     checkErrorBadData("prom", uri, "bad_data: bad input data"),
						Severity: checks.Warning,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireMetadataPath},
					resp:  respondWithBadData(),
				},
			},
		},
		{
			description: "counter",
			content:     "- alert: my alert\n  expr: http_requests_total\n",
			checker:     newCounterCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
		},
		{
			description: "rate(counter) > 1",
			content:     "- alert: my alert\n  expr: rate(http_requests_total[2m]) > 1\n",
			checker:     newCounterCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
		},
		{
			description: "absent(counter)",
			content:     "- alert: my alert\n  expr: absent(http_requests_total) > 0\n",
			checker:     newCounterCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
		},
		{
			description: "absent_over_time(counter)",
			content:     "- alert: my alert\n  expr: absent_over_time(http_requests_total[2m]) > 0\n",
			checker:     newCounterCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
		},
		{
			description: "present_over_time(counter)",
			content:     "- alert: my alert\n  expr: present_over_time(http_requests_total[2m]) > 0\n",
			checker:     newCounterCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
		},
		{
			description: "changes(counter)",
			content:     "- alert: my alert\n  expr: changes(http_requests_total[2m]) > 0\n",
			checker:     newCounterCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
		},
		{
			description: "resets(counter)",
			content:     "- alert: my alert\n  expr: resets(http_requests_total[2m]) > 0\n",
			checker:     newCounterCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
		},
		{
			description: "count(counter)",
			content:     "- alert: my alert\n  expr: count(http_requests_total) > 0\n",
			checker:     newCounterCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
		},
		{
			description: "group(counter)",
			content:     "- alert: my alert\n  expr: group(http_requests_total) > 0\n",
			checker:     newCounterCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
		},
		{
			description: "count_over_time(counter)",
			content:     "- alert: my alert\n  expr: count_over_time(http_requests_total[2m]) > 0\n",
			checker:     newCounterCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
		},
		{
			description: "increase(counter)",
			content:     "- alert: my alert\n  expr: increase(http_requests_total[2m]) > 0\n",
			checker:     newCounterCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
		},
		{
			description: "timestamp(counter)",
			content:     "- alert: my alert\n  expr: time() - timestamp(http_requests_total) > 1\n",
			checker:     newCounterCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
		},
		{
			description: "sum(rate(counter)) > 1",
			content:     "- alert: my alert\n  expr: sum(rate(http_requests_total[2m])) > 1\n",
			checker:     newCounterCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
		},
		{
			description: "ok unless counter",
			content: `
- alert: my alert
  expr: absent(http_requests_total{cluster="dev"}) unless http_requests_total{cluster="prod"}
`,
			checker:    newCounterCheck,
			prometheus: newSimpleProm,
			problems:   noProblems,
		},
		{
			description: "counter > 1 unless ok",
			content: `
- alert: my alert
  expr:  http_requests_total{cluster="prod"} > 1 unless absent(http_requests_total{cluster="dev"})
`,
			checker:    newCounterCheck,
			prometheus: newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 3,
							Last:  3,
						},
						Reporter: "promql/counter",
						Text:     counterText("prom", uri, "http_requests_total"),
						Details:  checks.CounterCheckDetails,
						Severity: checks.Warning,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireMetadataPath},
					resp: metadataResponse{metadata: map[string][]v1.Metadata{
						"http_requests_total": {{Type: "counter"}},
					}},
				},
			},
		},
		{
			description: "counter > 1",
			content:     "- alert: my alert\n  expr: http_requests_total > 1\n",
			checker:     newCounterCheck,
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "promql/counter",
						Text:     counterText("prom", uri, "http_requests_total"),
						Details:  checks.CounterCheckDetails,
						Severity: checks.Warning,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireMetadataPath},
					resp: metadataResponse{metadata: map[string][]v1.Metadata{
						"http_requests_total": {{Type: "counter"}},
					}},
				},
			},
		},
		{
			description: "counter == 1 and counter > 2 or counter < 3",
			content: `
- alert: my alert
  expr: http_requests_total == 1 and http_requests_total > 2 or http_requests_total < 3`,
			checker:    newCounterCheck,
			prometheus: newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 3,
							Last:  3,
						},
						Reporter: "promql/counter",
						Text:     counterText("prom", uri, "http_requests_total"),
						Details:  checks.CounterCheckDetails,
						Severity: checks.Warning,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireMetadataPath},
					resp: metadataResponse{metadata: map[string][]v1.Metadata{
						"http_requests_total": {{Type: "counter"}},
					}},
				},
			},
		},
		{
			description: "sum(counter) > 1",
			content:     "- alert: my alert\n  expr: sum(http_requests_total) > 1\n",
			checker:     newCounterCheck,
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "promql/counter",
						Text:     counterText("prom", uri, "http_requests_total"),
						Details:  checks.CounterCheckDetails,
						Severity: checks.Warning,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireMetadataPath},
					resp: metadataResponse{metadata: map[string][]v1.Metadata{
						"http_requests_total": {{Type: "counter"}},
					}},
				},
			},
		},
		{
			description: "delta(counter[2m]) > 1",
			content:     "- alert: my alert\n  expr: delta(http_requests_total[2m]) > 1\n",
			checker:     newCounterCheck,
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "promql/counter",
						Text:     counterText("prom", uri, "http_requests_total"),
						Details:  checks.CounterCheckDetails,
						Severity: checks.Warning,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireMetadataPath},
					resp: metadataResponse{metadata: map[string][]v1.Metadata{
						"http_requests_total": {{Type: "counter"}},
					}},
				},
			},
		},
		{
			description: "sum(counter) > 1",
			content:     "- alert: my alert\n  expr: sum(http_requests_total) > 1\n",
			checker:     newCounterCheck,
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "promql/counter",
						Text:     counterText("prom", uri, "http_requests_total"),
						Details:  checks.CounterCheckDetails,
						Severity: checks.Warning,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireMetadataPath},
					resp: metadataResponse{metadata: map[string][]v1.Metadata{
						"http_requests_total": {{Type: "counter"}},
					}},
				},
			},
		},
		{
			description: "counter > 1 / no metadata",
			content: `
- alert: my alert
  expr:  http_requests_total{cluster="prod"} > 1
`,
			checker:    newCounterCheck,
			prometheus: newSimpleProm,
			problems:   noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireMetadataPath},
					resp:  metadataResponse{metadata: map[string][]v1.Metadata{}},
				},
			},
		},
		{
			description: "counter > 1 / mixed metadata",
			content: `
- alert: my alert
  expr:  http_requests_total{cluster="prod"} > 1
`,
			checker:    newCounterCheck,
			prometheus: newSimpleProm,
			problems:   noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireMetadataPath},
					resp: metadataResponse{metadata: map[string][]v1.Metadata{
						"http_requests_total": {{Type: "counter"}, {Type: "gauge"}, {Type: "counter"}},
					}},
				},
			},
		},
	}
	runTests(t, testCases)
}
