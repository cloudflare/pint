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

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/git"
	"github.com/cloudflare/pint/internal/parser"
)

func TestGlobPathFinder(t *testing.T) {
	type testCaseT struct {
		files    map[string]string
		symlinks map[string]string
		err      string
		entries  []discovery.Entry
		finder   discovery.GlobFinder
	}

	p := parser.NewParser(false, parser.PrometheusSchema, model.UTF8Validation)
	testRuleBody := "# pint file/owner bob\n\n- record: foo\n  expr: sum(foo)\n"
	testFile, _ := p.Parse(strings.NewReader(testRuleBody))
	require.NoError(t, testFile.Error.Err)

	testCases := []testCaseT{
		{
			files:  map[string]string{},
			finder: discovery.NewGlobFinder([]string{"[]"}, git.NewPathFilter(nil, nil, nil), parser.PrometheusSchema, model.UTF8Validation, nil),
			err:    "failed to expand file path pattern []: syntax error in pattern",
		},
		{
			files:  map[string]string{},
			finder: discovery.NewGlobFinder([]string{"*"}, git.NewPathFilter(nil, nil, nil), parser.PrometheusSchema, model.UTF8Validation, nil),
			err:    "no matching files",
		},
		{
			files:  map[string]string{},
			finder: discovery.NewGlobFinder([]string{"*"}, git.NewPathFilter(nil, nil, nil), parser.PrometheusSchema, model.UTF8Validation, nil),
			err:    "no matching files",
		},
		{
			files:  map[string]string{},
			finder: discovery.NewGlobFinder([]string{"foo/*"}, git.NewPathFilter(nil, nil, nil), parser.PrometheusSchema, model.UTF8Validation, nil),
			err:    "no matching files",
		},
		{
			files:  map[string]string{"bar.yml": testRuleBody},
			finder: discovery.NewGlobFinder([]string{"foo/*"}, git.NewPathFilter(nil, nil, nil), parser.PrometheusSchema, model.UTF8Validation, nil),
			err:    "no matching files",
		},
		{
			files:  map[string]string{"bar.yml": testRuleBody},
			finder: discovery.NewGlobFinder([]string{"*"}, git.NewPathFilter(nil, nil, []*regexp.Regexp{regexp.MustCompile(".*")}), parser.PrometheusSchema, model.UTF8Validation, nil),
			entries: []discovery.Entry{
				{
					State: discovery.Noop,
					Path: discovery.Path{
						Name:          "bar.yml",
						SymlinkTarget: "bar.yml",
					},
					Rule:          testFile.Groups[0].Rules[0],
					ModifiedLines: testFile.Groups[0].Rules[0].Lines.Expand(),
					Owner:         "bob",
				},
			},
		},
		{
			files:  map[string]string{"foo/bar.yml": testRuleBody + "\n\n# pint file/owner alice\n"},
			finder: discovery.NewGlobFinder([]string{"*"}, git.NewPathFilter(nil, nil, []*regexp.Regexp{regexp.MustCompile(".*")}), parser.PrometheusSchema, model.UTF8Validation, nil),
			entries: []discovery.Entry{
				{
					State: discovery.Noop,
					Path: discovery.Path{
						Name:          "foo/bar.yml",
						SymlinkTarget: "foo/bar.yml",
					},
					Rule:          testFile.Groups[0].Rules[0],
					ModifiedLines: testFile.Groups[0].Rules[0].Lines.Expand(),
					Owner:         "alice",
				},
			},
		},
		{
			files:  map[string]string{"bar.yml": testRuleBody},
			finder: discovery.NewGlobFinder([]string{"*"}, git.NewPathFilter(nil, nil, nil), parser.PrometheusSchema, model.UTF8Validation, nil),
			entries: []discovery.Entry{
				{
					State: discovery.Noop,
					Path: discovery.Path{
						Name:          "bar.yml",
						SymlinkTarget: "bar.yml",
					},
					PathError: parser.ParseError{
						Err:  errors.New("YAML list is not allowed here, expected a YAML mapping"),
						Line: 3,
					},
					ModifiedLines: []int{1, 2, 3, 4},
					Owner:         "bob",
				},
			},
		},
		{
			files:  map[string]string{"bar.yml": "record:::{}\n  expr: sum(foo)\n\n# pint file/owner bob\n"},
			finder: discovery.NewGlobFinder([]string{"*"}, git.NewPathFilter(nil, nil, []*regexp.Regexp{regexp.MustCompile(".*")}), parser.PrometheusSchema, model.UTF8Validation, nil),
			entries: []discovery.Entry{
				{
					State: discovery.Noop,
					Path: discovery.Path{
						Name:          "bar.yml",
						SymlinkTarget: "bar.yml",
					},
					PathError: parser.ParseError{
						Err:  errors.New("mapping values are not allowed in this context"),
						Line: 2,
					},
					ModifiedLines: []int{1, 2, 3, 4},
					Owner:         "bob",
				},
			},
		},
		{
			files:    map[string]string{"bar.yml": testRuleBody},
			symlinks: map[string]string{"link.yml": "bar.yml"},
			finder:   discovery.NewGlobFinder([]string{"*"}, git.NewPathFilter(nil, nil, nil), parser.PrometheusSchema, model.UTF8Validation, nil),
			entries: []discovery.Entry{
				{
					State: discovery.Noop,
					Path: discovery.Path{
						Name:          "bar.yml",
						SymlinkTarget: "bar.yml",
					},
					PathError: parser.ParseError{
						Err:  errors.New("YAML list is not allowed here, expected a YAML mapping"),
						Line: 3,
					},
					ModifiedLines: []int{1, 2, 3, 4},
					Owner:         "bob",
				},
				{
					State: discovery.Noop,
					Path: discovery.Path{
						Name:          "link.yml",
						SymlinkTarget: "bar.yml",
					},
					PathError: parser.ParseError{
						Err:  errors.New("YAML list is not allowed here, expected a YAML mapping"),
						Line: 3,
					},
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
			finder: discovery.NewGlobFinder([]string{"*"}, git.NewPathFilter(nil, nil, nil), parser.PrometheusSchema, model.UTF8Validation, nil),
			entries: []discovery.Entry{
				{
					State: discovery.Noop,
					Path: discovery.Path{
						Name:          "a/bar.yml",
						SymlinkTarget: "a/bar.yml",
					},
					PathError: parser.ParseError{
						Err:  errors.New("YAML list is not allowed here, expected a YAML mapping"),
						Line: 3,
					},
					ModifiedLines: []int{1, 2, 3, 4},
					Owner:         "bob",
				},
				{
					State: discovery.Noop,
					Path: discovery.Path{
						Name:          "b/c/link.yml",
						SymlinkTarget: "a/bar.yml",
					},
					PathError: parser.ParseError{
						Err:  errors.New("YAML list is not allowed here, expected a YAML mapping"),
						Line: 3,
					},
					ModifiedLines: []int{1, 2, 3, 4},
					Owner:         "bob",
				},
				{
					State: discovery.Noop,
					Path: discovery.Path{
						Name:          "b/link.yml",
						SymlinkTarget: "a/bar.yml",
					},
					PathError: parser.ParseError{
						Err:  errors.New("YAML list is not allowed here, expected a YAML mapping"),
						Line: 3,
					},
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
			finder: discovery.NewGlobFinder([]string{"*"}, git.NewPathFilter(nil, nil, nil), parser.PrometheusSchema, model.UTF8Validation, nil),
			err:    "b/c/link.yml is a symlink but target file cannot be evaluated: lstat b/a: no such file or directory",
		},
		{
			files: map[string]string{"a/bar.yml": "xxx:\n"},
			symlinks: map[string]string{
				"b/c/link.yml": "../../a/bar.yml",
			},
			finder: discovery.NewGlobFinder([]string{"*"}, git.NewPathFilter(nil, nil, []*regexp.Regexp{regexp.MustCompile(".*")}), parser.PrometheusSchema, model.UTF8Validation, nil),
		},
		{
			files: map[string]string{"a/bar.yml": "xxx:\nyyy:\n"},
			symlinks: map[string]string{
				"b/c/link.yml": "../../a/bar.yml",
			},
			finder: discovery.NewGlobFinder([]string{"*"}, git.NewPathFilter(nil, nil, nil), parser.PrometheusSchema, model.UTF8Validation, nil),
			entries: []discovery.Entry{
				{
					State: discovery.Noop,
					Path: discovery.Path{
						Name:          "a/bar.yml",
						SymlinkTarget: "a/bar.yml",
					},
					PathError: parser.ParseError{
						Err:  errors.New("YAML list is not allowed here, expected a YAML mapping"),
						Line: 1,
					},
					ModifiedLines: []int{1, 2},
					Owner:         "",
				},
				{
					State: discovery.Noop,
					Path: discovery.Path{
						Name:          "b/c/link.yml",
						SymlinkTarget: "a/bar.yml",
					},
					PathError: parser.ParseError{
						Err:  errors.New("YAML list is not allowed here, expected a YAML mapping"),
						Line: 1,
					},
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
			finder: discovery.NewGlobFinder([]string{"*"}, git.NewPathFilter(nil, nil, []*regexp.Regexp{regexp.MustCompile(".*")}), parser.PrometheusSchema, model.UTF8Validation, nil),
		},
		{
			files: map[string]string{"a/bar.yml": "xxx:\nyyy:\n"},
			symlinks: map[string]string{
				"b/c/d": "../../a",
			},
			finder: discovery.NewGlobFinder([]string{"*"}, git.NewPathFilter(nil, nil, []*regexp.Regexp{regexp.MustCompile(".*")}), parser.PrometheusSchema, model.UTF8Validation, nil),
		},
		{
			files: map[string]string{"a/bar.yml": testRuleBody},
			symlinks: map[string]string{
				"b/c/link.yml": "../../a/bar.yml",
			},
			finder: discovery.NewGlobFinder([]string{"*"}, git.NewPathFilter(nil, nil, nil), parser.PrometheusSchema, model.UTF8Validation, nil),
			entries: []discovery.Entry{
				{
					State: discovery.Noop,
					Path: discovery.Path{
						Name:          "a/bar.yml",
						SymlinkTarget: "a/bar.yml",
					},
					PathError: parser.ParseError{
						Err:  errors.New("YAML list is not allowed here, expected a YAML mapping"),
						Line: 3,
					},
					ModifiedLines: []int{1, 2, 3, 4},
					Owner:         "bob",
				},
				{
					State: discovery.Noop,
					Path: discovery.Path{
						Name:          "b/c/link.yml",
						SymlinkTarget: "a/bar.yml",
					},
					PathError: parser.ParseError{
						Err:  errors.New("YAML list is not allowed here, expected a YAML mapping"),
						Line: 3,
					},
					ModifiedLines: []int{1, 2, 3, 4},
					Owner:         "bob",
				},
			},
		},
		{
			symlinks: map[string]string{
				"input.yml": "/xx/ccc/fdd",
			},
			finder: discovery.NewGlobFinder([]string{"*"}, git.NewPathFilter(nil, nil, nil), parser.PrometheusSchema, model.UTF8Validation, nil),
			err:    "input.yml is a symlink but target file cannot be evaluated: lstat /xx: no such file or directory",
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			workdir := t.TempDir()
			t.Chdir(workdir)

			for p, content := range tc.files {
				if strings.Contains(p, "/") {
					err := os.MkdirAll(path.Dir(p), 0o755)
					require.NoError(t, err)
				}
				err := os.WriteFile(p, []byte(content), 0o644)
				require.NoError(t, err)
			}
			for symlink, target := range tc.symlinks {
				if strings.Contains(symlink, "/") {
					err := os.MkdirAll(path.Dir(symlink), 0o755)
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
