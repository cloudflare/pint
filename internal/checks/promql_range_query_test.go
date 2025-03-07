package checks_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/promapi"
)

func newRangeQueryCheck(prom *promapi.FailoverGroup) checks.RuleChecker {
	return checks.NewRangeQueryCheck(prom, 0, "", checks.Fatal)
}

func newRangeQueryCheckWithLimit(prom *promapi.FailoverGroup) checks.RuleChecker {
	return checks.NewRangeQueryCheck(prom, time.Hour*4, "some text", checks.Bug)
}

func retentionToLow(name, uri, metric, qr, retention string) string {
	return fmt.Sprintf("`%s` selector is trying to query Prometheus for %s worth of metrics, but `%s` Prometheus server at %s is configured to only keep %s of metrics history.",
		metric, qr, name, uri, retention)
}

func TestRangeQueryCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "ignores rules with syntax errors",
			content:     "- record: foo\n  expr: sum(foo) without(\n",
			checker:     newRangeQueryCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
		},
		{
			description: "flag query error",
			content:     "- record: foo\n  expr: rate(foo[30d])\n",
			checker:     newRangeQueryCheck,
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Reporter: "promql/range_query",
						Summary:  "unable to run checks",
						Severity: checks.Bug,
						Diagnostics: []output.Diagnostic{
							{
								Message: checkErrorUnableToRun(checks.RangeQueryCheckName, "prom", uri, "server_error: internal error"),
							},
						},
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp:  respondWithInternalError(),
				},
			},
		},
		{
			description: "flag parse error",
			content:     "- record: foo\n  expr: rate(foo[30d])\n",
			checker:     newRangeQueryCheck,
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Reporter: "promql/range_query",
						Summary:  "unable to run checks",
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Message: `Cannot parse --storage.tsdb.retention.time="abc" flag value: not a valid duration string: "abc"`,
							},
						},
					},
					{
						Reporter: "promql/range_query",
						Summary:  "query beyond configured retention",
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Message: retentionToLow("prom", uri, "foo[30d]", "30d", "15d"),
							},
						},
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp: flagsResponse{flags: map[string]string{
						"storage.tsdb.retention.time": "abc",
					}},
				},
			},
		},
		{
			description: "flag unsupported",
			content:     "- record: foo\n  expr: rate(foo[30d])\n",
			checker:     newRangeQueryCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp:  httpResponse{code: http.StatusNotFound, body: "Not Found"},
				},
			},
		},
		{
			description: "flag not set, 10d",
			content:     "- record: foo\n  expr: rate(foo[10d])\n",
			checker:     newRangeQueryCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp:  flagsResponse{flags: map[string]string{}},
				},
			},
		},
		{
			description: "flag not set, 20d",
			content:     "- record: foo\n  expr: rate(foo[20d])\n",
			checker:     newRangeQueryCheck,
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Reporter: "promql/range_query",
						Summary:  "query beyond configured retention",
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Message: retentionToLow("prom", uri, "foo[20d]", "20d", "15d"),
							},
						},
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp:  flagsResponse{flags: map[string]string{}},
				},
			},
		},
		{
			description: "flag set to 11d, 10d",
			content:     "- record: foo\n  expr: rate(foo[10d])\n",
			checker:     newRangeQueryCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp: flagsResponse{flags: map[string]string{
						"storage.tsdb.retention.time": "11d",
					}},
				},
			},
		},
		{
			description: "flag set to 11d, 11d1h",
			content:     "- record: foo\n  expr: rate(foo[11d1h])\n",
			checker:     newRangeQueryCheck,
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Reporter: "promql/range_query",
						Summary:  "query beyond configured retention",
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Message: retentionToLow("prom", uri, "foo[11d1h]", "11d1h", "11d"),
							},
						},
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp: flagsResponse{flags: map[string]string{
						"storage.tsdb.retention.time": "11d",
					}},
				},
			},
		},
		{
			description: "flag with 0s, 20d",
			content:     "- record: foo\n  expr: rate(foo[20d])\n",
			checker:     newRangeQueryCheck,
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Reporter: "promql/range_query",
						Summary:  "query beyond configured retention",
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Message: retentionToLow("prom", uri, "foo[20d]", "20d", "15d"),
							},
						},
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp: flagsResponse{flags: map[string]string{
						"storage.tsdb.retention":      "0s",
						"storage.tsdb.retention.size": "0B",
						"storage.tsdb.retention.time": "0s",
					}},
				},
			},
		},
		{
			description: "flag with 0s, 10d",
			content:     "- record: foo\n  expr: rate(foo[10d])\n",
			checker:     newRangeQueryCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp: flagsResponse{flags: map[string]string{
						"storage.tsdb.retention":      "0s",
						"storage.tsdb.retention.size": "0B",
						"storage.tsdb.retention.time": "0s",
					}},
				},
			},
		},
		{
			description: "limit / 3h",
			content:     "- record: foo\n  expr: rate(foo[3h])\n",
			checker:     newRangeQueryCheckWithLimit,
			prometheus:  noProm,
			problems:    noProblems,
		},
		{
			description: "limit / 5h",
			content:     "- record: foo\n  expr: rate(foo[5h])\n",
			checker:     newRangeQueryCheckWithLimit,
			prometheus:  noProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Reporter: "promql/range_query",
						Summary:  "query beyond configured retention",
						Details:  "Rule comment: some text",
						Severity: checks.Bug,
						Diagnostics: []output.Diagnostic{
							{
								Message: "`foo[5h]` selector is trying to query Prometheus for 5h worth of metrics, but 4h is the maximum allowed range query.",
							},
						},
					},
				}
			},
		},
	}
	runTests(t, testCases)
}
