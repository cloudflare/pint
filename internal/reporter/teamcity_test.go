package reporter_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/neilotoole/slogt"
	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/git"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/reporter"
)

func TestTeamCityReporter(t *testing.T) {
	type testCaseT struct {
		description string
		output      string
		err         string
		summary     reporter.Summary
	}

	p := parser.NewParser(parser.DefaultOptions)
	mockFile := p.Parse(strings.NewReader(`
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
					Path: discovery.Path{
						SymlinkTarget: "foo.txt",
						Name:          "foo.txt",
					},
					Lines: git.LineMap{
						1: git.LineMeta{Old: 1},
						2: git.LineMeta{Old: 2, Modified: true},
					},
					Rule: mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines: diags.LineRange{
							First: 1,
							Last:  2,
						},
						Reporter: "mock",
						Summary:  "mock text",
						Details:  "mock details",
						Severity: checks.Information,
					},
				},
			}),
			output: `##teamcity[testSuiteStarted name='mock']
##teamcity[testSuiteStarted name='Information']
##teamcity[testStarted name='foo.txt:1']
##teamcity[testStdErr name='foo.txt:1' out='mock text']
##teamcity[testFinished name='foo.txt:1']
##teamcity[testSuiteFinished name='Information']
##teamcity[testSuiteFinished name='mock']
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
					Lines: git.LineMap{
						1: git.LineMeta{Old: 1},
						2: git.LineMeta{Old: 2, Modified: true},
					},
					Rule: mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines: diags.LineRange{
							First: 1,
							Last:  2,
						},
						Reporter: "mock",
						Summary:  "mock text",
						Severity: checks.Bug,
					},
				},
			}),
			output: `##teamcity[testSuiteStarted name='mock']
##teamcity[testSuiteStarted name='Bug']
##teamcity[testStarted name='foo.txt:1']
##teamcity[testFailed name='foo.txt:1' message='' details='mock text']
##teamcity[testFinished name='foo.txt:1']
##teamcity[testSuiteFinished name='Bug']
##teamcity[testSuiteFinished name='mock']
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
					Lines: git.LineMap{
						1: git.LineMeta{Old: 1},
						2: git.LineMeta{Old: 2, Modified: true},
					},
					Rule: mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines: diags.LineRange{
							First: 1,
							Last:  2,
						},
						Reporter: "mock",
						Summary: `mock text
with [new lines] and pipe| chars that are 'quoted'
`,
						Severity: checks.Bug,
					},
				},
			}),
			output: `##teamcity[testSuiteStarted name='mock']
##teamcity[testSuiteStarted name='Bug']
##teamcity[testStarted name='foo.txt:1']
##teamcity[testFailed name='foo.txt:1' message='' details='mock text|nwith |[new lines|] and pipe|| chars that are |'quoted|'|n']
##teamcity[testFinished name='foo.txt:1']
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
