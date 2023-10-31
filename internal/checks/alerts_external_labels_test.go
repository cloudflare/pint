package checks_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/promapi"
)

func newAlertsExternalLabelsCheck(prom *promapi.FailoverGroup) checks.RuleChecker {
	return checks.NewAlertsExternalLabelsCheck(prom)
}

func alertsExternalLabelsText(name, uri, label string) string {
	return fmt.Sprintf("template is using %q external label but prometheus %q at %s doesn't have this label configured in global:external_labels", label, name, uri)
}

func TestAlertsExternalLabelsCountCheck(t *testing.T) {
	content := `
- alert: Foo Is Down
  expr: up{job="foo"} == 0
  annotations:
    summary: "{{ $labels.job }} is down"
    "{{.ExternalLabels.cluster}}": "This is {{ .ExternalLabels.cluster }} cluster"
  labels:
    job: "{{ $labels.job }}"
    cluster: "{{ $externalLabels.cluster }} / {{ $externalLabels.cluster }}"
    "{{ $externalLabels.cluster }}": "{{ $externalLabels.cluster }}"
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
						Fragment: "alert: Foo Is Down",
						Lines:    []int{2, 3, 4, 5, 6, 7, 8, 9, 10},
						Reporter: checks.AlertsExternalLabelsCheckName,
						Text:     checkErrorBadData("prom", uri, "bad_data: bad input data"),
						Severity: checks.Bug,
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
			prometheus: func(s string) *promapi.FailoverGroup {
				return simpleProm("prom", "http://127.0.0.1:1111", time.Second*5, false)
			},
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{
					{
						Fragment: "alert: Foo Is Down",
						Lines:    []int{2, 3, 4, 5, 6, 7, 8, 9, 10},
						Reporter: checks.AlertsExternalLabelsCheckName,
						Text:     checkErrorUnableToRun(checks.AlertsExternalLabelsCheckName, "prom", "http://127.0.0.1:1111", `connection refused`),
						Severity: checks.Warning,
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
						Fragment: `cluster: {{ $externalLabels.cluster }} / {{ $externalLabels.cluster }}`,
						Lines:    []int{9},
						Reporter: checks.AlertsExternalLabelsCheckName,
						Text:     alertsExternalLabelsText("prom", uri, "cluster"),
						Severity: checks.Bug,
					},
					{
						Fragment: `{{ $externalLabels.cluster }}: {{ $externalLabels.cluster }}`,
						Lines:    []int{10},
						Reporter: checks.AlertsExternalLabelsCheckName,
						Text:     alertsExternalLabelsText("prom", uri, "cluster"),
						Severity: checks.Bug,
					},
					{
						Fragment: `{{ $externalLabels.cluster }}: {{ $externalLabels.cluster }}`,
						Lines:    []int{10},
						Reporter: checks.AlertsExternalLabelsCheckName,
						Text:     alertsExternalLabelsText("prom", uri, "cluster"),
						Severity: checks.Bug,
					},
					{
						Fragment: `{{.ExternalLabels.cluster}}: This is {{ .ExternalLabels.cluster }} cluster`,
						Lines:    []int{6},
						Reporter: checks.AlertsExternalLabelsCheckName,
						Text:     alertsExternalLabelsText("prom", uri, "cluster"),
						Severity: checks.Bug,
					},
					{
						Fragment: `{{.ExternalLabels.cluster}}: This is {{ .ExternalLabels.cluster }} cluster`,
						Lines:    []int{6},
						Reporter: checks.AlertsExternalLabelsCheckName,
						Text:     alertsExternalLabelsText("prom", uri, "cluster"),
						Severity: checks.Bug,
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
	}

	runTests(t, testCases)
}
