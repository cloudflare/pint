package checks_test

import (
	"net/http"
	"testing"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/promapi"
)

func newOffsetCheck(prom *promapi.FailoverGroup) checks.RuleChecker {
	return checks.NewOffsetCheck(prom)
}

func TestOffsetCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "ignores rules with syntax errors",
			content:     "- record: foo\n  expr: sum(foo) without(\n",
			checker:     newOffsetCheck,
			prometheus:  newSimpleProm,
		},
		{
			description: "flag query error",
			content:     "- record: foo\n  expr: foo offset 30d\n",
			checker:     newOffsetCheck,
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
			content:     "- record: foo\n  expr: foo offset 30d\n",
			checker:     newOffsetCheck,
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
			content:     "- record: foo\n  expr: foo offset 30d\n",
			checker:     newOffsetCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp:  httpResponse{code: http.StatusNotFound, body: "Not Found"},
				},
			},
		},
		{
			// default retention is 15d, offset 10d is within bounds
			description: "flag not set, 10d offset",
			content:     "- record: foo\n  expr: foo offset 10d\n",
			checker:     newOffsetCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp:  flagsResponse{flags: map[string]string{}},
				},
			},
		},
		{
			// default retention is 15d, offset 20d exceeds it
			description: "flag not set, 20d offset",
			content:     "- record: foo\n  expr: foo offset 20d\n",
			checker:     newOffsetCheck,
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
			description: "retention 11d, offset 10d",
			content:     "- record: foo\n  expr: foo offset 10d\n",
			checker:     newOffsetCheck,
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
			description: "retention 11d, offset 11d1h",
			content:     "- record: foo\n  expr: foo offset 11d1h\n",
			checker:     newOffsetCheck,
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
			// retention flags are 0s which means default 15d kicks in, offset 20d exceeds it
			description: "flag with 0s, 20d offset",
			content:     "- record: foo\n  expr: foo offset 20d\n",
			checker:     newOffsetCheck,
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
			description: "flag with 0s, 10d offset",
			content:     "- record: foo\n  expr: foo offset 10d\n",
			checker:     newOffsetCheck,
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
			// only the second selector has an offset that exceeds retention
			description: "binary expr with one offset exceeding retention",
			content:     "- record: foo\n  expr: foo + foo offset 20d\n",
			checker:     newOffsetCheck,
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
			description: "no offset used",
			content:     "- record: foo\n  expr: rate(foo[5m])\n",
			checker:     newOffsetCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp:  flagsResponse{flags: map[string]string{}},
				},
			},
		},
		{
			// subquery with offset exceeding default retention
			description: "subquery offset exceeding retention",
			content:     "- record: foo\n  expr: max_over_time(foo[1h:5m] offset 20d)\n",
			checker:     newOffsetCheck,
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
			description: "subquery offset within retention",
			content:     "- record: foo\n  expr: max_over_time(foo[1h:5m] offset 10d)\n",
			checker:     newOffsetCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp:  flagsResponse{flags: map[string]string{}},
				},
			},
		},
		{
			// metric name contains "offset" — highlight must point at the keyword, not the metric name
			description: "metric name containing offset",
			content:     "- record: foo\n  expr: foo_offset_offset offset 20d\n",
			checker:     newOffsetCheck,
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
			// offset on a matrix selector (the offset is actually on the inner VectorSelector)
			description: "matrix selector with offset exceeding retention",
			content:     "- record: foo\n  expr: rate(foo[5m] offset 20d)\n",
			checker:     newOffsetCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp:  flagsResponse{flags: map[string]string{}},
				},
			},
		},
	}
	runTests(t, testCases)
}
