package discovery_test

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/prometheus/prometheus/model/rulefmt"
	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
)

func TestGlobPathFinder(t *testing.T) {
	type testCaseT struct {
		files   map[string]string
		finder  discovery.GlobFinder
		entries []discovery.Entry
		err     error
	}

	p := parser.NewParser()
	testRuleBody := "# pint file/owner bob\n\n- record: foo\n  expr: sum(foo)\n"
	testRules, err := p.Parse([]byte(testRuleBody))
	require.NoError(t, err)

	_, strictErrs := rulefmt.Parse([]byte(testRuleBody))

	testCases := []testCaseT{
		{
			files:  map[string]string{},
			finder: discovery.NewGlobFinder([]string{"[]"}, nil),
			err:    filepath.ErrBadPattern,
		},
		{
			files:  map[string]string{},
			finder: discovery.NewGlobFinder([]string{"*"}, nil),
			err:    fmt.Errorf("no matching files"),
		},
		{
			files:  map[string]string{},
			finder: discovery.NewGlobFinder([]string{"*"}, nil),
			err:    fmt.Errorf("no matching files"),
		},
		{
			files:  map[string]string{},
			finder: discovery.NewGlobFinder([]string{"foo/*"}, nil),
			err:    fmt.Errorf("no matching files"),
		},
		{
			files:  map[string]string{"bar.yml": testRuleBody},
			finder: discovery.NewGlobFinder([]string{"foo/*"}, nil),
			err:    fmt.Errorf("no matching files"),
		},
		{
			files:  map[string]string{"bar.yml": testRuleBody},
			finder: discovery.NewGlobFinder([]string{"*"}, []*regexp.Regexp{regexp.MustCompile(".*")}),
			entries: []discovery.Entry{
				{
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
			finder: discovery.NewGlobFinder([]string{"*"}, []*regexp.Regexp{regexp.MustCompile(".*")}),
			entries: []discovery.Entry{
				{
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
			finder: discovery.NewGlobFinder([]string{"*"}, nil),
			entries: []discovery.Entry{
				{
					ReportedPath:  "bar.yml",
					SourcePath:    "bar.yml",
					PathError:     strictErrs[0],
					ModifiedLines: []int{1, 2, 3, 4},
					Owner:         "bob",
				},
			},
		},
		{
			files:  map[string]string{"bar.yml": "record:::{}\n  expr: sum(foo)\n\n# pint file/owner bob\n"},
			finder: discovery.NewGlobFinder([]string{"*"}, []*regexp.Regexp{regexp.MustCompile(".*")}),
			entries: []discovery.Entry{
				{
					ReportedPath:  "bar.yml",
					SourcePath:    "bar.yml",
					PathError:     errors.New("yaml: line 2: mapping values are not allowed in this context"),
					ModifiedLines: []int{1, 2, 3, 4},
					Owner:         "bob",
				},
			},
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

			entries, err := tc.finder.Find()
			require.Equal(t, tc.err, err)
			require.Equal(t, tc.entries, entries)
		})
	}
}
