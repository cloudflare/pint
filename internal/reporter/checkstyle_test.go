package reporter_test

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/neilotoole/slogt"
	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/reporter"
)

func TestCheckstyleReporter(t *testing.T) {
	type testCaseT struct {
		description string
		output      string
		err         string
		summary     reporter.Summary
	}

	p := parser.NewParser(false)
	mockRules, _ := p.Parse([]byte(`
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
					Rule:          mockRules[0],
					Problem: checks.Problem{
						Lines: parser.LineRange{
							First: 5,
							Last:  6,
						},
						Reporter: "mock",
						Text:     "mock text",
						Details:  "mock details",
						Severity: checks.Information,
					},
				},
			}),
			output: `<?xml version="1.0" encoding="UTF-8"?>
<checkstyle version="4.3">
  <file name="foo.txt">
    <error line="5" severity="Information" message="Text:mock text&#xA; Details:mock details" source="mock"></error>
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
					Rule:          mockRules[0],
					Problem: checks.Problem{
						Lines: parser.LineRange{
							First: 5,
							Last:  6,
						},
						Reporter: "mock",
						Text:     "mock text",
						Severity: checks.Bug,
					},
				},
			}),
			output: `<?xml version="1.0" encoding="UTF-8"?>
<checkstyle version="4.3">
  <file name="foo.txt">
    <error line="5" severity="Bug" message="Text:mock text&#xA; Details:" source="mock"></error>
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
					Rule:          mockRules[0],
					Problem: checks.Problem{
						Lines: parser.LineRange{
							First: 5,
							Last:  6,
						},
						Reporter: "mock",
						Text: `mock text
		with [new lines] and pipe| chars that are 'quoted'
		`,
						Severity: checks.Bug,
					},
				},
			}),
			output: `<?xml version="1.0" encoding="UTF-8"?>
<checkstyle version="4.3">
  <file name="foo.txt">
    <error line="5" severity="Bug" message="Text:mock text&#xA;&#x9;&#x9;with [new lines] and pipe| chars that are &#39;quoted&#39;&#xA;&#x9;&#x9;&#xA; Details:" source="mock"></error>
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
			err := reporter.Submit(tc.summary)

			if tc.err != "" {
				require.EqualError(t, err, tc.err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.output, out.String())
			}
		})
	}
}
