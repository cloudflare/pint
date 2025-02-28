package checks_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/parser"
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
			description: "ignores rules with no absent()",
			content:     "- alert: foo\n  expr: count(foo)\n  for: 2m\n",
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
			description: "count() or absent() without for",
			content:     "- alert: foo\n  expr: count(foo) > 5 or absent(foo)\n",
			checker:     newAlertsAbsentCheck,
			prometheus:  newSimpleProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: checks.AlertsAbsentCheckName,
						Summary:  "absent() based alert without for",
						Details:  checks.AlertsAbsentCheckDetails,
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Message: "Using `absent()` might cause false positive alerts when Prometheus restarts.",
							},
						},
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
			description: "absent() without for",
			content:     "- alert: foo\n  expr: absent(foo)\n",
			checker:     newAlertsAbsentCheck,
			prometheus:  newSimpleProm,
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: checks.AlertsAbsentCheckName,
						Summary:  "absent() based alert without for",
						Details:  checks.AlertsAbsentCheckDetails,
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Message: "Using `absent()` might cause false positive alerts when Prometheus restarts.",
							},
						},
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
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: checks.AlertsAbsentCheckName,
						Summary:  "absent() based alert with insufficient for",
						Details:  checks.AlertsAbsentCheckDetails,
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Message: "Using `absent()` might cause false positive alerts when Prometheus restarts.",
							},
							{
								Message: "Use a value that's at least twice Prometheus scrape interval (`1m`).",
							},
						},
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
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: checks.AlertsAbsentCheckName,
						Summary:  "absent() based alert with insufficient for",
						Details:  checks.AlertsAbsentCheckDetails,
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Message: "Using `absent()` might cause false positive alerts when Prometheus restarts.",
							},
							{
								Message: "Use a value that's at least twice Prometheus scrape interval (`53s`).",
							},
						},
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
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: checks.AlertsAbsentCheckName,
						Summary:  "absent() based alert with insufficient for",
						Details:  checks.AlertsAbsentCheckDetails,
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Message: "Using `absent()` might cause false positive alerts when Prometheus restarts.",
							},
							{
								Message: "Use a value that's at least twice Prometheus scrape interval (`1m`).",
							},
						},
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
						Summary:  "unable to run checks",
						Severity: checks.Warning,
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
						Summary:  "unable to run checks",
						Severity: checks.Bug,
						Diagnostics: []output.Diagnostic{
							{
								Message: checkErrorUnableToRun(checks.AlertsAbsentCheckName, "prom", uri, fmt.Sprintf("failed to decode config data in %s response: yaml: line 2: could not find expected ':'", uri)),
							},
						},
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
						Summary:  "unable to run checks",
						Severity: checks.Bug,
						Diagnostics: []output.Diagnostic{
							{
								Message: checkErrorUnableToRun(checks.AlertsAbsentCheckName, "prom", "http://127.0.0.1:1111", "connection refused"),
							},
						},
					},
				}
			},
		},
		{
			description: "404",
			content:     "- alert: foo\n  expr: absent(foo)\n",
			checker:     newAlertsAbsentCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
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
			problems: func(s string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: checks.AlertsAbsentCheckName,
						Summary:  "unable to run checks",
						Severity: checks.Warning,
						Diagnostics: []output.Diagnostic{
							{
								Message: checkErrorBadData("prom", s, "bad_response: 600 status code 600"),
							},
						},
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  httpResponse{code: 600, body: "Bogus error code"},
				},
			},
		},
	}
	runTests(t, testCases)
}
