package reporter_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/reporter"
)

func TestConsoleReporterReadFileFailures(t *testing.T) {
	type testCaseT struct {
		description string
		report      reporter.Report
	}

	testCases := []testCaseT{
		{
			description: "missing file falls back to summary details",
			report: reporter.Report{
				Path: discovery.Path{Name: "/this/path/does/not/exist", SymlinkTarget: "/this/path/does/not/exist"},
				Problem: checks.Problem{
					Anchor:   checks.AnchorAfter,
					Details:  "details from summary",
					Lines:    diags.LineRange{First: 1, Last: 1},
					Reporter: "mock",
					Severity: checks.Warning,
					Summary:  "summary text",
				},
			},
		},
		{
			description: "directory path falls back to summary details",
			report: reporter.Report{
				Path: func() discovery.Path {
					path := t.TempDir()
					return discovery.Path{Name: path, SymlinkTarget: path}
				}(),
				Problem: checks.Problem{
					Anchor:   checks.AnchorAfter,
					Details:  "details from summary",
					Lines:    diags.LineRange{First: 1, Last: 1},
					Reporter: "mock",
					Severity: checks.Warning,
					Summary:  "summary text",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			out := bytes.NewBuffer(nil)
			cr := reporter.NewConsoleReporter(out, checks.Information, true, true)
			summary := reporter.NewSummary([]reporter.Report{tc.report})

			err := cr.Submit(t.Context(), summary)
			require.NoError(t, err)
			require.Equal(
				t,
				"Warning: summary text (mock)\n  ---> "+tc.report.Path.Name+":1\n       ^^^ details from summary\n\n",
				out.String(),
			)
		})
	}
}
