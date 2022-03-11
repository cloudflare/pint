package discovery_test

import (
	"fmt"
	"regexp"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/discovery"
)

func TestModifiedFiles(t *testing.T) {
	type testCaseT struct {
		detector    discovery.FileFinder
		output      []discovery.File
		shouldError bool
	}

	testCases := []testCaseT{
		{
			detector: discovery.NewGitBranchFileFinder(func(args ...string) ([]byte, error) {
				return nil, fmt.Errorf("mock error")
			}, nil, "main"),
			output:      nil,
			shouldError: true,
		},
		{
			detector: discovery.NewGitBranchFileFinder(func(args ...string) ([]byte, error) {
				if args[0] == "rev-list" {
					return []byte("commit1\ncommit2\n"), nil
				}
				return nil, fmt.Errorf("mock error")
			}, nil, "main"),
			output:      nil,
			shouldError: true,
		},
		{
			detector: discovery.NewGitBranchFileFinder(func(args ...string) ([]byte, error) {
				if args[0] == "rev-list" {
					return []byte("commit1\ncommit3\n"), nil
				}
				content := `commit1
M       foo/bar/1.txt
M       foo/bar/3.txt
commit2
M       5.txt
M       foo/bar/2.txt
commit3

A       foo/bar/4.txt
R053    src1.txt        dst1.txt
R100    foo/bar/src2.txt        src2.txt
M       5.txt
C50     foo/bar/cp1.txt         foo/cp1.txt
`
				return []byte([]byte(content)), nil
			}, nil, "main"),
			output: []discovery.File{
				{Path: "5.txt", Commits: []string{"commit2", "commit3"}},
				{Path: "dst1.txt", Commits: []string{"commit3"}},
				{Path: "foo/bar/1.txt", Commits: []string{"commit1"}},
				{Path: "foo/bar/2.txt", Commits: []string{"commit2"}},
				{Path: "foo/bar/3.txt", Commits: []string{"commit1"}},
				{Path: "foo/bar/4.txt", Commits: []string{"commit3"}},
				{Path: "foo/cp1.txt", Commits: []string{"commit3"}},
				{Path: "src2.txt", Commits: []string{"commit3"}},
			},
		},
		{
			detector: discovery.NewGitBranchFileFinder(func(args ...string) ([]byte, error) {
				if args[0] == "rev-list" {
					return []byte("commit1\ncommit3\n"), nil
				}
				content := `commit1
M       foo/1.txt
M       3.txt
commit2
M       5.txt
M       bar/2.txt
commit3

A       xxx/bar/4.txt
R053    src1.txt        dst1.txt
R100    foo/bar/src2.txt        src2.txt
M       5.txt
C50     foo/bar/cp1.txt         foo/cp1.del
`
				return []byte([]byte(content)), nil
			}, []*regexp.Regexp{
				regexp.MustCompile("^foo/.+.txt$"),
				regexp.MustCompile("^bar/.+.txt$"),
			}, "main"),
			output: []discovery.File{
				{Path: "bar/2.txt", Commits: []string{"commit2"}},
				{Path: "foo/1.txt", Commits: []string{"commit1"}},
			},
		},
		{
			detector: discovery.NewGitBranchFileFinder(func(args ...string) ([]byte, error) {
				if args[0] == "rev-list" {
					return []byte("commit1\ncommit3\n"), nil
				}
				content := `commit1
M       file1
M       file2

commit2
M       file1
M       file3

commit3
R100    file1  file4
M       file2
`
				return []byte([]byte(content)), nil
			}, nil, "main"),
			output: []discovery.File{
				{Path: "file2", Commits: []string{"commit1", "commit3"}},
				{Path: "file3", Commits: []string{"commit2"}},
				{Path: "file4", Commits: []string{"commit1", "commit2", "commit3"}},
			},
		},
		{
			detector: discovery.NewGitBranchFileFinder(func(args ...string) ([]byte, error) {
				if args[0] == "rev-list" {
					return []byte("commit1\ncommit3\n"), nil
				}
				content := `commit1
A       file1

commit2
A       file2

commit3
D       file1
`
				return []byte([]byte(content)), nil
			}, nil, "main"),
			output: []discovery.File{
				{Path: "file2", Commits: []string{"commit2"}},
			},
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			output, err := tc.detector.Find()

			hadError := (err != nil)
			if hadError != tc.shouldError {
				t.Errorf("git.ModifiedFiles() returned err=%v, expected=%v", err, tc.shouldError)
				return
			}

			if hadError {
				return
			}

			require.Equal(t, tc.output, output.Results(), "git.ModifiedFiles() returned wrong output")
		})
	}
}
