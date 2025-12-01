package main

import (
	"context"
	"testing"
	"time"

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
	return checks.CheckMeta{Online: false}
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

func TestScanWorker(t *testing.T) {
	type testCaseT struct {
		name            string
		jobs            []scanJob
		expectedReports int
		cancelCtx       bool
	}

	testCases := []testCaseT{
		{
			name:      "context cancelled before job processed",
			cancelCtx: true,
			jobs: []scanJob{
				{
					check: testCheck{name: "test"},
					entry: &discovery.Entry{
						State: discovery.Noop,
						Rule:  parser.Rule{},
					},
				},
			},
			expectedReports: 0,
		},
		{
			name:      "unknown state triggers warning log",
			cancelCtx: false,
			jobs: []scanJob{
				{
					check: testCheck{name: "test"},
					entry: &discovery.Entry{
						State: discovery.Unknown,
						Path:  discovery.Path{Name: "test.yaml"},
						Rule:  parser.Rule{},
					},
				},
			},
			expectedReports: 0,
		},
		{
			name:      "job with problems produces reports",
			cancelCtx: false,
			jobs: []scanJob{
				{
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
				},
			},
			expectedReports: 1,
		},
		{
			name:      "multiple jobs produce multiple reports",
			cancelCtx: false,
			jobs: []scanJob{
				{
					check: testCheck{
						name:     "test",
						problems: []checks.Problem{{Severity: checks.Bug, Summary: "p1"}},
					},
					entry: &discovery.Entry{
						State: discovery.Noop,
						Path:  discovery.Path{Name: "test.yaml"},
						Rule:  parser.Rule{},
					},
				},
				{
					check: testCheck{
						name:     "test",
						problems: []checks.Problem{{Severity: checks.Bug, Summary: "p2"}},
					},
					entry: &discovery.Entry{
						State: discovery.Noop,
						Path:  discovery.Path{Name: "test.yaml"},
						Rule:  parser.Rule{},
					},
				},
				{
					check: testCheck{
						name:     "test",
						problems: []checks.Problem{{Severity: checks.Bug, Summary: "p3"}},
					},
					entry: &discovery.Entry{
						State: discovery.Noop,
						Path:  discovery.Path{Name: "test.yaml"},
						Rule:  parser.Rule{},
					},
				},
			},
			expectedReports: 3,
		},
		{
			name:      "job with no problems produces no reports",
			cancelCtx: false,
			jobs: []scanJob{
				{
					check: testCheck{name: "test", problems: nil},
					entry: &discovery.Entry{
						State: discovery.Noop,
						Path:  discovery.Path{Name: "test.yaml"},
						Rule:  parser.Rule{},
					},
				},
			},
			expectedReports: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			jobs := make(chan scanJob, len(tc.jobs)+1)
			results := make(chan reporter.Report, 10)

			done := make(chan struct{})
			go func() {
				scanWorker(ctx, jobs, results)
				close(results)
				close(done)
			}()

			if tc.cancelCtx {
				cancel()
			}

			for _, job := range tc.jobs {
				jobs <- job
			}
			close(jobs)

			var reports []reporter.Report
			for r := range results {
				reports = append(reports, r)
			}

			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("worker did not finish in time")
			}

			require.Len(t, reports, tc.expectedReports)
		})
	}
}
