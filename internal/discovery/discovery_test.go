package discovery

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/parser"
)

func TestReadRules(t *testing.T) {
	mustParse := func(offset int, s string) parser.Rule {
		p := parser.NewParser(false, parser.PrometheusSchema, model.UTF8Validation)
		file, _ := p.Parse(strings.NewReader(strings.Repeat("\n", offset) + s))
		if file.Error.Err != nil {
			panic(fmt.Sprintf("failed to parse rule:\n---\n%s\n---\nerror: %s", s, file.Error))
		}
		if len(file.Groups) != 1 {
			panic(fmt.Sprintf("wrong number of groups returned: %d\n---\n%s\n---", len(file.Groups), s))
		}
		if len(file.Groups[0].Rules) != 1 {
			panic(fmt.Sprintf("wrong number of rules returned: %d\n---\n%s\n---", len(file.Groups[0].Rules), s))
		}
		return file.Groups[0].Rules[0]
	}

	type testCaseT struct {
		sourceFunc   func(t *testing.T) io.Reader
		title        string
		reportedPath string
		sourcePath   string
		err          string
		entries      []Entry
		isStrict     bool
	}

	testCases := []testCaseT{
		{
			title:        "nil input",
			reportedPath: "rules.yml",
			sourcePath:   "rules.yml",
			sourceFunc: func(_ *testing.T) io.Reader {
				return bytes.NewBuffer(nil)
			},
			isStrict: false,
		},
		{
			title:        "nil input",
			reportedPath: "rules.yml",
			sourcePath:   "rules.yml",
			sourceFunc: func(_ *testing.T) io.Reader {
				return bytes.NewBuffer(nil)
			},
			isStrict: true,
		},
		{
			title:        "empty input",
			reportedPath: "rules.yml",
			sourcePath:   "rules.yml",
			sourceFunc: func(_ *testing.T) io.Reader {
				return bytes.NewBuffer([]byte("     "))
			},
			isStrict: false,
		},
		{
			title:        "empty input",
			reportedPath: "rules.yml",
			sourcePath:   "rules.yml",
			sourceFunc: func(_ *testing.T) io.Reader {
				return bytes.NewBuffer([]byte("     "))
			},
			isStrict: true,
		},
		{
			title:        "no rules, just a comment",
			reportedPath: "rules.yml",
			sourcePath:   "rules.yml",
			sourceFunc: func(_ *testing.T) io.Reader {
				return bytes.NewBuffer([]byte("\n\n   # pint file/disable xxx  \n\n"))
			},
			isStrict: false,
		},
		{
			title:        "file/disable comment",
			reportedPath: "rules.yml",
			sourcePath:   "rules.yml",
			sourceFunc: func(_ *testing.T) io.Reader {
				return bytes.NewBuffer([]byte(`
# pint file/disable promql/series

- record: foo
  expr: bar
`))
			},
			isStrict: false,
			entries: []Entry{
				{
					State: Unknown,
					Path: Path{
						Name:          "rules.yml",
						SymlinkTarget: "rules.yml",
					},
					ModifiedLines:  []int{4, 5},
					Rule:           mustParse(3, "- record: foo\n  expr: bar\n"),
					DisabledChecks: []string{"promql/series"},
				},
			},
		},
		{
			title:        "file/disable comment",
			reportedPath: "rules.yml",
			sourcePath:   "rules.yml",
			sourceFunc: func(_ *testing.T) io.Reader {
				return bytes.NewBuffer([]byte(`
# pint file/disable promql/series

groups:
- name: foo
  rules:
  - record: foo
    expr: bar
`))
			},
			isStrict: true,
			entries: []Entry{
				{
					State: Unknown,
					Path: Path{
						Name:          "rules.yml",
						SymlinkTarget: "rules.yml",
					},
					ModifiedLines:  []int{7, 8},
					Rule:           mustParse(6, "  - record: foo\n    expr: bar\n"),
					DisabledChecks: []string{"promql/series"},
				},
			},
		},
		{
			title:        "single expired snooze comment",
			reportedPath: "rules.yml",
			sourcePath:   "rules.yml",
			sourceFunc: func(_ *testing.T) io.Reader {
				return bytes.NewBuffer([]byte(`
# pint file/snooze 2000-01-01T00:00:00Z promql/series

- record: foo
  expr: bar
`))
			},
			isStrict: false,
			entries: []Entry{
				{
					State: Unknown,
					Path: Path{
						Name:          "rules.yml",
						SymlinkTarget: "rules.yml",
					},
					ModifiedLines: []int{4, 5},
					Rule:          mustParse(3, "- record: foo\n  expr: bar\n"),
				},
			},
		},
		{
			title:        "single expired snooze comment",
			reportedPath: "rules.yml",
			sourcePath:   "rules.yml",
			sourceFunc: func(_ *testing.T) io.Reader {
				return bytes.NewBuffer([]byte(`
# pint file/snooze 2000-01-01T00:00:00Z promql/series

groups:
- name: foo
  rules:
  - record: foo
    expr: bar
`))
			},
			isStrict: true,
			entries: []Entry{
				{
					State: Unknown,
					Path: Path{
						Name:          "rules.yml",
						SymlinkTarget: "rules.yml",
					},
					ModifiedLines: []int{7, 8},
					Rule:          mustParse(6, "  - record: foo\n    expr: bar\n"),
				},
			},
		},
		{
			title:        "single valid snooze comment",
			reportedPath: "rules.yml",
			sourcePath:   "rules.yml",
			sourceFunc: func(_ *testing.T) io.Reader {
				return bytes.NewBuffer([]byte(`
# pint file/snooze 2099-01-01T00:00:00Z promql/series

- record: foo
  expr: bar
`))
			},
			isStrict: false,
			entries: []Entry{
				{
					State: Unknown,
					Path: Path{
						Name:          "rules.yml",
						SymlinkTarget: "rules.yml",
					},
					ModifiedLines:  []int{4, 5},
					Rule:           mustParse(3, "- record: foo\n  expr: bar\n"),
					DisabledChecks: []string{"promql/series"},
				},
			},
		},
		{
			title:        "single valid snooze comment",
			reportedPath: "rules.yml",
			sourcePath:   "rules.yml",
			sourceFunc: func(_ *testing.T) io.Reader {
				return bytes.NewBuffer([]byte(`
# pint file/snooze 2099-01-01T00:00:00Z promql/series

groups:
- name: foo
  rules:
  - record: foo
    expr: bar
`))
			},
			isStrict: true,
			entries: []Entry{
				{
					State: Unknown,
					Path: Path{
						Name:          "rules.yml",
						SymlinkTarget: "rules.yml",
					},
					ModifiedLines:  []int{7, 8},
					Rule:           mustParse(6, "  - record: foo\n    expr: bar\n"),
					DisabledChecks: []string{"promql/series"},
				},
			},
		},
		{
			title:        "ignore/file",
			reportedPath: "rules.yml",
			sourcePath:   "rules.yml",
			sourceFunc: func(_ *testing.T) io.Reader {
				return bytes.NewBuffer([]byte(`
# pint ignore/file

- record: foo
  expr: bar
`))
			},
			isStrict: false,
			entries: []Entry{
				{
					State: Unknown,
					Path: Path{
						Name:          "rules.yml",
						SymlinkTarget: "rules.yml",
					},
					ModifiedLines: []int{1, 2, 3, 4, 5},
					PathError: FileIgnoreError{
						Diagnostic: diags.Diagnostic{
							Message: "This file was excluded from pint checks.",
							Pos: diags.PositionRanges{
								{Line: 2, FirstColumn: 1, LastColumn: 18},
							},
							FirstColumn: 1,
							LastColumn:  18,
						},
					},
				},
			},
		},
		{
			title:        "ignore/file",
			reportedPath: "rules.yml",
			sourcePath:   "rules.yml",
			sourceFunc: func(_ *testing.T) io.Reader {
				return bytes.NewBuffer([]byte(`
# pint ignore/file

groups:
- name: foo
  rules:
  - record: foo
    expr: bar
`))
			},
			isStrict: true,
			entries: []Entry{
				{
					State: Unknown,
					Path: Path{
						Name:          "rules.yml",
						SymlinkTarget: "rules.yml",
					},
					ModifiedLines: []int{1, 2, 3, 4, 5, 6, 7, 8},
					PathError: FileIgnoreError{
						Diagnostic: diags.Diagnostic{
							Message: "This file was excluded from pint checks.",
							Pos: diags.PositionRanges{
								{Line: 2, FirstColumn: 1, LastColumn: 18},
							},
							FirstColumn: 1,
							LastColumn:  18,
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(
			fmt.Sprintf("rPath=%s sPath=%s strict=%v title=%s", tc.reportedPath, tc.sourcePath, tc.isStrict, tc.title),
			func(t *testing.T) {
				r := tc.sourceFunc(t)
				p := parser.NewParser(tc.isStrict, parser.PrometheusSchema, model.UTF8Validation)
				entries, err := readRules(tc.reportedPath, tc.sourcePath, r, p, nil)
				if tc.err != "" {
					require.EqualError(t, err, tc.err)
				} else {
					require.NoError(t, err)

					expected, err := json.MarshalIndent(tc.entries, "", "  ")
					require.NoError(t, err, "json(expected)")
					got, err := json.MarshalIndent(entries, "", "  ")
					require.NoError(t, err, "json(got)")
					require.Equal(t, string(expected), string(got))
				}
			})
	}
}
