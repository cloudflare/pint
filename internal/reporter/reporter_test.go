package reporter

import (
	"testing"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"

	"github.com/stretchr/testify/require"
)

func TestReportIsEqual(t *testing.T) {
	type testCaseT struct {
		a, b     Report
		expected bool
	}

	testCases := []testCaseT{
		{
			a:        Report{Path: discovery.Path{Name: "foo"}, Rule: parser.Rule{}},
			b:        Report{Path: discovery.Path{Name: "foo"}, Rule: parser.Rule{}},
			expected: true,
		},
		{
			a:        Report{Path: discovery.Path{Name: "foo"}, Rule: parser.Rule{}},
			b:        Report{Path: discovery.Path{Name: "bar"}, Rule: parser.Rule{}},
			expected: false,
		},
		{
			a:        Report{Path: discovery.Path{Name: "foo"}, Rule: parser.Rule{}, Owner: "bob"},
			b:        Report{Path: discovery.Path{Name: "foo"}, Rule: parser.Rule{}},
			expected: false,
		},
		{
			a:        Report{Path: discovery.Path{Name: "foo"}, Rule: parser.Rule{}, Problem: checks.Problem{Lines: diags.LineRange{First: 1}}},
			b:        Report{Path: discovery.Path{Name: "foo"}, Rule: parser.Rule{}},
			expected: false,
		},
		{
			a:        Report{Path: discovery.Path{Name: "foo"}, Rule: parser.Rule{Lines: diags.LineRange{Last: 2}}, Problem: checks.Problem{Lines: diags.LineRange{Last: 1}}},
			b:        Report{Path: discovery.Path{Name: "foo"}, Rule: parser.Rule{Lines: diags.LineRange{Last: 1}}, Problem: checks.Problem{Lines: diags.LineRange{Last: 1}}},
			expected: false,
		},
		{
			a:        Report{Path: discovery.Path{Name: "foo"}, Rule: parser.Rule{Lines: diags.LineRange{Last: 2}}, Problem: checks.Problem{Lines: diags.LineRange{Last: 1}}},
			b:        Report{Path: discovery.Path{Name: "foo"}, Rule: parser.Rule{Lines: diags.LineRange{Last: 1}}, Problem: checks.Problem{Lines: diags.LineRange{Last: 1}}},
			expected: false,
		},
		{
			a:        Report{Path: discovery.Path{Name: "foo"}, Rule: parser.Rule{}, Problem: checks.Problem{Reporter: "a"}},
			b:        Report{Path: discovery.Path{Name: "foo"}, Rule: parser.Rule{}, Problem: checks.Problem{Reporter: "b"}},
			expected: false,
		},
		{
			a:        Report{Path: discovery.Path{Name: "foo"}, Rule: parser.Rule{}, Problem: checks.Problem{Summary: "a"}},
			b:        Report{Path: discovery.Path{Name: "foo"}, Rule: parser.Rule{}, Problem: checks.Problem{Summary: "b"}},
			expected: false,
		},
		{
			a:        Report{Path: discovery.Path{Name: "foo"}, Rule: parser.Rule{}, Problem: checks.Problem{Severity: checks.Bug}},
			b:        Report{Path: discovery.Path{Name: "foo"}, Rule: parser.Rule{}, Problem: checks.Problem{Severity: checks.Warning}},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run("isEqual", func(t *testing.T) {
			require.Equal(t, tc.expected, tc.a.isEqual(tc.b))
		})
	}
}

func TestSummaryHasReport(t *testing.T) {
	summary := NewSummary([]Report{
		{Path: discovery.Path{Name: "foo"}, Rule: parser.Rule{}},
	})

	t.Run("has report", func(t *testing.T) {
		require.True(t, summary.hasReport(Report{Path: discovery.Path{Name: "foo"}, Rule: parser.Rule{}}))
	})

	t.Run("doesn't have report", func(t *testing.T) {
		require.False(t, summary.hasReport(Report{Path: discovery.Path{Name: "bar"}, Rule: parser.Rule{}}))
	})
}

func TestIsSameDiagnostics(t *testing.T) {
	type testCaseT struct {
		name     string
		a, b     []diags.Diagnostic
		expected bool
	}

	testCases := []testCaseT{
		{
			name:     "both empty",
			expected: true,
		},
		{
			name:     "a empty",
			b:        []diags.Diagnostic{{Message: "foo"}},
			expected: false,
		},
		{
			name:     "b empty",
			a:        []diags.Diagnostic{{Message: "foo"}},
			expected: false,
		},
		{
			name:     "same",
			a:        []diags.Diagnostic{{Message: "foo"}},
			b:        []diags.Diagnostic{{Message: "foo"}},
			expected: true,
		},
		{
			name:     "different",
			a:        []diags.Diagnostic{{Message: "foo"}},
			b:        []diags.Diagnostic{{Message: "bar"}},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, isSameDiagnostics(tc.a, tc.b))
		})
	}
}

func TestIsSameDiagnosticsMessage(t *testing.T) {
	type testCaseT struct {
		name     string
		a, b     []diags.Diagnostic
		expected bool
	}

	testCases := []testCaseT{
		{
			name:     "both empty",
			expected: true,
		},
		{
			name:     "a empty",
			b:        []diags.Diagnostic{{Message: "foo"}},
			expected: false,
		},
		{
			name:     "b empty",
			a:        []diags.Diagnostic{{Message: "foo"}},
			expected: false,
		},
		{
			name:     "same",
			a:        []diags.Diagnostic{{Message: "foo"}},
			b:        []diags.Diagnostic{{Message: "foo"}},
			expected: true,
		},
		{
			name:     "different",
			a:        []diags.Diagnostic{{Message: "foo"}},
			b:        []diags.Diagnostic{{Message: "bar"}},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, isSameDiagnosticsMessage(tc.a, tc.b))
		})
	}
}

func TestReportIsSameIssue(t *testing.T) {
	type testCaseT struct {
		name     string
		a, b     Report
		expected bool
	}

	testCases := []testCaseT{
		{
			name: "same",
			a: Report{
				Problem: checks.Problem{
					Reporter: "promql/series",
					Summary:  "Series has high cardinality",
					Severity: checks.Warning,
				},
			},
			b: Report{
				Problem: checks.Problem{
					Reporter: "promql/series",
					Summary:  "Series has high cardinality",
					Severity: checks.Warning,
				},
			},
			expected: true,
		},
		{
			name: "different reporter",
			a: Report{
				Problem: checks.Problem{
					Reporter: "promql/series",
					Summary:  "Series has high cardinality",
					Severity: checks.Warning,
				},
			},
			b: Report{
				Problem: checks.Problem{
					Reporter: "promql/rate",
					Summary:  "Series has high cardinality",
					Severity: checks.Warning,
				},
			},
			expected: false,
		},
		{
			name: "different summary",
			a: Report{
				Problem: checks.Problem{
					Reporter: "promql/series",
					Summary:  "a",
					Severity: checks.Warning,
				},
			},
			b: Report{
				Problem: checks.Problem{
					Reporter: "promql/series",
					Summary:  "b",
					Severity: checks.Warning,
				},
			},
			expected: false,
		},
		{
			name: "different severity",
			a: Report{
				Problem: checks.Problem{
					Reporter: "promql/series",
					Summary:  "a",
					Severity: checks.Warning,
				},
			},
			b: Report{
				Problem: checks.Problem{
					Reporter: "promql/series",
					Summary:  "a",
					Severity: checks.Bug,
				},
			},
			expected: false,
		},
		{
			name: "different diagnostics message",
			a: Report{
				Problem: checks.Problem{
					Reporter:    "promql/series",
					Summary:     "a",
					Severity:    checks.Warning,
					Diagnostics: []diags.Diagnostic{{Message: "x"}},
				},
			},
			b: Report{
				Problem: checks.Problem{
					Reporter:    "promql/series",
					Summary:     "a",
					Severity:    checks.Warning,
					Diagnostics: []diags.Diagnostic{{Message: "y"}},
				},
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, tc.a.isSameIssue(tc.b))
		})
	}
}

func TestSummaryReportsPerPath(t *testing.T) {
	summary := NewSummary([]Report{
		{Path: discovery.Path{Name: "foo"}},
		{Path: discovery.Path{Name: "foo"}},
		{Path: discovery.Path{Name: "bar"}},
	})

	reports := summary.ReportsPerPath()
	require.Len(t, reports, 2)
	require.Len(t, reports[0], 2)
	require.Len(t, reports[1], 1)
}

func TestSummaryHasFatalProblems(t *testing.T) {
	t.Run("no fatals", func(t *testing.T) {
		summary := NewSummary([]Report{
			{Problem: checks.Problem{Severity: checks.Warning}},
		})
		require.False(t, summary.HasFatalProblems())
	})

	t.Run("with fatals", func(t *testing.T) {
		summary := NewSummary([]Report{
			{Problem: checks.Problem{Severity: checks.Warning}},
			{Problem: checks.Problem{Severity: checks.Fatal}},
		})
		require.True(t, summary.HasFatalProblems())
	})
}

func TestSummaryCountBySeverity(t *testing.T) {
	summary := NewSummary([]Report{
		{Problem: checks.Problem{Severity: checks.Warning}},
		{Problem: checks.Problem{Severity: checks.Warning}},
		{Problem: checks.Problem{Severity: checks.Bug}},
	})

	counts := summary.CountBySeverity()
	require.Equal(t, 2, counts[checks.Warning])
	require.Equal(t, 1, counts[checks.Bug])
}

func TestMakePrometheusDetailsComment(t *testing.T) {
	s := NewSummary(nil)
	s.MarkCheckDisabled("prom-b", "api-2", []string{"z", "a"})
	s.MarkCheckDisabled("prom-b", "api-1", []string{"m"})
	s.MarkCheckDisabled("prom-a", "api-1", []string{"x"})

	comment := makePrometheusDetailsComment(s)

	expected := `Some checks were disabled because one or more configured Prometheus server doesn't seem to support all required Prometheus APIs.
This usually means that you're running pint against a service like Thanos or Mimir that allows to query metrics but doesn't implement all APIs documented [here](https://prometheus.io/docs/prometheus/latest/querying/api/).
Since pint uses many of these API endpoint for querying information needed to run online checks only a real Prometheus server will allow it to run all of these checks.
Below is the list of checks that were disabled for each Prometheus server defined in pint config file.

- ` + "`prom-a`" + `
  - ` + "`api-1`" + ` is unsupported, disabled checks:
    - [x](https://cloudflare.github.io/pint/checks/x.html)
- ` + "`prom-b`" + `
  - ` + "`api-1`" + ` is unsupported, disabled checks:
    - [m](https://cloudflare.github.io/pint/checks/m.html)
  - ` + "`api-2`" + ` is unsupported, disabled checks:
    - [a](https://cloudflare.github.io/pint/checks/a.html)
    - [z](https://cloudflare.github.io/pint/checks/z.html)
`
	require.Equal(t, expected, comment)
}

func TestSummaryDedup(t *testing.T) {
	summary := NewSummary([]Report{
		{Problem: checks.Problem{Reporter: "a", Summary: "a", Severity: checks.Bug}},
		{Problem: checks.Problem{Reporter: "a", Summary: "a", Severity: checks.Bug}},
		{Problem: checks.Problem{Reporter: "b", Summary: "b", Severity: checks.Warning}},
	})

	summary.Dedup()

	var notDup int
	for _, r := range summary.Reports() {
		if !r.IsDuplicate {
			notDup++
		}
	}
	require.Equal(t, 2, notDup)
}
