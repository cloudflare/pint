package reporter_test

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/neilotoole/slogt"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/reporter"
)

type failingWriter struct{}

func (failingWriter) Write(_ []byte) (int, error) {
	return 0, errors.New("write error")
}

func TestCheckstyleReporter(t *testing.T) {
	type testCaseT struct {
		description string
		output      string
		err         string
		summary     reporter.Summary
	}

	p := parser.NewParser(false, parser.PrometheusSchema, model.UTF8Validation)
	mockFile := p.Parse(strings.NewReader(`
- record: target is down
  expr: up == 0
`))

	testCases := []testCaseT{
		{
			description: "no reports",
			summary:     reporter.Summary{},
			output: `<?xml version="1.0" encoding="UTF-8"?>
<checkstyle version="4.3"></checkstyle>
`,
		},
		{
			description: "info report",
			summary: reporter.NewSummary([]reporter.Report{
				{
					Path: discovery.Path{
						SymlinkTarget: "foo.txt",
						Name:          "foo.txt",
					},
					ModifiedLines: []int{2, 4, 5},
					Rule:          mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines: diags.LineRange{
							First: 5,
							Last:  6,
						},
						Reporter: "mock",
						Summary:  "mock text",
						Details:  "mock details",
						Severity: checks.Information,
					},
				},
			}),
			output: `<?xml version="1.0" encoding="UTF-8"?>
<checkstyle version="4.3">
  <file name="foo.txt">
    <error line="5" severity="Information" message="mock text&#xA;mock details" source="mock"></error>
  </file>
</checkstyle>
`,
		},
		{
			description: "bug report",
			summary: reporter.NewSummary([]reporter.Report{
				{
					Path: discovery.Path{
						SymlinkTarget: "foo.txt",
						Name:          "foo.txt",
					},
					ModifiedLines: []int{2, 4, 5},
					Rule:          mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines: diags.LineRange{
							First: 5,
							Last:  6,
						},
						Reporter: "mock",
						Summary:  "mock text",
						Severity: checks.Bug,
					},
				},
			}),
			output: `<?xml version="1.0" encoding="UTF-8"?>
<checkstyle version="4.3">
  <file name="foo.txt">
    <error line="5" severity="Bug" message="mock text" source="mock"></error>
  </file>
</checkstyle>
`,
		},
		{
			description: "escaping characters",
			summary: reporter.NewSummary([]reporter.Report{
				{
					Path: discovery.Path{
						SymlinkTarget: "foo.txt",
						Name:          "foo.txt",
					},
					ModifiedLines: []int{2, 4, 5},
					Rule:          mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines: diags.LineRange{
							First: 5,
							Last:  6,
						},
						Reporter: "mock",
						Summary: `mock text
		with [new lines] and pipe| chars that are 'quoted'
		`,
						Severity: checks.Bug,
					},
				},
			}),
			output: `<?xml version="1.0" encoding="UTF-8"?>
<checkstyle version="4.3">
  <file name="foo.txt">
    <error line="5" severity="Bug" message="mock text&#xA;&#x9;&#x9;with [new lines] and pipe| chars that are &#39;quoted&#39;&#xA;&#x9;&#x9;" source="mock"></error>
  </file>
</checkstyle>
`,
		},
		{
			description: "multiple files",
			summary: reporter.NewSummary([]reporter.Report{
				{
					Path: discovery.Path{
						SymlinkTarget: "foo.txt",
						Name:          "foo.txt",
					},
					ModifiedLines: []int{1},
					Rule:          mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines:    diags.LineRange{First: 1, Last: 1},
						Reporter: "mock",
						Summary:  "problem in foo",
						Severity: checks.Warning,
					},
				},
				{
					Path: discovery.Path{
						SymlinkTarget: "bar.txt",
						Name:          "bar.txt",
					},
					ModifiedLines: []int{2},
					Rule:          mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines:    diags.LineRange{First: 2, Last: 2},
						Reporter: "mock",
						Summary:  "problem in bar",
						Severity: checks.Fatal,
					},
				},
			}),
			output: `<?xml version="1.0" encoding="UTF-8"?>
<checkstyle version="4.3">
  <file name="foo.txt">
    <error line="1" severity="Warning" message="problem in foo" source="mock"></error>
  </file>
  <file name="bar.txt">
    <error line="2" severity="Fatal" message="problem in bar" source="mock"></error>
  </file>
</checkstyle>
`,
		},
		{
			description: "multiple reports same file",
			summary: reporter.NewSummary([]reporter.Report{
				{
					Path: discovery.Path{
						SymlinkTarget: "foo.txt",
						Name:          "foo.txt",
					},
					ModifiedLines: []int{1, 5},
					Rule:          mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines:    diags.LineRange{First: 1, Last: 1},
						Reporter: "mock1",
						Summary:  "first problem",
						Severity: checks.Information,
					},
				},
				{
					Path: discovery.Path{
						SymlinkTarget: "foo.txt",
						Name:          "foo.txt",
					},
					ModifiedLines: []int{1, 5},
					Rule:          mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines:    diags.LineRange{First: 5, Last: 5},
						Reporter: "mock2",
						Summary:  "second problem",
						Details:  "with details",
						Severity: checks.Bug,
					},
				},
			}),
			output: `<?xml version="1.0" encoding="UTF-8"?>
<checkstyle version="4.3">
  <file name="foo.txt">
    <error line="1" severity="Information" message="first problem" source="mock1"></error>
    <error line="5" severity="Bug" message="second problem&#xA;with details" source="mock2"></error>
  </file>
</checkstyle>
`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			slog.SetDefault(slogt.New(t))

			out := bytes.NewBuffer(nil)

			reporter := reporter.NewCheckStyleReporter(out)
			err := reporter.Submit(t.Context(), tc.summary)

			if tc.err != "" {
				require.EqualError(t, err, tc.err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.output, out.String())
			}
		})
	}
}

func TestCheckstyleReporterWriteError(t *testing.T) {
	slog.SetDefault(slogt.New(t))

	r := reporter.NewCheckStyleReporter(failingWriter{})
	err := r.Submit(t.Context(), reporter.Summary{})
	require.EqualError(t, err, "write error")
}
