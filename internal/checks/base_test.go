package checks_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/rs/zerolog"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
)

type checkTest struct {
	description string
	content     string
	checker     checks.RuleChecker
	problems    []checks.Problem
}

func runTests(t *testing.T, testCases []checkTest, opts ...cmp.Option) {
	zerolog.SetGlobalLevel(zerolog.FatalLevel)

	p := parser.NewParser()
	ctx := context.Background()
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			rules, err := p.Parse([]byte(tc.content))
			if err != nil {
				t.Fatal(err)
			}
			for _, rule := range rules {
				problems := tc.checker.Check(ctx, rule)
				if diff := cmp.Diff(tc.problems, problems, opts...); diff != "" {
					t.Fatalf("Check() returned wrong problem list (-want +got):\n%s", diff)
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
				_ = tc.checker.Check(ctx, rule)
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
				_ = tc.checker.Check(ctx, rule)
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
				t.Fatalf("checks.ParseSeverity() returned err=%v, expected=%v", err, tc.shouldError)
			}

			if hadError {
				return
			}

			if sev.String() != tc.output {
				t.Fatalf("checks.ParseSeverity() returned severity=%q, expected=%q", sev, tc.output)
			}
		})
	}
}

func simpleProm(name, uri string, timeout time.Duration, required bool) *promapi.FailoverGroup {
	return promapi.NewFailoverGroup(
		name,
		[]*promapi.Prometheus{
			promapi.NewPrometheus(name, uri, timeout),
		},
		required,
	)
}
