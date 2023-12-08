package discovery_test

import (
	"encoding/json"
	"errors"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/prometheus/prometheus/model/rulefmt"
	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/git"
	"github.com/cloudflare/pint/internal/parser"
)

func TestGlobPathFinder(t *testing.T) {
	type testCaseT struct {
		files    map[string]string
		symlinks map[string]string
		finder   discovery.GlobFinder
		entries  []discovery.Entry
		err      string
	}

	p := parser.NewParser()
	testRuleBody := "# pint file/owner bob\n\n- record: foo\n  expr: sum(foo)\n"
	testRules, err := p.Parse([]byte(testRuleBody))
	require.NoError(t, err)

	parseErr := func(input string) error {
		_, err := rulefmt.Parse([]byte(input))
		if err == nil {
			panic(input)
		}
		return err[0]
	}

	testCases := []testCaseT{
		{
			files:  map[string]string{},
			finder: discovery.NewGlobFinder([]string{"[]"}, git.NewPathFilter(nil, nil, nil)),
			err:    "failed to expand file path pattern []: syntax error in pattern",
		},
		{
			files:  map[string]string{},
			finder: discovery.NewGlobFinder([]string{"*"}, git.NewPathFilter(nil, nil, nil)),
			err:    "no matching files",
		},
		{
			files:  map[string]string{},
			finder: discovery.NewGlobFinder([]string{"*"}, git.NewPathFilter(nil, nil, nil)),
			err:    "no matching files",
		},
		{
			files:  map[string]string{},
			finder: discovery.NewGlobFinder([]string{"foo/*"}, git.NewPathFilter(nil, nil, nil)),
			err:    "no matching files",
		},
		{
			files:  map[string]string{"bar.yml": testRuleBody},
			finder: discovery.NewGlobFinder([]string{"foo/*"}, git.NewPathFilter(nil, nil, nil)),
			err:    "no matching files",
		},
		{
			files:  map[string]string{"bar.yml": testRuleBody},
			finder: discovery.NewGlobFinder([]string{"*"}, git.NewPathFilter(nil, nil, []*regexp.Regexp{regexp.MustCompile(".*")})),
			entries: []discovery.Entry{
				{
					State:         discovery.Noop,
					ReportedPath:  "bar.yml",
					SourcePath:    "bar.yml",
					Rule:          testRules[0],
					ModifiedLines: testRules[0].Lines(),
					Owner:         "bob",
				},
			},
		},
		{
			files:  map[string]string{"foo/bar.yml": testRuleBody + "\n\n# pint file/owner alice\n"},
			finder: discovery.NewGlobFinder([]string{"*"}, git.NewPathFilter(nil, nil, []*regexp.Regexp{regexp.MustCompile(".*")})),
			entries: []discovery.Entry{
				{
					State:         discovery.Noop,
					ReportedPath:  "foo/bar.yml",
					SourcePath:    "foo/bar.yml",
					Rule:          testRules[0],
					ModifiedLines: testRules[0].Lines(),
					Owner:         "alice",
				},
			},
		},
		{
			files:  map[string]string{"bar.yml": testRuleBody},
			finder: discovery.NewGlobFinder([]string{"*"}, git.NewPathFilter(nil, nil, nil)),
			entries: []discovery.Entry{
				{
					State:         discovery.Noop,
					ReportedPath:  "bar.yml",
					SourcePath:    "bar.yml",
					PathError:     parseErr(testRuleBody),
					ModifiedLines: []int{1, 2, 3, 4},
					Owner:         "bob",
				},
			},
		},
		{
			files:  map[string]string{"bar.yml": "record:::{}\n  expr: sum(foo)\n\n# pint file/owner bob\n"},
			finder: discovery.NewGlobFinder([]string{"*"}, git.NewPathFilter(nil, nil, []*regexp.Regexp{regexp.MustCompile(".*")})),
			entries: []discovery.Entry{
				{
					State:         discovery.Noop,
					ReportedPath:  "bar.yml",
					SourcePath:    "bar.yml",
					PathError:     errors.New("yaml: line 2: mapping values are not allowed in this context"),
					ModifiedLines: []int{1, 2, 3, 4},
					Owner:         "bob",
				},
			},
		},
		{
			files:    map[string]string{"bar.yml": testRuleBody},
			symlinks: map[string]string{"link.yml": "bar.yml"},
			finder:   discovery.NewGlobFinder([]string{"*"}, git.NewPathFilter(nil, nil, nil)),
			entries: []discovery.Entry{
				{
					State:         discovery.Noop,
					ReportedPath:  "bar.yml",
					SourcePath:    "bar.yml",
					PathError:     parseErr(testRuleBody),
					ModifiedLines: []int{1, 2, 3, 4},
					Owner:         "bob",
				},
				{
					State:         discovery.Noop,
					ReportedPath:  "bar.yml",
					SourcePath:    "link.yml",
					PathError:     parseErr(testRuleBody),
					ModifiedLines: []int{1, 2, 3, 4},
					Owner:         "bob",
				},
			},
		},
		{
			files: map[string]string{"a/bar.yml": testRuleBody},
			symlinks: map[string]string{
				"b/link.yml":   "../a/bar.yml",
				"b/c/link.yml": "../../a/bar.yml",
			},
			finder: discovery.NewGlobFinder([]string{"*"}, git.NewPathFilter(nil, nil, nil)),
			entries: []discovery.Entry{
				{
					State:         discovery.Noop,
					ReportedPath:  "a/bar.yml",
					SourcePath:    "a/bar.yml",
					PathError:     parseErr(testRuleBody),
					ModifiedLines: []int{1, 2, 3, 4},
					Owner:         "bob",
				},
				{
					State:         discovery.Noop,
					ReportedPath:  "a/bar.yml",
					SourcePath:    "b/c/link.yml",
					PathError:     parseErr(testRuleBody),
					ModifiedLines: []int{1, 2, 3, 4},
					Owner:         "bob",
				},
				{
					State:         discovery.Noop,
					ReportedPath:  "a/bar.yml",
					SourcePath:    "b/link.yml",
					PathError:     parseErr(testRuleBody),
					ModifiedLines: []int{1, 2, 3, 4},
					Owner:         "bob",
				},
			},
		},
		{
			files: map[string]string{"a/bar.yml": testRuleBody},
			symlinks: map[string]string{
				"b/link.yml":   "../a/bar.yml",
				"b/c/link.yml": "../a/bar.yml",
			},
			finder: discovery.NewGlobFinder([]string{"*"}, git.NewPathFilter(nil, nil, nil)),
			err:    "b/c/link.yml is a symlink but target file cannot be evaluated: lstat b/a: no such file or directory",
		},
		{
			files: map[string]string{"a/bar.yml": "xxx:\n"},
			symlinks: map[string]string{
				"b/c/link.yml": "../../a/bar.yml",
			},
			finder: discovery.NewGlobFinder([]string{"*"}, git.NewPathFilter(nil, nil, []*regexp.Regexp{regexp.MustCompile(".*")})),
		},
		{
			files: map[string]string{"a/bar.yml": "xxx:\nyyy:\n"},
			symlinks: map[string]string{
				"b/c/link.yml": "../../a/bar.yml",
			},
			finder: discovery.NewGlobFinder([]string{"*"}, git.NewPathFilter(nil, nil, nil)),
			entries: []discovery.Entry{
				{
					State:         discovery.Noop,
					ReportedPath:  "a/bar.yml",
					SourcePath:    "a/bar.yml",
					PathError:     parseErr("xxx:\nyyy:\n"),
					ModifiedLines: []int{1, 2},
					Owner:         "",
				},
				{
					State:         discovery.Noop,
					ReportedPath:  "a/bar.yml",
					SourcePath:    "b/c/link.yml",
					PathError:     parseErr("xxx:\nyyy:\n"),
					ModifiedLines: []int{1, 2},
					Owner:         "",
				},
			},
		},
		{
			files: map[string]string{"a/bar.yml": "xxx:\nyyy:\n"},
			symlinks: map[string]string{
				"b/c/link.yml": "../../a/bar.yml",
			},
			finder: discovery.NewGlobFinder([]string{"*"}, git.NewPathFilter(nil, nil, []*regexp.Regexp{regexp.MustCompile(".*")})),
		},
		{
			files: map[string]string{"a/bar.yml": "xxx:\nyyy:\n"},
			symlinks: map[string]string{
				"b/c/d": "../../a",
			},
			finder: discovery.NewGlobFinder([]string{"*"}, git.NewPathFilter(nil, nil, []*regexp.Regexp{regexp.MustCompile(".*")})),
		},
		{
			files: map[string]string{"a/bar.yml": testRuleBody},
			symlinks: map[string]string{
				"b/c/link.yml": "../../a/bar.yml",
			},
			finder: discovery.NewGlobFinder([]string{"*"}, git.NewPathFilter(nil, nil, nil)),
			entries: []discovery.Entry{
				{
					State:         discovery.Noop,
					ReportedPath:  "a/bar.yml",
					SourcePath:    "a/bar.yml",
					PathError:     parseErr(testRuleBody),
					ModifiedLines: []int{1, 2, 3, 4},
					Owner:         "bob",
				},
				{
					State:         discovery.Noop,
					ReportedPath:  "a/bar.yml",
					SourcePath:    "b/c/link.yml",
					PathError:     parseErr(testRuleBody),
					ModifiedLines: []int{1, 2, 3, 4},
					Owner:         "bob",
				},
			},
		},
		{
			symlinks: map[string]string{
				"input.yml": "/xx/ccc/fdd",
			},
			finder: discovery.NewGlobFinder([]string{"*"}, git.NewPathFilter(nil, nil, nil)),
			err:    "input.yml is a symlink but target file cannot be evaluated: lstat /xx: no such file or directory",
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			workdir := t.TempDir()
			err := os.Chdir(workdir)
			require.NoError(t, err)

			for p, content := range tc.files {
				if strings.Contains(p, "/") {
					err = os.MkdirAll(path.Dir(p), 0o755)
					require.NoError(t, err)
				}
				err = os.WriteFile(p, []byte(content), 0o644)
				require.NoError(t, err)
			}
			for symlink, target := range tc.symlinks {
				if strings.Contains(symlink, "/") {
					err = os.MkdirAll(path.Dir(symlink), 0o755)
					require.NoError(t, err)
				}
				require.NoError(t, os.Symlink(target, symlink))
			}

			entries, err := tc.finder.Find()
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
