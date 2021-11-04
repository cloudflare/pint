package discovery_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/cloudflare/pint/internal/discovery"

	"github.com/google/go-cmp/cmp"
)

func TestGlobFileFinder(t *testing.T) {
	type testCaseT struct {
		files       []string
		pattern     string
		output      []discovery.File
		shouldError bool
	}

	testCases := []testCaseT{
		{
			files:       []string{},
			pattern:     "*",
			output:      nil,
			shouldError: false,
		},
		{
			files:       []string{},
			pattern:     "xxx",
			output:      nil,
			shouldError: false,
		},
		{
			files:       []string{"1.txt"},
			pattern:     "*",
			output:      []discovery.File{{Path: "1.txt"}},
			shouldError: false,
		},
		{
			files:       []string{"1.txt", "2.txt"},
			pattern:     "*.txt",
			output:      []discovery.File{{Path: "1.txt"}, {Path: "2.txt"}},
			shouldError: false,
		},
		{
			files:       []string{"1.txt", "2/2.txt"},
			pattern:     "*",
			output:      []discovery.File{{Path: "1.txt"}, {Path: "2/2.txt"}},
			shouldError: false,
		},
		{
			files:       []string{"1.txt", "2/2.txt", "2/2.foo", "3/3.txt"},
			pattern:     "2/*.txt",
			output:      []discovery.File{{Path: "2/2.txt"}},
			shouldError: false,
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			tmpDir, err := ioutil.TempDir("", "TestGlobFileFinder")
			if err != nil {
				t.Errorf("ioutil.TempDir() returned an error: %s", err)
				return
			}
			defer func() {
				err := os.RemoveAll(tmpDir)
				if err != nil {
					panic(err)
				}
			}()

			for _, path := range tc.files {
				fullpath := filepath.Join(tmpDir, path)
				if dir := filepath.Dir(fullpath); dir != tmpDir {
					if _, err := os.Stat(dir); os.IsNotExist(err) {
						if err := os.Mkdir(dir, 0755); err != nil {
							t.Errorf("os.Mkdir(%s) returned an error: %s", dir, err)
							return
						}

					}
				}
				if _, err := os.Create(fullpath); err != nil {
					t.Errorf("os.Create(%s) returned an error: %s", path, err)
					return
				}
			}

			err = os.Chdir(tmpDir)
			if err != nil {
				t.Errorf("os.Chdir(%s) returned an error: %s", tmpDir, err)
			}

			gd := discovery.NewGlobFileFinder()
			output, err := gd.Find(tc.pattern)
			hadError := err != nil
			if hadError != tc.shouldError {
				t.Errorf("GlobFileFinder.Discover() returned err=%s, expected=%v", err, tc.shouldError)
			}

			if hadError {
				return
			}

			if diff := cmp.Diff(tc.output, output.Results()); diff != "" {
				t.Errorf("GlobFileFinder.Discover() returned wrong output (-want +got):\n%s", diff)
			}
		})
	}
}
