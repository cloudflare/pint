package checks_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/promapi"
)

func newAlertsAbsentCheck(prom *promapi.FailoverGroup) checks.RuleChecker {
	return checks.NewAlertsAbsentCheck(prom)
}

func TestAlertsAbsentCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "ignores recording rules",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker:     newAlertsAbsentCheck,
			prometheus:  newSimpleProm,
		},
		{
			description: "ignores rules with syntax errors",
			content:     "- alert: foo\n  expr: sum(foo) without(\n",
			checker:     newAlertsAbsentCheck,
			prometheus:  newSimpleProm,
		},
		{
			description: "ignores rules with no absent()",
			content:     "- alert: foo\n  expr: count(foo)\n  for: 2m\n",
			checker:     newAlertsAbsentCheck,
			prometheus:  newSimpleProm,
		},
		{
			description: "ignores rules with invalid duration",
			content:     "- alert: foo\n  expr: absent(foo)\n  for: abc\n",
			checker:     newAlertsAbsentCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
			},
		},
		{
			description: "count() or absent() without for",
			content:     "- alert: foo\n  expr: count(foo) > 5 or absent(foo)\n",
			checker:     newAlertsAbsentCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
			},
			problems: true,
		},
		{
			description: "absent() without for",
			content:     "- alert: foo\n  expr: absent(foo)\n",
			checker:     newAlertsAbsentCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
			},
			problems: true,
		},
		{
			description: "absent() < 2x scrape_interval",
			content:     "- alert: foo\n  expr: absent(foo)\n  for: 1m\n",
			checker:     newAlertsAbsentCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
			},
			problems: true,
		},
		{
			description: "absent() < 2x scrape_interval, 53s",
			content:     "- alert: foo\n  expr: absent(foo)\n  for: 1m\n",
			checker:     newAlertsAbsentCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 53s\n"},
				},
			},
			problems: true,
		},
		{
			description: "absent() < 2x scrape_interval, no config",
			content:     "- alert: foo\n  expr: absent(foo)\n  for: 30s\n",
			checker:     newAlertsAbsentCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "{}"},
				},
			},
			problems: true,
		},
		{
			description: "absent() == 2x scrape_interval",
			content:     "- alert: foo\n  expr: absent(foo)\n  for: 2m\n",
			checker:     newAlertsAbsentCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
			},
		},
		{
			description: "invalid status",
			content:     "- alert: foo\n  expr: absent(foo)\n",
			checker:     newAlertsAbsentCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  respondWithBadData(),
				},
			},
			problems: true,
		},
		{
			description: "invalid YAML",
			content:     "- alert: foo\n  expr: absent(foo)\n",
			checker:     newAlertsAbsentCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:::\nglobal:{}{}{}\n"},
				},
			},
			problems: true,
		},
		{
			description: "connection refused",
			content:     "- alert: foo\n  expr: absent(foo)\n",
			checker:     newAlertsAbsentCheck,
			prometheus: func(_ string) *promapi.FailoverGroup {
				return simpleProm("prom", "http://127.0.0.1:1111", time.Second, true)
			},
			problems: true,
		},
		{
			description: "404",
			content:     "- alert: foo\n  expr: absent(foo)\n",
			checker:     newAlertsAbsentCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  httpResponse{code: http.StatusNotFound, body: "Not Found"},
				},
			},
		},
		{
			description: "600",
			content:     "- alert: foo\n  expr: absent(foo)\n",
			checker:     newAlertsAbsentCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  httpResponse{code: 600, body: "Bogus error code"},
				},
			},
			problems: true,
		},
	}
	runTests(t, testCases)
}
