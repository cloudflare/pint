package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/reporter"
)

type testCheck struct {
	name     string
	problems []checks.Problem
}

func (tc testCheck) Meta() checks.CheckMeta {
	return checks.CheckMeta{}
}

func (tc testCheck) String() string {
	return tc.name
}

func (tc testCheck) Reporter() string {
	return tc.name
}

func (tc testCheck) Check(_ context.Context, _ *discovery.Entry, _ []*discovery.Entry) []checks.Problem {
	return tc.problems
}

func TestRunCheck(t *testing.T) {
	type testCaseT struct {
		name            string
		setup           func(context.Context) context.Context
		check           checks.RuleChecker
		entry           *discovery.Entry
		entries         []*discovery.Entry
		expectedReports []reporter.Report
	}

	testCases := []testCaseT{
		{
			name: "context cancelled before check runs",
			setup: func(ctx context.Context) context.Context {
				ctx, cancel := context.WithCancel(ctx)
				cancel()
				return ctx
			},
			check: testCheck{name: "test"},
			entry: &discovery.Entry{
				State: discovery.Noop,
				Rule:  parser.Rule{},
			},
			expectedReports: nil,
		},
		{
			name:  "unknown state produces no reports",
			check: testCheck{name: "test"},
			entry: &discovery.Entry{
				State: discovery.Unknown,
				Path:  discovery.Path{Name: "test.yaml"},
				Rule:  parser.Rule{},
			},
			expectedReports: nil,
		},
		{
			name: "check with single problem produces one report",
			check: testCheck{
				name: "test",
				problems: []checks.Problem{
					{Severity: checks.Bug, Summary: "test problem"},
				},
			},
			entry: &discovery.Entry{
				State: discovery.Noop,
				Path:  discovery.Path{Name: "test.yaml"},
				Rule:  parser.Rule{},
			},
			expectedReports: []reporter.Report{
				{
					Path:        discovery.Path{Name: "test.yaml"},
					Rule:        parser.Rule{},
					Problem:     checks.Problem{Severity: checks.Bug, Summary: "test problem"},
					IsDuplicate: false,
					Duplicates:  nil,
				},
			},
		},
		{
			name: "check with multiple problems produces multiple reports",
			check: testCheck{
				name: "test",
				problems: []checks.Problem{
					{Severity: checks.Bug, Summary: "p1"},
					{Severity: checks.Warning, Summary: "p2"},
				},
			},
			entry: &discovery.Entry{
				State: discovery.Noop,
				Path:  discovery.Path{Name: "test.yaml"},
				Rule:  parser.Rule{},
			},
			expectedReports: []reporter.Report{
				{
					Path:        discovery.Path{Name: "test.yaml"},
					Rule:        parser.Rule{},
					Problem:     checks.Problem{Severity: checks.Bug, Summary: "p1"},
					IsDuplicate: false,
					Duplicates:  nil,
				},
				{
					Path:        discovery.Path{Name: "test.yaml"},
					Rule:        parser.Rule{},
					Problem:     checks.Problem{Severity: checks.Warning, Summary: "p2"},
					IsDuplicate: false,
					Duplicates:  nil,
				},
			},
		},
		{
			name: "check with no problems produces no reports",
			check: testCheck{
				name:     "test",
				problems: nil,
			},
			entry: &discovery.Entry{
				State: discovery.Noop,
				Path:  discovery.Path{Name: "test.yaml"},
				Rule:  parser.Rule{},
			},
			expectedReports: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.setup != nil {
				ctx = tc.setup(ctx)
			}

			reports := runCheck(ctx, tc.check, tc.entry, tc.entries)
			require.Equal(t, tc.expectedReports, reports)
		})
	}
}
