package checks_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/promapi"
)

func newAlertsExternalLabelsCheck(prom *promapi.FailoverGroup) checks.RuleChecker {
	return checks.NewAlertsExternalLabelsCheck(prom)
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
		},
		{
			description: "ignores rules with syntax errors",
			content:     "- alert: Foo Is Down\n  expr: sum(\n",
			checker:     newAlertsExternalLabelsCheck,
			prometheus:  newSimpleProm,
		},
		{
			description: "bad request",
			content:     content,
			checker:     newAlertsExternalLabelsCheck,
			prometheus:  newSimpleProm,
			problems:    true,
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
			problems: true,
		},
		{
			description: "all labels present",
			content:     content,
			checker:     newAlertsExternalLabelsCheck,
			prometheus:  newSimpleProm,
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
			problems:    true,
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
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  httpResponse{code: http.StatusNotFound, body: "Not Found"},
				},
			},
		},
		{
			description:   "no cluster label / strict / group labels set",
			contentStrict: true,
			content: `
groups:
- name: mygroup
  labels:
    cluster: "{{ $externalLabels.cluster }}"
  rules:
  - alert: Foo Is Down
    expr: up{job="foo"} == 0
`,
			checker:    newAlertsExternalLabelsCheck,
			prometheus: newSimpleProm,
			problems:   true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  external_labels:\n    bob: foo\n"},
				},
			},
		},
		{
			description: "no cluster label / relaxed / group labels set",
			content: `
groups:
- name: mygroup
  labels:
    cluster: "{{ $externalLabels.cluster }}"
  rules:
  - alert: Foo Is Down
    expr: up{job="foo"} == 0
`,
			checker:    newAlertsExternalLabelsCheck,
			prometheus: newSimpleProm,
			problems:   true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  external_labels:\n    bob: foo\n"},
				},
			},
		},
		{
			description: "invalid template syntax in annotation",
			content: `
- alert: Foo Is Down
  expr: up{job="foo"} == 0
  annotations:
    summary: "{{ $externalLabels.cluster"
`,
			checker:    newAlertsExternalLabelsCheck,
			prometheus: newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  external_labels:\n    cluster: foo\n"},
				},
			},
		},
		{
			description: "invalid template syntax in label",
			content: `
- alert: Foo Is Down
  expr: up{job="foo"} == 0
  labels:
    cluster: "{{ $externalLabels.cluster"
`,
			checker:    newAlertsExternalLabelsCheck,
			prometheus: newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireConfigPath},
					resp:  configResponse{yaml: "global:\n  external_labels:\n    cluster: foo\n"},
				},
			},
		},
	}

	runTests(t, testCases)
}
