package discovery

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
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
		file := p.Parse(strings.NewReader(strings.Repeat("\n", offset) + s))
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
							Kind:        diags.Issue,
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
							Kind:        diags.Issue,
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
				entries := readRules(tc.reportedPath, tc.sourcePath, r, p, nil)
				expected, err := json.MarshalIndent(tc.entries, "", "  ")
				require.NoError(t, err, "json(expected)")
				got, err := json.MarshalIndent(entries, "", "  ")
				require.NoError(t, err, "json(got)")
				require.Equal(t, string(expected), string(got))
			})
	}
}

func TestChangeTypeStringDefault(t *testing.T) {
	c := ChangeType(255)
	require.Equal(t, "---", c.String())
}

func TestPathString(t *testing.T) {
	testCases := []struct {
		title    string
		path     Path
		expected string
	}{
		{
			title:    "no symlink",
			path:     Path{Name: "rules.yml", SymlinkTarget: "rules.yml"},
			expected: "rules.yml",
		},
		{
			title:    "with symlink",
			path:     Path{Name: "link.yml", SymlinkTarget: "rules.yml"},
			expected: "link.yml ~> rules.yml",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			require.Equal(t, tc.expected, tc.path.String())
		})
	}
}

func TestCommonLines(t *testing.T) {
	testCases := []struct {
		title    string
		a        []int
		b        []int
		expected []int
	}{
		{
			title:    "both empty",
			a:        nil,
			b:        nil,
			expected: nil,
		},
		{
			title:    "a empty",
			a:        nil,
			b:        []int{1, 2, 3},
			expected: nil,
		},
		{
			title:    "b empty",
			a:        []int{1, 2, 3},
			b:        nil,
			expected: nil,
		},
		{
			title:    "no overlap",
			a:        []int{1, 2, 3},
			b:        []int{4, 5, 6},
			expected: nil,
		},
		{
			title:    "full overlap same order",
			a:        []int{1, 2, 3},
			b:        []int{1, 2, 3},
			expected: []int{1, 2, 3},
		},
		{
			title:    "partial overlap from a",
			a:        []int{1, 2, 3},
			b:        []int{2, 3, 4},
			expected: []int{2, 3},
		},
		{
			title:    "partial overlap b has extras",
			a:        []int{2, 3},
			b:        []int{1, 2, 3, 4},
			expected: []int{2, 3},
		},
		{
			title:    "b has element in a not yet in common",
			a:        []int{1, 3},
			b:        []int{3, 1},
			expected: []int{1, 3},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			result := commonLines(tc.a, tc.b)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestFindSymlinks(t *testing.T) {
	testCases := []struct {
		title   string
		setup   func(t *testing.T)
		cleanup func(t *testing.T)
		err     string
	}{
		{
			title: "walkdir error on unreadable directory",
			setup: func(t *testing.T) {
				require.NoError(t, os.Mkdir("noread", 0o000))
			},
			cleanup: func(_ *testing.T) {
				_ = os.Chmod("noread", 0o755)
			},
			err: "open noread: permission denied",
		},
		{
			title: "eval symlinks error on broken symlink",
			setup: func(t *testing.T) {
				require.NoError(t, os.WriteFile("target.txt", []byte("test"), 0o644))
				require.NoError(t, os.Symlink("target.txt", "link.txt"))
				require.NoError(t, os.Remove("target.txt"))
			},
			err: "link.txt is a symlink but target file cannot be evaluated: lstat target.txt: no such file or directory",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			t.Chdir(t.TempDir())
			tc.setup(t)
			if tc.cleanup != nil {
				t.Cleanup(func() { tc.cleanup(t) })
			}

			_, err := findSymlinks()
			require.EqualError(t, err, tc.err)
		})
	}
}

func TestAddSymlinkedEntries(t *testing.T) {
	testCases := []struct {
		setup   func(t *testing.T)
		title   string
		err     string
		entries []*Entry
	}{
		{
			title: "error from findSymlinks",
			setup: func(t *testing.T) {
				require.NoError(t, os.Symlink("/nonexistent/path", "broken.yml"))
			},
			entries: []*Entry{},
			err:     "broken.yml is a symlink but target file cannot be evaluated: lstat /nonexistent: no such file or directory",
		},
		{
			title: "skip removed entry",
			entries: []*Entry{
				{
					State: Removed,
					Path:  Path{Name: "a.yml", SymlinkTarget: "a.yml"},
				},
			},
		},
		{
			title: "skip entry with path error",
			entries: []*Entry{
				{
					State:     Noop,
					Path:      Path{Name: "b.yml", SymlinkTarget: "b.yml"},
					PathError: errors.New("some error"),
				},
			},
		},
		{
			title: "skip entry with rule error",
			entries: []*Entry{
				{
					State: Noop,
					Path:  Path{Name: "c.yml", SymlinkTarget: "c.yml"},
					Rule: parser.Rule{
						Error: parser.ParseError{Err: errors.New("parse error")},
					},
				},
			},
		},
		{
			title: "skip entry that is already a symlink",
			entries: []*Entry{
				{
					State: Noop,
					Path:  Path{Name: "link.yml", SymlinkTarget: "d.yml"},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if tc.setup != nil {
				tc.setup(t)
			}

			result, err := addSymlinkedEntries(tc.entries)
			if tc.err != "" {
				require.EqualError(t, err, tc.err)
			} else {
				require.NoError(t, err)
				require.Empty(t, result)
			}
		})
	}
}
