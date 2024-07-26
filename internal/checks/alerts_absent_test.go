package checks_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
)

func newAlertsAbsentCheck(prom *promapi.FailoverGroup) checks.RuleChecker {
	return checks.NewAlertsAbsentCheck(prom)
}

func absentForNeeded(prom, uri, d string) string {
	return fmt.Sprintf("Alert query is using absent() which might cause false positives when `%s` Prometheus server at %s restarts, please add `for: %s` to avoid this.", prom, uri, d)
}

func TestAlertsAbsentCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "ignores recording rules",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker:     newAlertsAbsentCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
		},
		{
			description: "ignores rules with syntax errors",
			content:     "- alert: foo\n  expr: sum(foo) without(\n",
			checker:     newAlertsAbsentCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
		},
		{
			description: "ignores rules with invalid duration",
			content:     "- alert: foo\n  expr: absent(foo)\n  for: abc\n",
			checker:     newAlertsAbsentCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
			},
		},
		{
			description: "absent() without for",
			content:     "- alert: foo\n  expr: absent(foo)\n",
			checker:     newAlertsAbsentCheck,
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: checks.AlertsAbsentCheckName,
						Text:     absentForNeeded("prom", uri, "2m"),
						Details:  checks.AlertsAbsentCheckDetails,
						Severity: checks.Warning,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
			},
		},
		{
			description: "absent() < 2x scrape_interval",
			content:     "- alert: foo\n  expr: absent(foo)\n  for: 1m\n",
			checker:     newAlertsAbsentCheck,
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: checks.AlertsAbsentCheckName,
						Text:     absentForNeeded("prom", uri, "2m"),
						Details:  checks.AlertsAbsentCheckDetails,
						Severity: checks.Warning,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 1m\n"},
				},
			},
		},
		{
			description: "absent() < 2x scrape_interval, 53s",
			content:     "- alert: foo\n  expr: absent(foo)\n  for: 1m\n",
			checker:     newAlertsAbsentCheck,
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: checks.AlertsAbsentCheckName,
						Text:     absentForNeeded("prom", uri, "2m"),
						Details:  checks.AlertsAbsentCheckDetails,
						Severity: checks.Warning,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  scrape_interval: 53s\n"},
				},
			},
		},
		{
			description: "absent() < 2x scrape_interval, no config",
			content:     "- alert: foo\n  expr: absent(foo)\n  for: 30s\n",
			checker:     newAlertsAbsentCheck,
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: checks.AlertsAbsentCheckName,
						Text:     absentForNeeded("prom", uri, "2m"),
						Details:  checks.AlertsAbsentCheckDetails,
						Severity: checks.Warning,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "{}"},
				},
			},
		},
		{
			description: "absent() == 2x scrape_interval",
			content:     "- alert: foo\n  expr: absent(foo)\n  for: 2m\n",
			checker:     newAlertsAbsentCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
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
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: checks.AlertsAbsentCheckName,
						Text:     checkErrorBadData("prom", uri, "bad_data: bad input data"),
						Severity: checks.Warning,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  respondWithBadData(),
				},
			},
		},
		{
			description: "invalid YAML",
			content:     "- alert: foo\n  expr: absent(foo)\n",
			checker:     newAlertsAbsentCheck,
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: checks.AlertsAbsentCheckName,
						Text:     checkErrorUnableToRun(checks.AlertsAbsentCheckName, "prom", uri, fmt.Sprintf("failed to decode config data in %s response: yaml: line 2: could not find expected ':'", uri)),
						Severity: checks.Bug,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:::\nglobal:{}{}{}\n"},
				},
			},
		},
		{
			description: "connection refused",
			content:     "- alert: foo\n  expr: absent(foo)\n",
			checker:     newAlertsAbsentCheck,
			prometheus: func(_ string) *promapi.FailoverGroup {
				return simpleProm("prom", "http://127.0.0.1:1111", time.Second, true)
			},
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: checks.AlertsAbsentCheckName,
						Text:     checkErrorUnableToRun(checks.AlertsAbsentCheckName, "prom", "http://127.0.0.1:1111", "connection refused"),
						Severity: checks.Bug,
					},
				}
			},
		},
	}
	runTests(t, testCases)
}
