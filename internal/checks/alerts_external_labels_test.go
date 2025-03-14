package checks_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/promapi"
)

func newAlertsExternalLabelsCheck(prom *promapi.FailoverGroup) checks.RuleChecker {
	return checks.NewAlertsExternalLabelsCheck(prom)
}

func alertsExternalLabelsText(name, uri, label string) string {
	return fmt.Sprintf("Template is using `%s` external label but `%s` Prometheus server at %s doesn't have this label configured in global:external_labels.", label, name, uri)
}

func alertsExternalLabelsDetails(name, uri string) string {
	return fmt.Sprintf("[Click here](%s/config) to see `%s` Prometheus runtime configuration.", uri, name)
}

func TestAlertsExternalLabelsCountCheck(t *testing.T) {
	content := `
- alert: Foo Is Down
  expr: up{job="foo"} == 0
  annotations:
    summary: "{{ $labels.job }} is down"
    cluster: "This is {{ .ExternalLabels.cluster }} cluster"
  labels:
    job: "{{ $labels.job }}"
    twice: "{{ $externalLabels.cluster }} / {{ $externalLabels.cluster }}"
    cluster: "{{ $externalLabels.cluster }}"
`

	testCases := []checkTest{
		{
			description: "ignores recording rules",
			content:     "- record: foo\n  expr: up == 0\n",
			checker:     newAlertsExternalLabelsCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
		},
		{
			description: "ignores rules with syntax errors",
			content:     "- alert: Foo Is Down\n  expr: sum(\n",
			checker:     newAlertsExternalLabelsCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
		},
		{
			description: "bad request",
			content:     content,
			checker:     newAlertsExternalLabelsCheck,
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Reporter: checks.AlertsExternalLabelsCheckName,
						Summary:  "unable to run checks",
						Severity: checks.Bug,
						Diagnostics: []diags.Diagnostic{
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
			description: "connection refused / upstream not required / warning",
			content:     content,
			checker:     newAlertsExternalLabelsCheck,
			prometheus: func(_ string) *promapi.FailoverGroup {
				return simpleProm("prom", "http://127.0.0.1:1111", time.Second*5, false)
			},
			problems: func(_ string) []checks.Problem {
				return []checks.Problem{
					{
						Reporter: checks.AlertsExternalLabelsCheckName,
						Summary:  "unable to run checks",
						Severity: checks.Warning,
						Diagnostics: []diags.Diagnostic{
							{
								Message: checkErrorUnableToRun("prom", "http://127.0.0.1:1111", `connection refused`),
							},
						},
					},
				}
			},
		},
		{
			description: "all labels present",
			content:     content,
			checker:     newAlertsExternalLabelsCheck,
			prometheus:  newSimpleProm,
			problems:    noProblems,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  external_labels:\n    cluster: foo\n"},
				},
			},
		},
		{
			description: "no cluster label",
			content:     content,
			checker:     newAlertsExternalLabelsCheck,
			prometheus:  newSimpleProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Reporter: checks.AlertsExternalLabelsCheckName,
						Summary:  "invalid label",
						Details:  alertsExternalLabelsDetails("prom", uri),
						Severity: checks.Bug,
						Diagnostics: []diags.Diagnostic{
							{
								Message: alertsExternalLabelsText("prom", uri, "cluster"),
							},
						},
					},
					{
						Reporter: checks.AlertsExternalLabelsCheckName,
						Summary:  "invalid label",
						Details:  alertsExternalLabelsDetails("prom", uri),
						Severity: checks.Bug,
						Diagnostics: []diags.Diagnostic{
							{
								Message: alertsExternalLabelsText("prom", uri, "cluster"),
							},
						},
					},
					{
						Reporter: checks.AlertsExternalLabelsCheckName,
						Summary:  "invalid label",
						Details:  alertsExternalLabelsDetails("prom", uri),
						Severity: checks.Bug,
						Diagnostics: []diags.Diagnostic{
							{
								Message: alertsExternalLabelsText("prom", uri, "cluster"),
							},
						},
					},
				}
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  external_labels:\n    bob: foo\n"},
				},
			},
		},
		{
			description: "config 404",
			content:     content,
			checker:     newAlertsExternalLabelsCheck,
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
