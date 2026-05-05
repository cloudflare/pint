package discovery

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/parser"
)

func TestReadRules(t *testing.T) {
	mustParse := func(offset int, s string) parser.Rule {
		p := parser.NewParser(parser.DefaultOptions)
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
		sourceFunc    func(t *testing.T) io.Reader
		check         func(t *testing.T, entries []*Entry)
		title         string
		reportedPath  string
		sourcePath    string
		entries       []Entry
		allowedOwners []*regexp.Regexp
		isStrict      bool
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
					State: Noop,
					Path: Path{
						Name:          "rules.yml",
						SymlinkTarget: "rules.yml",
					},
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
					State: Noop,
					Path: Path{
						Name:          "rules.yml",
						SymlinkTarget: "rules.yml",
					},
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
					State: Noop,
					Path: Path{
						Name:          "rules.yml",
						SymlinkTarget: "rules.yml",
					},
					Rule: mustParse(3, "- record: foo\n  expr: bar\n"),
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
					State: Noop,
					Path: Path{
						Name:          "rules.yml",
						SymlinkTarget: "rules.yml",
					},
					Rule: mustParse(6, "  - record: foo\n    expr: bar\n"),
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
					State: Noop,
					Path: Path{
						Name:          "rules.yml",
						SymlinkTarget: "rules.yml",
					},
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
					State: Noop,
					Path: Path{
						Name:          "rules.yml",
						SymlinkTarget: "rules.yml",
					},
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
					State: Noop,
					Path: Path{
						Name:          "rules.yml",
						SymlinkTarget: "rules.yml",
					},
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
					State: Noop,
					Path: Path{
						Name:          "rules.yml",
						SymlinkTarget: "rules.yml",
					},
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
			title:         "invalid owner",
			reportedPath:  "rules.yml",
			sourcePath:    "rules.yml",
			allowedOwners: []*regexp.Regexp{regexp.MustCompile("^team-")},
			sourceFunc: func(_ *testing.T) io.Reader {
				return bytes.NewBufferString("# pint file/owner bob\n")
			},
			check: func(t *testing.T, entries []*Entry) {
				t.Helper()
				require.Len(t, entries, 1)
				require.Error(t, entries[0].PathError)
				require.Contains(t, entries[0].PathError.Error(), "doesn't match any of the allowed owner values")
			},
		},
		{
			title:        "invalid comment",
			reportedPath: "rules.yml",
			sourcePath:   "rules.yml",
			sourceFunc: func(_ *testing.T) io.Reader {
				return bytes.NewBufferString("# pint file/owner\n\n- record: foo\n  expr: sum(foo)\n")
			},
			check: func(t *testing.T, entries []*Entry) {
				t.Helper()
				var found bool
				for _, e := range entries {
					if e.PathError != nil {
						found = true
						require.Contains(t, e.PathError.Error(), "pint control comment")
					}
				}
				require.True(t, found, "expected at least one entry with PathError from invalid comment")
			},
		},
		{
			title:        "group error",
			reportedPath: "rules.yml",
			sourcePath:   "rules.yml",
			isStrict:     true,
			sourceFunc: func(_ *testing.T) io.Reader {
				return bytes.NewBufferString("groups:\n- name: foo\n  interval: bad\n  rules:\n  - record: foo\n    expr: bar\n")
			},
			check: func(t *testing.T, entries []*Entry) {
				t.Helper()
				var found bool
				for _, e := range entries {
					if e.PathError != nil {
						found = true
					}
				}
				require.True(t, found, "expected at least one entry with PathError from group error")
			},
		},
		{
			title:        "rule owner override",
			reportedPath: "rules.yml",
			sourcePath:   "rules.yml",
			sourceFunc: func(_ *testing.T) io.Reader {
				return bytes.NewBufferString("# pint file/owner alice\n\n- record: foo\n  # pint rule/owner bob\n  expr: sum(foo)\n")
			},
			check: func(t *testing.T, entries []*Entry) {
				t.Helper()
				require.Len(t, entries, 1)
				require.Equal(t, "bob", entries[0].Owner)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(
			fmt.Sprintf("rPath=%s sPath=%s strict=%v title=%s", tc.reportedPath, tc.sourcePath, tc.isStrict, tc.title),
			func(t *testing.T) {
				r := tc.sourceFunc(t)
				p := parser.NewParser(parser.DefaultOptions.WithStrict(tc.isStrict))
				entries := readRules(tc.reportedPath, tc.sourcePath, r, p, tc.allowedOwners, nil)
				if tc.check != nil {
					tc.check(t, entries)
				} else {
					expected, err := json.MarshalIndent(tc.entries, "", "  ")
					require.NoError(t, err, "json(expected)")
					got, err := json.MarshalIndent(entries, "", "  ")
					require.NoError(t, err, "json(got)")
					require.Equal(t, string(expected), string(got))
				}
			})
	}
}

func TestIsValidOwner(t *testing.T) {
	type testCaseT struct {
		name     string
		owner    string
		valid    []*regexp.Regexp
		expected bool
	}

	testCases := []testCaseT{
		{
			name:     "no restrictions",
			owner:    "anyone",
			expected: true,
		},
		{
			name:     "matching owner",
			owner:    "team-sre",
			valid:    []*regexp.Regexp{regexp.MustCompile("^team-")},
			expected: true,
		},
		{
			name:     "non-matching owner",
			owner:    "bob",
			valid:    []*regexp.Regexp{regexp.MustCompile("^team-")},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, isValidOwner(tc.owner, tc.valid))
		})
	}
}

func TestEntryLabels(t *testing.T) {
	makeKey := func() *parser.YamlNode {
		return &parser.YamlNode{Value: "labels"}
	}
	makeItems := func(keys ...string) []*parser.YamlKeyValue {
		items := make([]*parser.YamlKeyValue, len(keys))
		for i, k := range keys {
			items[i] = &parser.YamlKeyValue{
				Key:   &parser.YamlNode{Value: k},
				Value: &parser.YamlNode{Value: "v"},
			}
		}
		return items
	}

	type testCaseT struct {
		name     string
		entry    Entry
		expected int
	}

	testCases := []testCaseT{
		{
			name:     "empty entry",
			entry:    Entry{},
			expected: 0,
		},
		{
			name: "alerting rule labels only",
			entry: Entry{
				Rule: parser.Rule{
					AlertingRule: &parser.AlertingRule{
						Labels: &parser.YamlMap{Key: makeKey(), Items: makeItems("severity")},
					},
				},
			},
			expected: 1,
		},
		{
			name: "alerting rule labels with group",
			entry: Entry{
				Rule: parser.Rule{
					AlertingRule: &parser.AlertingRule{
						Labels: &parser.YamlMap{Key: makeKey(), Items: makeItems("severity")},
					},
				},
				Group: &parser.Group{
					Labels: &parser.YamlMap{Key: makeKey(), Items: makeItems("env")},
				},
			},
			expected: 2,
		},
		{
			name: "recording rule labels only",
			entry: Entry{
				Rule: parser.Rule{
					RecordingRule: &parser.RecordingRule{
						Labels: &parser.YamlMap{Key: makeKey(), Items: makeItems("job")},
					},
				},
			},
			expected: 1,
		},
		{
			name: "recording rule labels with group",
			entry: Entry{
				Rule: parser.Rule{
					RecordingRule: &parser.RecordingRule{
						Labels: &parser.YamlMap{Key: makeKey(), Items: makeItems("job")},
					},
				},
				Group: &parser.Group{
					Labels: &parser.YamlMap{Key: makeKey(), Items: makeItems("env")},
				},
			},
			expected: 2,
		},
		{
			name: "group labels only",
			entry: Entry{
				Rule: parser.Rule{},
				Group: &parser.Group{
					Labels: &parser.YamlMap{Key: makeKey(), Items: makeItems("env")},
				},
			},
			expected: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ym := tc.entry.Labels()
			require.Len(t, ym.Items, tc.expected)
		})
	}
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
		setup     func(t *testing.T)
		title     string
		err       string
		entries   []*Entry
		resultLen int
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
		{
			title: "symlink matches entry",
			setup: func(t *testing.T) {
				require.NoError(t, os.WriteFile("real.yml", []byte("test"), 0o644))
				require.NoError(t, os.Symlink("real.yml", "link.yml"))
			},
			entries: []*Entry{
				{
					State: Noop,
					Path:  Path{Name: "real.yml", SymlinkTarget: "real.yml"},
					Owner: "alice",
				},
			},
			resultLen: 1,
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
				require.Len(t, result, tc.resultLen)
			}
		})
	}
}
