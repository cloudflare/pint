package git_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/git"
)

func blameLine(sha string, line int, filename, content string) string {
	return fmt.Sprintf(`%s %d %d 1
author Alice Mock
author-mail <alice@example.com>
author-time 1559927997
author-tz 0000
committer Alice Mock
committer-mail <alice@example.com>
committer-time 1559927997
committer-tz 0000
summary Mock commit title
boundary
filename %s
	%s
`, sha, line, line, filename, content)
}

func TestGitBlame(t *testing.T) {
	type testCaseT struct {
		mock        git.CommandRunner
		path        string
		output      git.LineBlames
		shouldError bool
	}

	testCases := []testCaseT{
		{
			mock: func(args ...string) ([]byte, error) {
				return nil, nil
			},
			path:   "foo.txt",
			output: nil,
		},
		{
			mock: func(args ...string) ([]byte, error) {
				content := blameLine("b33a88cea35abc47f9973983626e1c6f3f3abc44", 1, "foo.txt", "")
				return []byte(content), nil
			},
			path: "foo.txt",
			output: git.LineBlames{
				{
					Filename: "foo.txt",
					Line:     1,
					Commit:   "b33a88cea35abc47f9973983626e1c6f3f3abc44",
				},
			},
		},
		{
			mock: func(args ...string) ([]byte, error) {
				content := blameLine("b33a88cea35abc47f9973983626e1c6f3f3abc44", 1, "foo.txt", "") +
					blameLine("b33a88cea35abc47f9973983626e1c6f3f3abc44", 2, "foo.txt", "") +
					blameLine("82987dec74ba8e434ba393d83491ace784473291", 3, "foo.txt", "") +
					blameLine("b33a88cea35abc47f9973983626e1c6f3f3abc44", 4, "bar.txt", "")
				return []byte(content), nil
			},
			path: "foo.txt",
			output: git.LineBlames{
				{
					Filename: "foo.txt",
					Line:     1,
					Commit:   "b33a88cea35abc47f9973983626e1c6f3f3abc44",
				},
				{Filename: "foo.txt", Line: 2, Commit: "b33a88cea35abc47f9973983626e1c6f3f3abc44"},
				{Filename: "foo.txt", Line: 3, Commit: "82987dec74ba8e434ba393d83491ace784473291"},
			},
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			output, err := git.Blame(tc.path, tc.mock)

			hadError := (err != nil)
			if hadError != tc.shouldError {
				t.Errorf("git.Blame() returned err=%v, expected=%v", err, tc.shouldError)
				return
			}

			require.Equal(t, tc.output, output, "git.Blame() returned wrong output ")
		})
	}
}

func TestCommitRange(t *testing.T) {
	type testCaseT struct {
		mock        git.CommandRunner
		output      git.CommitRangeResults
		shouldError bool
	}

	testCases := []testCaseT{
		{
			mock: func(args ...string) ([]byte, error) {
				return nil, fmt.Errorf("mock error")
			},
			output:      git.CommitRangeResults{},
			shouldError: true,
		},
		{
			mock: func(args ...string) ([]byte, error) {
				return []byte([]byte("")), nil
			},
			output:      git.CommitRangeResults{},
			shouldError: true,
		},
		{
			mock: func(args ...string) ([]byte, error) {
				return []byte([]byte("commit1\n")), nil
			},
			output: git.CommitRangeResults{
				From: "commit1",
				To:   "commit1",
			},
		},
		{
			mock: func(args ...string) ([]byte, error) {
				return []byte([]byte("commit1\ncommit2\ncommit3\n")), nil
			},
			output: git.CommitRangeResults{
				From: "commit1",
				To:   "commit3",
			},
		},
		{
			mock: func(args ...string) ([]byte, error) {
				return []byte("commit2\ncommit1"), nil
			},
			output: git.CommitRangeResults{
				From: "commit2",
				To:   "commit1",
			},
		},
		{
			mock: func(args ...string) ([]byte, error) {
				return []byte("commit2\ncommit1\n"), nil
			},
			output: git.CommitRangeResults{
				From: "commit2",
				To:   "commit1",
			},
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			output, err := git.CommitRange(tc.mock, "main")

			hadError := (err != nil)
			if hadError != tc.shouldError {
				t.Errorf("git.CommitRange() returned err=%v, expected=%v", err, tc.shouldError)
				return
			}

			require.Equal(t, tc.output, output, "git.CommitRange() returned wrong output")
		})
	}
}

func TestCurrentBranch(t *testing.T) {
	type testCaseT struct {
		mock        git.CommandRunner
		output      string
		shouldError bool
	}

	testCases := []testCaseT{
		{
			mock: func(args ...string) ([]byte, error) {
				return nil, fmt.Errorf("mock error")
			},
			output:      "",
			shouldError: true,
		},
		{
			mock: func(args ...string) ([]byte, error) {
				return []byte([]byte("")), nil
			},
			output:      "",
			shouldError: false,
		},
		{
			mock: func(args ...string) ([]byte, error) {
				return []byte([]byte("foo")), nil
			},
			output: "foo",
		},
		{
			mock: func(args ...string) ([]byte, error) {
				return []byte([]byte("foo bar\n")), nil
			},
			output: "foo bar",
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			output, err := git.CurrentBranch(tc.mock)

			hadError := (err != nil)
			if hadError != tc.shouldError {
				t.Errorf("git.CurrentBranch() returned err=%v, expected=%v", err, tc.shouldError)
				return
			}

			require.Equal(t, tc.output, output, "git.CurrentBranch() returned wrong output")
		})
	}
}
