package checks_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
)

func textExternalLabelsRR(name, uri, k, v string) string {
	return fmt.Sprintf("`%s` Prometheus server at %s external_labels already has %s=%q label set, please choose a different name for this label to avoid any conflicts.", name, uri, k, v)
}

func textExternalLabelsAR(name, uri, k, v string) string {
	return fmt.Sprintf("This label is redundant. `%s` Prometheus server at %s external_labels already has %s=%q label set and it will be automatically added to all alerts, there's no need to set it manually.", name, uri, k, v)
}

func newLabelsConflict(prom *promapi.FailoverGroup) checks.RuleChecker {
	return checks.NewLabelsConflictCheck(prom)
}

func TestLabelsConflictCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "ignores rules with syntax errors",
			content:     "- record: foo\n  expr: sum(foo) without(\n",
			checker:     newLabelsConflict,
			prometheus:  newSimpleProm,
			problems:    noProblems,
		},
		{
			description: "ignores alerting rules",
			content:     "- alert: foo\n  expr: up == 0\n",
			checker:     newLabelsConflict,
			prometheus:  newSimpleProm,
			problems:    noProblems,
		},
		{
			description: "no labels / recording",
			content:     "- record: foo\n  expr: up == 0\n",
			checker:     newLabelsConflict,
			prometheus:  newSimpleProm,
			problems:    noProblems,
		},
		{
			description: "no labels / alerting",
			content:     "- alert: foo\n  expr: up == 0\n",
			checker:     newLabelsConflict,
			prometheus:  newSimpleProm,
			problems:    noProblems,
		},
		{
			description: "connection refused",
			content:     "- record: foo\n  expr: up == 0\n  labels:\n    foo: bar\n",
			checker:     newLabelsConflict,
			prometheus: func(_ string) *promapi.FailoverGroup {
				return simpleProm("prom", "http://127.0.0.1:1111", time.Second, false)
			},
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 3,
							Last:  4,
						},
						Reporter: checks.LabelsConflictCheckName,
						Text:     checkErrorUnableToRun(checks.LabelsConflictCheckName, "prom", "http://127.0.0.1:1111", "connection refused"),
						Severity: checks.Warning,
					},
				}
			},
		},
		{
			description: "conflict / recording",
			content:     "- record: foo\n  expr: up == 0\n  labels:\n    foo: bar\n",
			checker:     newLabelsConflict,
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 4,
							Last:  4,
						},
						Reporter: checks.LabelsConflictCheckName,
						Text:     textExternalLabelsRR("prom", uri, "foo", "bob"),
						Details:  alertsExternalLabelsDetails("prom", uri),
						Severity: checks.Warning,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  external_labels:\n    foo: bob\n"},
				},
			},
		},
		{
			description: "conflict / alerting / different",
			content:     "- alert: foo\n  expr: up == 0\n  labels:\n    foo: bar\n",
			checker:     newLabelsConflict,
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 4,
							Last:  4,
						},
						Reporter: checks.LabelsConflictCheckName,
						Text:     textExternalLabelsRR("prom", uri, "foo", "bob"),
						Details:  alertsExternalLabelsDetails("prom", uri),
						Severity: checks.Warning,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  external_labels:\n    foo: bob\n"},
				},
			},
		},
		{
			description: "conflict / alerting / identical",
			content:     "- alert: foo\n  expr: up == 0\n  labels:\n    foo: bar\n",
			checker:     newLabelsConflict,
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Lines: parser.LineRange{
							First: 4,
							Last:  4,
						},
						Reporter: checks.LabelsConflictCheckName,
						Text:     textExternalLabelsAR("prom", uri, "foo", "bar"),
						Details:  alertsExternalLabelsDetails("prom", uri),
						Severity: checks.Warning,
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  external_labels:\n    foo: bar\n"},
				},
			},
		},
		{
			description: "no conflict / recording",
			content:     "- record: foo\n  expr: up == 0\n  labels:\n    foo: bar\n",
			checker:     newLabelsConflict,
			prometheus:  newSimpleProm,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  external_labels:\n    bob: bob\n"},
				},
			},
		},
		{
			description: "no conflict / alerting",
			content:     "- alert: foo\n  expr: up == 0\n  labels:\n    foo: bar\n",
			checker:     newLabelsConflict,
			prometheus:  newSimpleProm,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  external_labels:\n    bob: bob\n"},
				},
			},
		},
		{
			description: "flags unsupported",
			content:     "- record: foo\n  expr: up == 0\n  labels:\n    foo: bar\n",
			checker:     newLabelsConflict,
			prometheus:  newSimpleProm,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  httpResponse{code: http.StatusNotFound, body: "Not Found"},
				},
			},
		},
	}

	runTests(t, testCases)
}
