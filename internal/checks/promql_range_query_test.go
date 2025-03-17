package checks_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/promapi"
)

func newRangeQueryCheck(prom *promapi.FailoverGroup) checks.RuleChecker {
	return checks.NewRangeQueryCheck(prom, 0, "", checks.Fatal)
}

func newRangeQueryCheckWithLimit(prom *promapi.FailoverGroup) checks.RuleChecker {
	return checks.NewRangeQueryCheck(prom, time.Hour*4, "some text", checks.Bug)
}

func TestRangeQueryCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "ignores rules with syntax errors",
			content:     "- record: foo\n  expr: sum(foo) without(\n",
			checker:     newRangeQueryCheck,
			prometheus:  newSimpleProm,
		},
		{
			description: "flag query error",
			content:     "- record: foo\n  expr: rate(foo[30d])\n",
			checker:     newRangeQueryCheck,
			prometheus:  newSimpleProm,
			problems:    true,
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
			problems:    true,
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
			problems:    true,
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
			problems:    true,
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
			problems:    true,
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
		},
		{
			description: "limit / 5h",
			content:     "- record: foo\n  expr: rate(foo[5h])\n",
			checker:     newRangeQueryCheckWithLimit,
			prometheus:  noProm,
			problems:    true,
		},
	}
	runTests(t, testCases)
}
