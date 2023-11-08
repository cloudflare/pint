package reporter_test

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/neilotoole/slogt"
	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/reporter"
)

func TestTeamCityReporter(t *testing.T) {
	type testCaseT struct {
		description string
		summary     reporter.Summary
		output      string
		err         string
	}

	p := parser.NewParser()
	mockRules, _ := p.Parse([]byte(`
- record: target is down
  expr: up == 0
`))

	testCases := []testCaseT{
		{
			description: "no reports",
			summary:     reporter.Summary{},
			output:      "",
		},
		{
			description: "info report",
			summary: reporter.NewSummary([]reporter.Report{
				{
					ReportedPath:  "foo.txt",
					SourcePath:    "foo.txt",
					ModifiedLines: []int{2, 4, 5},
					Rule:          mockRules[0],
					Problem: checks.Problem{
						Fragment: "up",
						Lines:    []int{5, 6},
						Reporter: "mock",
						Text:     "mock text",
						Details:  "mock details",
						Severity: checks.Information,
					},
				},
			}),
			output: `##teamcity[testSuiteStarted name='mock']
##teamcity[testSuiteStarted name='Information']
##teamcity[testStarted name='foo.txt:5']
##teamcity[testStdErr name='foo.txt:5' out='mock text']
##teamcity[testFinished name='foo.txt:5']
##teamcity[testSuiteFinished name='Information']
##teamcity[testSuiteFinished name='mock']
`,
		},
		{
			description: "bug report",
			summary: reporter.NewSummary([]reporter.Report{
				{
					ReportedPath:  "foo.txt",
					SourcePath:    "foo.txt",
					ModifiedLines: []int{2, 4, 5},
					Rule:          mockRules[0],
					Problem: checks.Problem{
						Fragment: "up",
						Lines:    []int{5, 6},
						Reporter: "mock",
						Text:     "mock text",
						Severity: checks.Bug,
					},
				},
			}),
			output: `##teamcity[testSuiteStarted name='mock']
##teamcity[testSuiteStarted name='Bug']
##teamcity[testStarted name='foo.txt:5']
##teamcity[testFailed name='foo.txt:5' message='' details='mock text']
##teamcity[testFinished name='foo.txt:5']
##teamcity[testSuiteFinished name='Bug']
##teamcity[testSuiteFinished name='mock']
`,
		},
		{
			description: "escaping characters",
			summary: reporter.NewSummary([]reporter.Report{
				{
					ReportedPath:  "foo.txt",
					SourcePath:    "foo.txt",
					ModifiedLines: []int{2, 4, 5},
					Rule:          mockRules[0],
					Problem: checks.Problem{
						Fragment: "up",
						Lines:    []int{5, 6},
						Reporter: "mock",
						Text: `mock text
with [new lines] and pipe| chars that are 'quoted'
`,
						Severity: checks.Bug,
					},
				},
			}),
			output: `##teamcity[testSuiteStarted name='mock']
##teamcity[testSuiteStarted name='Bug']
##teamcity[testStarted name='foo.txt:5']
##teamcity[testFailed name='foo.txt:5' message='' details='mock text|nwith |[new lines|] and pipe|| chars that are |'quoted|'|n']
##teamcity[testFinished name='foo.txt:5']
##teamcity[testSuiteFinished name='Bug']
##teamcity[testSuiteFinished name='mock']
`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			slog.SetDefault(slogt.New(t))

			out := bytes.NewBuffer(nil)

			reporter := reporter.NewTeamCityReporter(out)
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
