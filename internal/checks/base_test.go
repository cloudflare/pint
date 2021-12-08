package checks_test

import (
	"testing"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/google/go-cmp/cmp"
)

type checkTest struct {
	description string
	content     string
	checker     checks.RuleChecker
	problems    []checks.Problem
}

func runTests(t *testing.T, testCases []checkTest, opts ...cmp.Option) {
	p := parser.NewParser()
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			rules, err := p.Parse([]byte(tc.content))
			if err != nil {
				t.Fatal(err)
			}
			for _, rule := range rules {
				problems := tc.checker.Check(rule)
				if diff := cmp.Diff(tc.problems, problems, opts...); diff != "" {
					t.Errorf("Check() returned wrong problem list (-want +got):\n%s", diff)
					return
				}
			}
		})
		t.Run(tc.description+" (bogus alerting rule)", func(t *testing.T) {
			rules, err := p.Parse([]byte(`
- alert: foo
  expr: 'foo{}{} > 0'
  annotations:
    summary: '{{ $labels.job }} is incorrect'
`))
			if err != nil {
				t.Fatal(err)
			}
			for _, rule := range rules {
				_ = tc.checker.Check(rule)
			}
		})
		t.Run(tc.description+" (bogus recording rule)", func(t *testing.T) {
			rules, err := p.Parse([]byte(`
- record: foo
  expr: 'foo{}{}'
`))
			if err != nil {
				t.Fatal(err)
			}
			for _, rule := range rules {
				_ = tc.checker.Check(rule)
			}
		})
	}
}

func TestParseSeverity(t *testing.T) {
	type testCaseT struct {
		input       string
		output      string
		shouldError bool
	}

	testCases := []testCaseT{
		{"xxx", "", true},
		{"Bug", "", true},
		{"fatal", "Fatal", false},
		{"bug", "Bug", false},
		{"info", "Information", false},
		{"warning", "Warning", false},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			sev, err := checks.ParseSeverity(tc.input)
			hadError := err != nil

			if hadError != tc.shouldError {
				t.Errorf("checks.ParseSeverity() returned err=%v, expected=%v", err, tc.shouldError)
				return
			}

			if hadError {
				return
			}

			if sev.String() != tc.output {
				t.Errorf("checks.ParseSeverity() returned severity=%q, expected=%q", sev, tc.output)
			}
		})
	}
}
