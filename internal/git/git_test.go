package git_test

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/git"
)

func TestCurrentBranch(t *testing.T) {
	type testCaseT struct {
		mock        git.CommandRunner
		output      string
		shouldError bool
	}

	testCases := []testCaseT{
		{
			mock: func(_ ...string) ([]byte, error) {
				return nil, errors.New("mock error")
			},
			output:      "",
			shouldError: true,
		},
		{
			mock: func(_ ...string) ([]byte, error) {
				return []byte(""), nil
			},
			output:      "",
			shouldError: false,
		},
		{
			mock: func(_ ...string) ([]byte, error) {
				return []byte("foo"), nil
			},
			output: "foo",
		},
		{
			mock: func(_ ...string) ([]byte, error) {
				return []byte("foo bar\n"), nil
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

func TestRunGit(t *testing.T) {
	type testCaseT struct {
		output *regexp.Regexp
		err    string
		args   []string
	}

	testCases := []testCaseT{
		{
			args:   []string{"version"},
			output: regexp.MustCompile("^git version"),
		},
		{
			args: []string{"xxx"},
			err:  "git: 'xxx' is not a git command. See 'git --help'.\n",
		},
		{
			// config --get with non-existent key exits with 1 but no stderr
			args: []string{"config", "--get", "nonexistent.key.that.does.not.exist"},
			err:  "exit status 1",
		},
	}

	for _, tc := range testCases {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			output, err := git.RunGit(tc.args...)
			if tc.err != "" {
				require.EqualError(t, err, tc.err)
			} else {
				require.NoError(t, err)
				require.Regexp(t, tc.output, string(output))
			}
		})
	}
}

func TestHeadCommit(t *testing.T) {
	type testCaseT struct {
		mock        git.CommandRunner
		output      string
		shouldError bool
	}

	testCases := []testCaseT{
		{
			mock: func(_ ...string) ([]byte, error) {
				return nil, errors.New("mock error")
			},
			output:      "",
			shouldError: true,
		},
		{
			mock: func(_ ...string) ([]byte, error) {
				return []byte("abc123\n"), nil
			},
			output: "abc123",
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			output, err := git.HeadCommit(tc.mock)

			hadError := (err != nil)
			if hadError != tc.shouldError {
				t.Errorf("git.HeadCommit() returned err=%v, expected=%v", err, tc.shouldError)
				return
			}

			require.Equal(t, tc.output, output, "git.HeadCommit() returned wrong output")
		})
	}
}

func TestCommitMessage(t *testing.T) {
	type testCaseT struct {
		mock        git.CommandRunner
		output      string
		shouldError bool
	}

	testCases := []testCaseT{
		{
			mock: func(_ ...string) ([]byte, error) {
				return nil, errors.New("mock error")
			},
			output:      "",
			shouldError: true,
		},
		{
			mock: func(_ ...string) ([]byte, error) {
				return []byte(""), nil
			},
			output:      "",
			shouldError: false,
		},
		{
			mock: func(_ ...string) ([]byte, error) {
				return []byte("foo"), nil
			},
			output: "foo",
		},
		{
			mock: func(_ ...string) ([]byte, error) {
				return []byte("foo bar\n"), nil
			},
			output: "foo bar\n",
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			output, err := git.CommitMessage(tc.mock, "abc1234567890")

			hadError := (err != nil)
			if hadError != tc.shouldError {
				t.Errorf("git.CommitMessage() returned err=%v, expected=%v", err, tc.shouldError)
				return
			}

			require.Equal(t, tc.output, output, "git.CommitMessage() returned wrong output")
		})
	}
}
