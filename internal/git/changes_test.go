package git_test

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/neilotoole/slogt"
	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/git"
)

func debugGitRun(t *testing.T) git.CommandRunner {
	return func(args ...string) ([]byte, error) {
		out, err := git.RunGit(args...)
		if err == nil {
			if len(out) == 0 {
				t.Logf("%s ~> no stdout", strings.Join(args, " "))
			} else {
				t.Logf("%s\n---\n%s---", strings.Join(args, " "), string(out))
			}
		}
		return out, err
	}
}

func mustRun(t *testing.T, args ...string) {
	_, err := debugGitRun(t)(args...)
	require.NoError(t, err, strings.Join(args, " "))
}

func gitCommit(t *testing.T, message string) {
	t.Setenv("GIT_AUTHOR_NAME", "pint")
	t.Setenv("GIT_AUTHOR_EMAIL", "pint@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "pint")
	t.Setenv("GIT_COMMITTER_EMAIL", "pint")
	mustRun(t, "commit", "-am", fmt.Sprintf("commit %s", message))
}

func TestChanges(t *testing.T) {
	type testCaseT struct {
		setup   func(t *testing.T) (git.CommandRunner, git.CommitRangeResults)
		title   string
		err     string
		changes []*git.FileChange
	}

	testCases := []testCaseT{
		{
			title: "git log error",
			setup: func(_ *testing.T) (git.CommandRunner, git.CommitRangeResults) {
				cmd := func(args ...string) ([]byte, error) {
					return nil, fmt.Errorf("mock git error: %v", args)
				}
				cr := git.CommitRangeResults{From: "a", To: "b"}
				return cmd, cr
			},
			changes: nil,
			err:     "failed to get the list of modified files from git: mock git error: [log --reverse --no-merges --first-parent --format=%H --name-status a^..b]",
		},
		{
			title: "chmod",
			setup: func(t *testing.T) (git.CommandRunner, git.CommitRangeResults) {
				mustRun(t, "init", "--initial-branch=main", ".")
				require.NoError(t, os.WriteFile("index.txt", []byte("foo"), 0o644))
				mustRun(t, "add", "index.txt")
				gitCommit(t, "init")

				mustRun(t, "checkout", "-b", "v2")
				require.NoError(t, os.Chmod("index.txt", 0o755))
				mustRun(t, "add", "index.txt")
				gitCommit(t, "chmod")

				cr, err := git.CommitRange(debugGitRun(t), "main")
				require.NoError(t, err)
				return debugGitRun(t), cr
			},
			changes: []*git.FileChange{
				{
					Path: git.PathDiff{
						Before: git.Path{
							Name: "index.txt",
							Type: git.File,
						},
						After: git.Path{
							Name: "index.txt",
							Type: git.File,
						},
					},
					Body: git.BodyDiff{
						Before:        []byte("foo"),
						After:         []byte("foo"),
						ModifiedLines: []int{},
					},
				},
			},
			err: "",
		},
		{
			title: "dir -> file",
			setup: func(t *testing.T) (git.CommandRunner, git.CommitRangeResults) {
				mustRun(t, "init", "--initial-branch=main", ".")
				require.NoError(t, os.Mkdir("index.txt", 0o755))
				require.NoError(t, os.WriteFile("index.txt/.keep", []byte("keep"), 0o644))
				mustRun(t, "add", "index.txt")
				gitCommit(t, "init")

				mustRun(t, "checkout", "-b", "v2")
				require.NoError(t, os.RemoveAll("index.txt"))
				require.NoError(t, os.WriteFile("index.txt", []byte("foo"), 0o644))
				mustRun(t, "add", "index.txt")
				gitCommit(t, "chmod")

				cr, err := git.CommitRange(debugGitRun(t), "main")
				require.NoError(t, err)
				return debugGitRun(t), cr
			},
			changes: []*git.FileChange{
				{
					Path: git.PathDiff{
						Before: git.Path{
							Name: "index.txt",
							Type: git.Dir,
						},
						After: git.Path{
							Name: "index.txt",
							Type: git.File,
						},
					},
					Body: git.BodyDiff{
						Before:        nil,
						After:         []byte("foo"),
						ModifiedLines: []int{1},
					},
				},
				{
					Path: git.PathDiff{
						Before: git.Path{
							Name: "index.txt/.keep",
							Type: git.File,
						},
						After: git.Path{
							Name: "index.txt/.keep",
							Type: git.Missing,
						},
					},
					Body: git.BodyDiff{
						Before:        []byte("keep"),
						After:         nil,
						ModifiedLines: []int{1},
					},
				},
			},
			err: "",
		},
		{
			title: "add file, delete and re-add",
			setup: func(t *testing.T) (git.CommandRunner, git.CommitRangeResults) {
				mustRun(t, "init", "--initial-branch=main", ".")
				require.NoError(t, os.WriteFile("index.txt", []byte("foo"), 0o644))
				mustRun(t, "add", "index.txt")
				gitCommit(t, "init")

				mustRun(t, "checkout", "-b", "v2")
				require.NoError(t, os.Remove("index.txt"))
				mustRun(t, "add", "index.txt")
				gitCommit(t, "rm")
				require.NoError(t, os.WriteFile("index.txt", []byte("foo"), 0o644))
				mustRun(t, "add", "index.txt")
				gitCommit(t, "add")

				cr, err := git.CommitRange(debugGitRun(t), "main")
				require.NoError(t, err)
				return debugGitRun(t), cr
			},
			changes: []*git.FileChange{
				{
					Path: git.PathDiff{
						Before: git.Path{
							Name: "index.txt",
							Type: git.File,
						},
						After: git.Path{
							Name: "index.txt",
							Type: git.File,
						},
					},
					Body: git.BodyDiff{
						Before:        []byte("foo"),
						After:         []byte("foo"),
						ModifiedLines: []int{1},
					},
				},
			},
			err: "",
		},
		{
			title: "file -> symlink",
			setup: func(t *testing.T) (git.CommandRunner, git.CommitRangeResults) {
				mustRun(t, "init", "--initial-branch=main", ".")
				require.NoError(t, os.WriteFile("index.txt", []byte("foo\n1\n"), 0o644))
				mustRun(t, "add", "index.txt")
				require.NoError(t, os.WriteFile("second file.txt", []byte("bar\n1\n"), 0o644))
				mustRun(t, "add", "second file.txt")
				gitCommit(t, "init")

				mustRun(t, "checkout", "-b", "v2")
				require.NoError(t, os.Remove("second file.txt"))
				require.NoError(t, os.Symlink("index.txt", "second file.txt"))
				mustRun(t, "add", "second file.txt")
				gitCommit(t, "symlink")

				cr, err := git.CommitRange(debugGitRun(t), "main")
				require.NoError(t, err)
				return debugGitRun(t), cr
			},
			changes: []*git.FileChange{
				{
					Path: git.PathDiff{
						Before: git.Path{
							Name: "second file.txt",
							Type: git.File,
						},
						After: git.Path{
							Name:          "second file.txt",
							Type:          git.Symlink,
							SymlinkTarget: "index.txt",
						},
					},
					Body: git.BodyDiff{
						Before:        []byte("bar\n1\n"),
						After:         []byte("foo\n1\n"),
						ModifiedLines: []int{1, 2},
					},
				},
			},
			err: "",
		},
		{
			title: "rename partial",
			setup: func(t *testing.T) (git.CommandRunner, git.CommitRangeResults) {
				mustRun(t, "init", "--initial-branch=main", ".")
				require.NoError(t, os.WriteFile("index.txt", []byte("1\n2\n3\n4\n5\n6\n7\n8\n9\n"), 0o644))
				mustRun(t, "add", "index.txt")
				gitCommit(t, "init")

				mustRun(t, "checkout", "-b", "v2")
				mustRun(t, "mv", "index.txt", "second.txt")
				require.NoError(t, os.WriteFile("second.txt", []byte("1\n2\n3\n4\n5\nX\nX\nX\nX\n"), 0o644))
				mustRun(t, "add", "second.txt")
				gitCommit(t, "mv")

				cr, err := git.CommitRange(debugGitRun(t), "main")
				require.NoError(t, err)
				return debugGitRun(t), cr
			},
			changes: []*git.FileChange{
				{
					Path: git.PathDiff{
						Before: git.Path{
							Name: "index.txt",
							Type: git.File,
						},
						After: git.Path{
							Name: "second.txt",
							Type: git.File,
						},
					},
					Body: git.BodyDiff{
						Before:        []byte("1\n2\n3\n4\n5\n6\n7\n8\n9\n"),
						After:         []byte("1\n2\n3\n4\n5\nX\nX\nX\nX\n"),
						ModifiedLines: []int{6, 7, 8, 9},
					},
				},
			},
			err: "",
		},
		{
			title: "rename 100% and edit",
			setup: func(t *testing.T) (git.CommandRunner, git.CommitRangeResults) {
				mustRun(t, "init", "--initial-branch=main", ".")
				require.NoError(t, os.WriteFile("index.txt", []byte("1\n2\n3\n4\n5\n6\n7\n8\n9\n"), 0o644))
				mustRun(t, "add", "index.txt")
				gitCommit(t, "init")

				mustRun(t, "checkout", "-b", "v2")
				mustRun(t, "mv", "index.txt", "second.txt")
				gitCommit(t, "mv")
				require.NoError(t, os.WriteFile("second.txt", []byte("1\n2\n3\n4\n5\nX\n7\n8\n9\n"), 0o644))
				mustRun(t, "add", "second.txt")
				gitCommit(t, "edit")

				cr, err := git.CommitRange(debugGitRun(t), "main")
				require.NoError(t, err)
				return debugGitRun(t), cr
			},
			changes: []*git.FileChange{
				{
					Path: git.PathDiff{
						Before: git.Path{
							Name: "index.txt",
							Type: git.File,
						},
						After: git.Path{
							Name: "second.txt",
							Type: git.File,
						},
					},
					Body: git.BodyDiff{
						Before:        []byte("1\n2\n3\n4\n5\n6\n7\n8\n9\n"),
						After:         []byte("1\n2\n3\n4\n5\nX\n7\n8\n9\n"),
						ModifiedLines: []int{6},
					},
				},
			},
			err: "",
		},
		{
			title: "add file, add another",
			setup: func(t *testing.T) (git.CommandRunner, git.CommitRangeResults) {
				mustRun(t, "init", "--initial-branch=main", ".")
				require.NoError(t, os.WriteFile("index.txt", []byte("foo"), 0o644))
				mustRun(t, "add", "index.txt")
				gitCommit(t, "init")

				mustRun(t, "checkout", "-b", "v2")
				require.NoError(t, os.WriteFile("second.txt", []byte("second"), 0o644))
				mustRun(t, "add", "second.txt")
				require.NoError(t, os.WriteFile("third.txt", []byte("third"), 0o644))
				mustRun(t, "add", "third.txt")
				gitCommit(t, "add two more")

				cr, err := git.CommitRange(debugGitRun(t), "main")
				require.NoError(t, err)
				return debugGitRun(t), cr
			},
			changes: []*git.FileChange{
				{
					Path: git.PathDiff{
						Before: git.Path{
							Name: "second.txt",
							Type: git.Missing,
						},
						After: git.Path{
							Name: "second.txt",
							Type: git.File,
						},
					},
					Body: git.BodyDiff{
						After:         []byte("second"),
						ModifiedLines: []int{1},
					},
				},
				{
					Path: git.PathDiff{
						Before: git.Path{
							Name: "third.txt",
							Type: git.Missing,
						},
						After: git.Path{
							Name: "third.txt",
							Type: git.File,
						},
					},
					Body: git.BodyDiff{
						After:         []byte("third"),
						ModifiedLines: []int{1},
					},
				},
			},
			err: "",
		},
		{
			title: "delete file",
			setup: func(t *testing.T) (git.CommandRunner, git.CommitRangeResults) {
				mustRun(t, "init", "--initial-branch=main", ".")
				require.NoError(t, os.WriteFile("index.txt", []byte("foo"), 0o644))
				mustRun(t, "add", "index.txt")
				require.NoError(t, os.WriteFile("second.txt", []byte("second"), 0o644))
				mustRun(t, "add", "second.txt")
				gitCommit(t, "init")

				mustRun(t, "checkout", "-b", "v2")
				require.NoError(t, os.Remove("second.txt"))
				mustRun(t, "add", "second.txt")
				gitCommit(t, "rm second")

				cr, err := git.CommitRange(debugGitRun(t), "main")
				require.NoError(t, err)
				return debugGitRun(t), cr
			},
			changes: []*git.FileChange{
				{
					Path: git.PathDiff{
						Before: git.Path{
							Name: "second.txt",
							Type: git.File,
						},
						After: git.Path{
							Name: "second.txt",
							Type: git.Missing,
						},
					},
					Body: git.BodyDiff{
						Before:        []byte("second"),
						ModifiedLines: []int{1},
					},
				},
			},
			err: "",
		},
		{
			title: "delete symlink",
			setup: func(t *testing.T) (git.CommandRunner, git.CommitRangeResults) {
				mustRun(t, "init", "--initial-branch=main", ".")
				require.NoError(t, os.WriteFile("index.txt", []byte("foo"), 0o644))
				mustRun(t, "add", "index.txt")
				require.NoError(t, os.Symlink("index.txt", "second.txt"))
				mustRun(t, "add", "second.txt")
				gitCommit(t, "init")

				mustRun(t, "checkout", "-b", "v2")
				require.NoError(t, os.Remove("second.txt"))
				mustRun(t, "add", "second.txt")
				gitCommit(t, "rm second")

				cr, err := git.CommitRange(debugGitRun(t), "main")
				require.NoError(t, err)
				return debugGitRun(t), cr
			},
			changes: []*git.FileChange{
				{
					Path: git.PathDiff{
						Before: git.Path{
							Name:          "second.txt",
							Type:          git.Symlink,
							SymlinkTarget: "index.txt",
						},
						After: git.Path{
							Name: "second.txt",
							Type: git.Missing,
						},
					},
					Body: git.BodyDiff{
						Before:        []byte("foo"),
						ModifiedLines: []int{1},
					},
				},
			},
			err: "",
		},
		{
			title: "delete directory with symlinks",
			setup: func(t *testing.T) (git.CommandRunner, git.CommitRangeResults) {
				mustRun(t, "init", "--initial-branch=main", ".")
				require.NoError(t, os.WriteFile("index.txt", []byte("foo"), 0o644))
				mustRun(t, "add", "index.txt")
				require.NoError(t, os.Mkdir("dir", 0o755))
				require.NoError(t, os.Symlink("../index.txt", "dir/first.txt"))
				require.NoError(t, os.Symlink("../index.txt", "dir/second.txt"))
				mustRun(t, "add", "dir")
				gitCommit(t, "init")

				mustRun(t, "checkout", "-b", "v2")
				require.NoError(t, os.RemoveAll("dir"))
				mustRun(t, "add", "dir")
				gitCommit(t, "rm dir")

				cr, err := git.CommitRange(debugGitRun(t), "main")
				require.NoError(t, err)
				return debugGitRun(t), cr
			},
			changes: []*git.FileChange{
				{
					Path: git.PathDiff{
						Before: git.Path{
							Name:          "dir/first.txt",
							Type:          git.Symlink,
							SymlinkTarget: "index.txt",
						},
						After: git.Path{
							Name: "dir/first.txt",
							Type: git.Missing,
						},
					},
					Body: git.BodyDiff{
						Before:        []byte("foo"),
						ModifiedLines: []int{1},
					},
				},
				{
					Path: git.PathDiff{
						Before: git.Path{
							Name:          "dir/second.txt",
							Type:          git.Symlink,
							SymlinkTarget: "index.txt",
						},
						After: git.Path{
							Name: "dir/second.txt",
							Type: git.Missing,
						},
					},
					Body: git.BodyDiff{
						Before:        []byte("foo"),
						ModifiedLines: []int{1},
					},
				},
			},
			err: "",
		},
		{
			title: "symlink target changed",
			setup: func(t *testing.T) (git.CommandRunner, git.CommitRangeResults) {
				mustRun(t, "init", "--initial-branch=main", ".")
				require.NoError(t, os.WriteFile("index.txt", []byte("foo\n1\n"), 0o644))
				mustRun(t, "add", "index.txt")
				require.NoError(t, os.WriteFile("second file.txt", []byte("bar\n1\n"), 0o644))
				mustRun(t, "add", "second file.txt")
				require.NoError(t, os.Mkdir("dir", 0o755))
				require.NoError(t, os.Symlink("../index.txt", "dir/first.txt"))
				require.NoError(t, os.Symlink("../second file.txt", "dir/second.txt"))
				mustRun(t, "add", "dir")
				gitCommit(t, "init")

				mustRun(t, "checkout", "-b", "v2")
				require.NoError(t, os.Remove("dir/second.txt"))
				require.NoError(t, os.Symlink("first.txt", "dir/second.txt"))
				mustRun(t, "add", "dir")
				gitCommit(t, "symlink change")

				cr, err := git.CommitRange(debugGitRun(t), "main")
				require.NoError(t, err)
				return debugGitRun(t), cr
			},
			changes: []*git.FileChange{
				{
					Path: git.PathDiff{
						Before: git.Path{
							Name:          "dir/second.txt",
							Type:          git.Symlink,
							SymlinkTarget: "second file.txt",
						},
						After: git.Path{
							Name:          "dir/second.txt",
							Type:          git.Symlink,
							SymlinkTarget: "index.txt",
						},
					},
					Body: git.BodyDiff{
						Before:        []byte("bar\n1\n"),
						After:         []byte("foo\n1\n"),
						ModifiedLines: []int{1, 2},
					},
				},
			},
			err: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			slog.SetDefault(slogt.New(t))

			dir := t.TempDir()
			err := os.Chdir(dir)
			require.NoError(t, err, "chdir")

			cmd, cr := tc.setup(t)
			changes, err := git.Changes(cmd, cr, git.NewPathFilter(nil, nil, nil))
			if tc.err != "" {
				require.EqualError(t, err, tc.err)
				require.Nil(t, changes)
			} else {
				require.NoError(t, err)
				require.Len(t, changes, len(tc.changes))
				for i := range tc.changes {
					require.Equal(t, tc.changes[i].Path, changes[i].Path, "changes[%d].Path", i)
					require.Equal(t, tc.changes[i].Body, changes[i].Body, "changes[%d].Body", i)
				}
			}
		})
	}
}
