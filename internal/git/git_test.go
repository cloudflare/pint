package git_test

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/git"
)

func TestDescribe(t *testing.T) {
	type testCaseT struct {
		name     string
		mock     git.CommandRunner
		expected git.Info
		err      string
	}

	testCases := []testCaseT{
		{
			// First git command (rev-parse --verify HEAD) fails.
			name: "head commit command fails",
			mock: func(_ context.Context, _ ...string) ([]byte, error) {
				return nil, errors.New("mock error")
			},
			err: "mock error",
		},
		{
			// First command succeeds, second (rev-parse --abbrev-ref HEAD) fails.
			name: "current branch command fails",
			mock: func() git.CommandRunner {
				var calls int
				return func(_ context.Context, _ ...string) ([]byte, error) {
					calls++
					if calls == 1 {
						return []byte("abc123\n"), nil
					}
					return nil, errors.New("branch error")
				}
			}(),
			err: "branch error",
		},
		{
			// Both commands succeed.
			name: "both commands succeed",
			mock: func() git.CommandRunner {
				var calls int
				return func(_ context.Context, _ ...string) ([]byte, error) {
					calls++
					if calls == 1 {
						return []byte("abc123\n"), nil
					}
					return []byte("feature/foo\n"), nil
				}
			}(),
			expected: git.Info{
				HeadCommit:    "abc123",
				CurrentBranch: "feature/foo",
			},
		},
		{
			// Both commands succeed with empty output.
			name: "both commands succeed with empty output",
			mock: func(_ context.Context, _ ...string) ([]byte, error) {
				return []byte(""), nil
			},
			expected: git.Info{
				HeadCommit:    "",
				CurrentBranch: "",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			info, err := git.Describe(t.Context(), tc.mock)
			if tc.err != "" {
				require.EqualError(t, err, tc.err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expected, info)
			}
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
			output, err := git.RunGit(t.Context(), tc.args...)
			if tc.err != "" {
				require.EqualError(t, err, tc.err)
			} else {
				require.NoError(t, err)
				require.Regexp(t, tc.output, string(output))
			}
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
			mock: func(_ context.Context, _ ...string) ([]byte, error) {
				return nil, errors.New("mock error")
			},
			output:      "",
			shouldError: true,
		},
		{
			mock: func(_ context.Context, _ ...string) ([]byte, error) {
				return []byte(""), nil
			},
			output:      "",
			shouldError: false,
		},
		{
			mock: func(_ context.Context, _ ...string) ([]byte, error) {
				return []byte("foo"), nil
			},
			output: "foo",
		},
		{
			mock: func(_ context.Context, _ ...string) ([]byte, error) {
				return []byte("foo bar\n"), nil
			},
			output: "foo bar\n",
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			output, err := git.CommitMessage(t.Context(), tc.mock, "abc1234567890")

			hadError := (err != nil)
			if hadError != tc.shouldError {
				t.Errorf("git.CommitMessage() returned err=%v, expected=%v", err, tc.shouldError)
				return
			}

			require.Equal(t, tc.output, output, "git.CommitMessage() returned wrong output")
		})
	}
}
