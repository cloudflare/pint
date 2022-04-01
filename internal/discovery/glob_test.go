package discovery_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

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

	testCases := []testCaseT{
		{
			files:  map[string]string{},
			finder: discovery.NewGlobFinder("[]"),
			err:    filepath.ErrBadPattern,
		},
		{
			files:  map[string]string{},
			finder: discovery.NewGlobFinder("*"),
			err:    fmt.Errorf("no matching files"),
		},
		{
			files:  map[string]string{},
			finder: discovery.NewGlobFinder("*"),
			err:    fmt.Errorf("no matching files"),
		},
		{
			files:  map[string]string{},
			finder: discovery.NewGlobFinder("foo/*"),
			err:    fmt.Errorf("no matching files"),
		},
		{
			files:  map[string]string{"bar.yml": testRuleBody},
			finder: discovery.NewGlobFinder("foo/*"),
			err:    fmt.Errorf("no matching files"),
		},
		{
			files:  map[string]string{"bar.yml": testRuleBody},
			finder: discovery.NewGlobFinder("*"),
			entries: []discovery.Entry{
				{
					Path:  "bar.yml",
					Rule:  testRules[0],
					Owner: "bob",
				},
			},
		},
		{
			files:  map[string]string{"foo/bar.yml": testRuleBody + "\n\n# pint file/owner alice\n"},
			finder: discovery.NewGlobFinder("*"),
			entries: []discovery.Entry{
				{
					Path:  "foo/bar.yml",
					Rule:  testRules[0],
					Owner: "alice",
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
				err = ioutil.WriteFile(p, []byte(content), 0o644)
				require.NoError(t, err)
			}

			entries, err := tc.finder.Find()
			require.Equal(t, tc.err, err)
			require.Equal(t, tc.entries, entries)
		})
	}
}
