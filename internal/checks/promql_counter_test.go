package checks_test

import (
	"testing"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/promapi"
)

func newCounterCheck(prom *promapi.FailoverGroup) checks.RuleChecker {
	return checks.NewCounterCheck(prom)
}

func TestCounterCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "ignores rules with syntax errors",
			content:     "- record: foo\n  expr: sum(foo) without(\n",
			checker:     newCounterCheck,
			prometheus:  newSimpleProm,
		},
		{
			description: "500 error from Prometheus API",
			content:     "- record: foo\n  expr: http_requests_total > 1\n",
			checker:     newCounterCheck,
			prometheus:  newSimpleProm,
			problems:    true,
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
			problems:    true,
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
		},
		{
			description: "rate(counter) > 1",
			content:     "- alert: my alert\n  expr: rate(http_requests_total[2m]) > 1\n",
			checker:     newCounterCheck,
			prometheus:  newSimpleProm,
		},
		{
			description: "absent(counter)",
			content:     "- alert: my alert\n  expr: absent(http_requests_total) > 0\n",
			checker:     newCounterCheck,
			prometheus:  newSimpleProm,
		},
		{
			description: "absent_over_time(counter)",
			content:     "- alert: my alert\n  expr: absent_over_time(http_requests_total[2m]) > 0\n",
			checker:     newCounterCheck,
			prometheus:  newSimpleProm,
		},
		{
			description: "present_over_time(counter)",
			content:     "- alert: my alert\n  expr: present_over_time(http_requests_total[2m]) > 0\n",
			checker:     newCounterCheck,
			prometheus:  newSimpleProm,
		},
		{
			description: "changes(counter)",
			content:     "- alert: my alert\n  expr: changes(http_requests_total[2m]) > 0\n",
			checker:     newCounterCheck,
			prometheus:  newSimpleProm,
		},
		{
			description: "resets(counter)",
			content:     "- alert: my alert\n  expr: resets(http_requests_total[2m]) > 0\n",
			checker:     newCounterCheck,
			prometheus:  newSimpleProm,
		},
		{
			description: "count(counter)",
			content:     "- alert: my alert\n  expr: count(http_requests_total) > 0\n",
			checker:     newCounterCheck,
			prometheus:  newSimpleProm,
		},
		{
			description: "group(counter)",
			content:     "- alert: my alert\n  expr: group(http_requests_total) > 0\n",
			checker:     newCounterCheck,
			prometheus:  newSimpleProm,
		},
		{
			description: "count_over_time(counter)",
			content:     "- alert: my alert\n  expr: count_over_time(http_requests_total[2m]) > 0\n",
			checker:     newCounterCheck,
			prometheus:  newSimpleProm,
		},
		{
			description: "increase(counter)",
			content:     "- alert: my alert\n  expr: increase(http_requests_total[2m]) > 0\n",
			checker:     newCounterCheck,
			prometheus:  newSimpleProm,
		},
		{
			description: "timestamp(counter)",
			content:     "- alert: my alert\n  expr: time() - timestamp(http_requests_total) > 1\n",
			checker:     newCounterCheck,
			prometheus:  newSimpleProm,
		},
		{
			description: "sum(rate(counter)) > 1",
			content:     "- alert: my alert\n  expr: sum(rate(http_requests_total[2m])) > 1\n",
			checker:     newCounterCheck,
			prometheus:  newSimpleProm,
		},
		{
			description: "ok unless counter",
			content: `
- alert: my alert
  expr: absent(http_requests_total{cluster="dev"}) unless http_requests_total{cluster="prod"}
`,
			checker:    newCounterCheck,
			prometheus: newSimpleProm,
		},
		{
			description: "counter > 1 unless ok",
			content: `
- alert: my alert
  expr:  http_requests_total{cluster="prod"} > 1 unless absent(http_requests_total{cluster="dev"})
`,
			checker:    newCounterCheck,
			prometheus: newSimpleProm,
			problems:   true,
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
			problems:    true,
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
			problems:   true,
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
			problems:    true,
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
			problems:    true,
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
			problems:    true,
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
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireMetadataPath},
					resp: metadataResponse{metadata: map[string][]v1.Metadata{
						"http_requests_total": {
							{Type: "counter"},
							{Type: "gauge"},
							{Type: "counter"},
						},
					}},
				},
			},
		},
	}
	runTests(t, testCases)
}
