package checks_test

import (
	"fmt"
	"testing"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/promapi"
)

func newRangeQueryCheck(prom *promapi.FailoverGroup) checks.RuleChecker {
	return checks.NewRangeQueryCheck(prom)
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
						Lines:    []int{2},
						Reporter: "promql/range_query",
						Text:     checkErrorUnableToRun(checks.RangeQueryCheckName, "prom", uri, "server_error: internal error"),
						Severity: checks.Bug,
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
						Lines:    []int{2},
						Reporter: "promql/range_query",
						Text:     `Cannot parse --storage.tsdb.retention.time="abc" flag value: not a valid duration string: "abc"`,
						Severity: checks.Warning,
					},
					{
						Lines:    []int{2},
						Reporter: "promql/range_query",
						Text:     retentionToLow("prom", uri, "foo[30d]", "30d", "15d"),
						Severity: checks.Warning,
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
						Lines:    []int{2},
						Reporter: "promql/range_query",
						Text:     retentionToLow("prom", uri, "foo[20d]", "20d", "15d"),
						Severity: checks.Warning,
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
						Lines:    []int{2},
						Reporter: "promql/range_query",
						Text:     retentionToLow("prom", uri, "foo[11d1h]", "11d1h", "11d"),
						Severity: checks.Warning,
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
	}
	runTests(t, testCases)
}
