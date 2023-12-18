package checks_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
)

func textExternalLabels(name, uri, k, v string) string {
	return fmt.Sprintf("`%s` Prometheus server at %s external_labels already has %s=%q label set, please choose a different name for this label to avoid any conflicts.", name, uri, k, v)
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
			description: "no labels",
			content:     "- record: foo\n  expr: up == 0\n",
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
			problems: func(uri string) []checks.Problem {
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
			description: "conflict",
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
						Text:     textExternalLabels("prom", uri, "foo", "bob"),
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
			description: "no conflict",
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
	}

	runTests(t, testCases)
}
