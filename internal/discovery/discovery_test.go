package discovery

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/parser"
)

type failingReader struct {
	err error
}

func (r failingReader) Read(p []byte) (int, error) {
	return 0, r.err
}

func TestReadRules(t *testing.T) {
	mustParse := func(offset int, s string) parser.Rule {
		p := parser.NewParser()
		r, err := p.Parse([]byte(strings.Repeat("\n", offset) + s))
		if err != nil {
			panic(fmt.Sprintf("failed to parse rule:\n---\n%s\n---\nerror: %s", s, err))
		}
		if len(r) != 1 {
			panic(fmt.Sprintf("wrong number of rules returned: %d\n---\n%s\n---", len(r), s))
		}
		return r[0]
	}

	type testCaseT struct {
		title        string
		reportedPath string
		sourcePath   string
		sourceFunc   func(t *testing.T) io.Reader
		isStrict     bool
		entries      []Entry
		err          string
	}

	testCases := []testCaseT{
		{
			title:        "nil input",
			reportedPath: "rules.yml",
			sourcePath:   "rules.yml",
			sourceFunc: func(t *testing.T) io.Reader {
				return bytes.NewBuffer(nil)
			},
			isStrict: false,
		},
		{
			title:        "nil input",
			reportedPath: "rules.yml",
			sourcePath:   "rules.yml",
			sourceFunc: func(t *testing.T) io.Reader {
				return bytes.NewBuffer(nil)
			},
			isStrict: true,
		},
		{
			title:        "empty input",
			reportedPath: "rules.yml",
			sourcePath:   "rules.yml",
			sourceFunc: func(t *testing.T) io.Reader {
				return bytes.NewBuffer([]byte("     "))
			},
			isStrict: false,
		},
		{
			title:        "empty input",
			reportedPath: "rules.yml",
			sourcePath:   "rules.yml",
			sourceFunc: func(t *testing.T) io.Reader {
				return bytes.NewBuffer([]byte("     "))
			},
			isStrict: true,
		},
		{
			title:        "reader error",
			reportedPath: "rules.yml",
			sourcePath:   "rules.yml",
			sourceFunc: func(t *testing.T) io.Reader {
				return failingReader{
					err: io.ErrClosedPipe,
				}
			},
			isStrict: false,
			err:      io.ErrClosedPipe.Error(),
		},
		{
			title:        "reader error",
			reportedPath: "rules.yml",
			sourcePath:   "rules.yml",
			sourceFunc: func(t *testing.T) io.Reader {
				return failingReader{
					err: io.ErrClosedPipe,
				}
			},
			isStrict: true,
			err:      io.ErrClosedPipe.Error(),
		},
		{
			title:        "no rules, just a comment",
			reportedPath: "rules.yml",
			sourcePath:   "rules.yml",
			sourceFunc: func(t *testing.T) io.Reader {
				return bytes.NewBuffer([]byte("\n\n   # pint file/disable   \n\n"))
			},
			isStrict: false,
		},
		{
			title:        "file/disable comment",
			reportedPath: "rules.yml",
			sourcePath:   "rules.yml",
			sourceFunc: func(t *testing.T) io.Reader {
				return bytes.NewBuffer([]byte(`
# pint file/disable promql/series

- record: foo
  expr: bar
`))
			},
			isStrict: false,
			entries: []Entry{
				{
					State:          Unknown,
					ReportedPath:   "rules.yml",
					SourcePath:     "rules.yml",
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
			sourceFunc: func(t *testing.T) io.Reader {
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
					State:          Unknown,
					ReportedPath:   "rules.yml",
					SourcePath:     "rules.yml",
					ModifiedLines:  []int{7, 8},
					Rule:           mustParse(6, "- record: foo\n  expr: bar\n"),
					DisabledChecks: []string{"promql/series"},
				},
			},
		},
		{
			title:        "single expired snooze comment",
			reportedPath: "rules.yml",
			sourcePath:   "rules.yml",
			sourceFunc: func(t *testing.T) io.Reader {
				return bytes.NewBuffer([]byte(`
# pint file/snooze 2000-01-01T00:00:00Z promql/series

- record: foo
  expr: bar
`))
			},
			isStrict: false,
			entries: []Entry{
				{
					State:         Unknown,
					ReportedPath:  "rules.yml",
					SourcePath:    "rules.yml",
					ModifiedLines: []int{4, 5},
					Rule:          mustParse(3, "- record: foo\n  expr: bar\n"),
				},
			},
		},
		{
			title:        "single expired snooze comment",
			reportedPath: "rules.yml",
			sourcePath:   "rules.yml",
			sourceFunc: func(t *testing.T) io.Reader {
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
					State:         Unknown,
					ReportedPath:  "rules.yml",
					SourcePath:    "rules.yml",
					ModifiedLines: []int{7, 8},
					Rule:          mustParse(6, "- record: foo\n  expr: bar\n"),
				},
			},
		},
		{
			title:        "single valid snooze comment",
			reportedPath: "rules.yml",
			sourcePath:   "rules.yml",
			sourceFunc: func(t *testing.T) io.Reader {
				return bytes.NewBuffer([]byte(`
# pint file/snooze 2099-01-01T00:00:00Z promql/series

- record: foo
  expr: bar
`))
			},
			isStrict: false,
			entries: []Entry{
				{
					State:          Unknown,
					ReportedPath:   "rules.yml",
					SourcePath:     "rules.yml",
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
			sourceFunc: func(t *testing.T) io.Reader {
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
					State:          Unknown,
					ReportedPath:   "rules.yml",
					SourcePath:     "rules.yml",
					ModifiedLines:  []int{7, 8},
					Rule:           mustParse(6, "- record: foo\n  expr: bar\n"),
					DisabledChecks: []string{"promql/series"},
				},
			},
		},
		{
			title:        "ignore/file",
			reportedPath: "rules.yml",
			sourcePath:   "rules.yml",
			sourceFunc: func(t *testing.T) io.Reader {
				return bytes.NewBuffer([]byte(`
# pint ignore/file

- record: foo
  expr: bar
`))
			},
			isStrict: false,
			entries: []Entry{
				{
					State:         Unknown,
					ReportedPath:  "rules.yml",
					SourcePath:    "rules.yml",
					ModifiedLines: []int{1, 2, 3, 4, 5},
					PathError:     ErrFileIsIgnored,
				},
			},
		},
		{
			title:        "ignore/file",
			reportedPath: "rules.yml",
			sourcePath:   "rules.yml",
			sourceFunc: func(t *testing.T) io.Reader {
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
					State:         Unknown,
					ReportedPath:  "rules.yml",
					SourcePath:    "rules.yml",
					ModifiedLines: []int{1, 2, 3, 4, 5, 6, 7, 8},
					PathError:     ErrFileIsIgnored,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(
			fmt.Sprintf("rPath=%s sPath=%s strict=%v title=%s", tc.reportedPath, tc.sourcePath, tc.isStrict, tc.title),
			func(t *testing.T) {
				r := tc.sourceFunc(t)
				entries, err := readRules(tc.reportedPath, tc.sourcePath, r, tc.isStrict)
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
