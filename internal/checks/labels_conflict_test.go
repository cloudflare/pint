package checks_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/promapi"
)

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
		},
		{
			description: "ignores alerting rules",
			content:     "- alert: foo\n  expr: up == 0\n",
			checker:     newLabelsConflict,
			prometheus:  newSimpleProm,
		},
		{
			description: "no labels / recording",
			content:     "- record: foo\n  expr: up == 0\n",
			checker:     newLabelsConflict,
			prometheus:  newSimpleProm,
		},
		{
			description: "no labels / alerting",
			content:     "- alert: foo\n  expr: up == 0\n",
			checker:     newLabelsConflict,
			prometheus:  newSimpleProm,
		},
		{
			description: "connection refused",
			content:     "- record: foo\n  expr: up == 0\n  labels:\n    foo: bar\n",
			checker:     newLabelsConflict,
			prometheus: func(_ string) *promapi.FailoverGroup {
				return simpleProm("prom", "http://127.0.0.1:1111", time.Second, false)
			},
			problems: true,
		},
		{
			description: "conflict / recording",
			content:     "- record: foo\n  expr: up == 0\n  labels:\n    foo: bar\n",
			checker:     newLabelsConflict,
			prometheus:  newSimpleProm,
			problems:    true,
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
			problems:    true,
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
			problems:    true,
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
