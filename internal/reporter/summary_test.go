package reporter

import (
	"testing"

	"github.com/cloudflare/pint/internal/checks"

	"github.com/stretchr/testify/require"
)

func TestNewSummary(t *testing.T) {
	summary := NewSummary(nil)
	require.Empty(t, summary.Reports())
}

func TestSummaryMarkCheckDisabled(t *testing.T) {
	summary := NewSummary(nil)
	summary.MarkCheckDisabled("prom", "config", []string{"check"})
	details := summary.GetPrometheusDetails()
	require.Len(t, details, 1)
	require.Equal(t, "prom", details[0].Name)
	require.Len(t, details[0].DisabledChecks, 1)
	require.Equal(t, "config", details[0].DisabledChecks[0].API)
	require.Len(t, details[0].DisabledChecks[0].Checks, 1)
	require.Equal(t, "check", details[0].DisabledChecks[0].Checks[0])
}

func TestSummaryReport(t *testing.T) {
	summary := NewSummary(nil)
	summary.Report(Report{})
	require.Len(t, summary.Reports(), 1)
}

func TestSummarySortReports(t *testing.T) {
	summary := NewSummary([]Report{
		{Problem: checks.Problem{Severity: checks.Bug}},
		{Problem: checks.Problem{Severity: checks.Warning}},
	})
	summary.SortReports()
	reports := summary.Reports()
	require.Equal(t, checks.Warning, reports[0].Problem.Severity)
	require.Equal(t, checks.Bug, reports[1].Problem.Severity)
}
