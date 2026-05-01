package git_test

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"regexp"
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
	mustRun(t, "commit", "-am", "commit "+message)
}

func TestChanges(t *testing.T) {
	type testCaseT struct {
		setup   func(t *testing.T) git.CommandRunner
		title   string
		err     string
		changes []*git.FileChange
	}

	testCases := []testCaseT{
		{
			title: "git log error",
			setup: func(_ *testing.T) git.CommandRunner {
				cmd := func(args ...string) ([]byte, error) {
					return nil, fmt.Errorf("mock git error: %v", args)
				}
				return cmd
			},
			changes: nil,
			err:     "failed to get the list of modified files from git: mock git error: [log --reverse --no-merges --first-parent --format=%H --name-status main..HEAD]",
		},
		{
			title: "chmod",
			setup: func(t *testing.T) git.CommandRunner {
				mustRun(t, "init", "--initial-branch=main", ".")
				require.NoError(t, os.WriteFile("index.txt", []byte("foo"), 0o644))
				mustRun(t, "add", "index.txt")
				gitCommit(t, "init")

				mustRun(t, "checkout", "-b", "v2")
				require.NoError(t, os.Chmod("index.txt", 0o755))
				mustRun(t, "add", "index.txt")
				gitCommit(t, "chmod")

				return debugGitRun(t)
			},
			changes: []*git.FileChange{
				{
					Commits: []string{"1"},
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
						Before: []byte("foo"),
						After:  []byte("foo"),
						Lines:  []git.LineNumber{},
					},
				},
			},
			err: "",
		},
		{
			title: "dir -> file",
			setup: func(t *testing.T) git.CommandRunner {
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

				return debugGitRun(t)
			},
			changes: []*git.FileChange{
				{
					Commits: []string{"1"},
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
						Before: nil,
						After:  []byte("foo"),
						Lines: []git.LineNumber{
							{Before: 0, After: 1},
						},
					},
				},
				{
					Commits: []string{"1"},
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
						Before: []byte("keep"),
						After:  nil,
						Lines: []git.LineNumber{
							{Before: 1, After: 0},
						},
					},
				},
			},
			err: "",
		},
		{
			title: "delete and re-add",
			setup: func(t *testing.T) git.CommandRunner {
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

				return debugGitRun(t)
			},
			changes: []*git.FileChange{
				{
					Commits: []string{"1", "2"},
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
						Before: []byte("foo"),
						After:  []byte("foo"),
						Lines:  []git.LineNumber{},
					},
				},
			},
			err: "",
		},
		{
			title: "file -> symlink",
			setup: func(t *testing.T) git.CommandRunner {
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

				return debugGitRun(t)
			},
			changes: []*git.FileChange{
				{
					Commits: []string{"1"},
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
						Before: []byte("bar\n1\n"),
						After:  []byte("foo\n1\n"),
						Lines: []git.LineNumber{
							{Before: 0, After: 1},
							{Before: 0, After: 2},
						},
					},
				},
			},
			err: "",
		},
		{
			title: "rename partial",
			setup: func(t *testing.T) git.CommandRunner {
				mustRun(t, "init", "--initial-branch=main", ".")
				require.NoError(t, os.WriteFile("index.txt", []byte("1\n2\n3\n4\n5\n6\n7\n8\n9\n"), 0o644))
				mustRun(t, "add", "index.txt")
				gitCommit(t, "init")

				mustRun(t, "checkout", "-b", "v2")
				mustRun(t, "mv", "index.txt", "second.txt")
				require.NoError(t, os.WriteFile("second.txt", []byte("1\n2\n3\n4\n5\nX\nX\nX\nX\n"), 0o644))
				mustRun(t, "add", "second.txt")
				gitCommit(t, "mv")

				return debugGitRun(t)
			},
			changes: []*git.FileChange{
				{
					Commits: []string{"1"},
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
						Before: []byte("1\n2\n3\n4\n5\n6\n7\n8\n9\n"),
						After:  []byte("1\n2\n3\n4\n5\nX\nX\nX\nX\n"),
						Lines: []git.LineNumber{
							{Before: 6, After: 6},
							{Before: 7, After: 7},
							{Before: 8, After: 8},
							{Before: 9, After: 9},
						},
					},
				},
			},
			err: "",
		},
		{
			title: "rename 100% and edit",
			setup: func(t *testing.T) git.CommandRunner {
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

				return debugGitRun(t)
			},
			changes: []*git.FileChange{
				{
					Commits: []string{"1", "2"},
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
						Before: []byte("1\n2\n3\n4\n5\n6\n7\n8\n9\n"),
						After:  []byte("1\n2\n3\n4\n5\nX\n7\n8\n9\n"),
						Lines: []git.LineNumber{
							{Before: 6, After: 6},
						},
					},
				},
			},
			err: "",
		},
		{
			title: "add file, add another",
			setup: func(t *testing.T) git.CommandRunner {
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

				return debugGitRun(t)
			},
			changes: []*git.FileChange{
				{
					Commits: []string{"1"},
					Path: git.PathDiff{
						Before: git.Path{
							Name: "",
							Type: git.Missing,
						},
						After: git.Path{
							Name: "second.txt",
							Type: git.File,
						},
					},
					Body: git.BodyDiff{
						After: []byte("second"),
						Lines: []git.LineNumber{
							{Before: 0, After: 1},
						},
					},
				},
				{
					Commits: []string{"1"},
					Path: git.PathDiff{
						Before: git.Path{
							Name: "",
							Type: git.Missing,
						},
						After: git.Path{
							Name: "third.txt",
							Type: git.File,
						},
					},
					Body: git.BodyDiff{
						After: []byte("third"),
						Lines: []git.LineNumber{
							{Before: 0, After: 1},
						},
					},
				},
			},
			err: "",
		},
		{
			title: "delete file",
			setup: func(t *testing.T) git.CommandRunner {
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

				return debugGitRun(t)
			},
			changes: []*git.FileChange{
				{
					Commits: []string{"1"},
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
						Before: []byte("second"),
						Lines: []git.LineNumber{
							{Before: 1, After: 0},
						},
					},
				},
			},
			err: "",
		},
		{
			title: "delete symlink",
			setup: func(t *testing.T) git.CommandRunner {
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

				return debugGitRun(t)
			},
			changes: []*git.FileChange{
				{
					Commits: []string{"1"},
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
						Before: []byte("foo"),
						Lines: []git.LineNumber{
							{Before: 1, After: 0},
						},
					},
				},
			},
			err: "",
		},
		{
			title: "delete directory with symlinks",
			setup: func(t *testing.T) git.CommandRunner {
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

				return debugGitRun(t)
			},
			changes: []*git.FileChange{
				{
					Commits: []string{"1"},
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
						Before: []byte("foo"),
						Lines: []git.LineNumber{
							{Before: 1, After: 0},
						},
					},
				},
				{
					Commits: []string{"1"},
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
						Before: []byte("foo"),
						Lines: []git.LineNumber{
							{Before: 1, After: 0},
						},
					},
				},
			},
			err: "",
		},
		{
			title: "symlink target changed",
			setup: func(t *testing.T) git.CommandRunner {
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

				return debugGitRun(t)
			},
			changes: []*git.FileChange{
				{
					Commits: []string{"1"},
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
						Before: []byte("bar\n1\n"),
						After:  []byte("foo\n1\n"),
						Lines: []git.LineNumber{
							{Before: 0, After: 1},
							{Before: 0, After: 2},
						},
					},
				},
			},
			err: "",
		},
		{
			title: "rule modified then file renamed",
			setup: func(t *testing.T) git.CommandRunner {
				mustRun(t, "init", "--initial-branch=main", ".")
				require.NoError(t, os.WriteFile("main.txt", []byte("l1\nl2\nl3\n"), 0o644))
				mustRun(t, "add", "main.txt")
				gitCommit(t, "init")

				mustRun(t, "checkout", "-b", "v2")
				require.NoError(t, os.WriteFile("main.txt", []byte("l1\nl3\n"), 0o644))
				mustRun(t, "add", "main.txt")
				gitCommit(t, "edit")

				mustRun(t, "mv", "main.txt", "pr.txt")
				gitCommit(t, "rename")

				return debugGitRun(t)
			},
			changes: []*git.FileChange{
				{
					Commits: []string{"1", "2"},
					Path: git.PathDiff{
						Before: git.Path{
							Name: "main.txt",
							Type: git.File,
						},
						After: git.Path{
							Name: "pr.txt",
							Type: git.File,
						},
					},
					Body: git.BodyDiff{
						Before: []byte("l1\nl2\nl3\n"),
						After:  []byte("l1\nl3\n"),
						Lines: []git.LineNumber{
							{Before: 2, After: 0},
						},
					},
				},
			},
			err: "",
		},
		{
			title: "directory_renamed",
			setup: func(t *testing.T) git.CommandRunner {
				mustRun(t, "init", "--initial-branch=main", ".")
				require.NoError(t, os.Mkdir("dir1", 0o755))
				require.NoError(t, os.Mkdir("dir1/rules", 0o755))
				require.NoError(t, os.WriteFile("dir1/rules/file1.txt", []byte("a1\na2\na3\n"), 0o644))
				require.NoError(t, os.WriteFile("dir1/rules/file2.txt", []byte("b1\nb2"), 0o644))
				mustRun(t, "add", "dir1")
				gitCommit(t, "init")

				mustRun(t, "checkout", "-b", "v1")
				mustRun(t, "mv", "dir1", "dir2")
				gitCommit(t, "rename")

				return debugGitRun(t)
			},
			changes: []*git.FileChange{
				{
					Commits: []string{"1"},
					Path: git.PathDiff{
						Before: git.Path{
							Name: "dir1/rules/file1.txt",
							Type: git.File,
						},
						After: git.Path{
							Name: "dir2/rules/file1.txt",
							Type: git.File,
						},
					},
					Body: git.BodyDiff{
						Before: []byte("a1\na2\na3\n"),
						After:  []byte("a1\na2\na3\n"),
						// Renamed without content changes.
						Lines: []git.LineNumber{},
					},
				},
				{
					Commits: []string{"1"},
					Path: git.PathDiff{
						Before: git.Path{
							Name: "dir1/rules/file2.txt",
							Type: git.File,
						},
						After: git.Path{
							Name: "dir2/rules/file2.txt",
							Type: git.File,
						},
					},
					Body: git.BodyDiff{
						Before: []byte("b1\nb2"),
						After:  []byte("b1\nb2"),
						// Renamed without content changes.
						Lines: []git.LineNumber{},
					},
				},
			},
			err: "",
		},
		{
			title: "rule partially replaced",
			setup: func(t *testing.T) git.CommandRunner {
				mustRun(t, "init", "--initial-branch=main", ".")
				require.NoError(t, os.WriteFile("main.txt", []byte("l1\nl2\nl3\n"), 0o644))
				mustRun(t, "add", "main.txt")
				gitCommit(t, "init")

				mustRun(t, "checkout", "-b", "v2")
				require.NoError(t, os.WriteFile("main.txt", []byte("l1\nl3\n"), 0o644))
				mustRun(t, "add", "main.txt")
				gitCommit(t, "edit")

				mustRun(t, "mv", "main.txt", "pr.txt")
				gitCommit(t, "rename")

				return debugGitRun(t)
			},
			changes: []*git.FileChange{
				{
					Commits: []string{"1", "2"},
					Path: git.PathDiff{
						Before: git.Path{
							Name: "main.txt",
							Type: git.File,
						},
						After: git.Path{
							Name: "pr.txt",
							Type: git.File,
						},
					},
					Body: git.BodyDiff{
						Before: []byte("l1\nl2\nl3\n"),
						After:  []byte("l1\nl3\n"),
						Lines: []git.LineNumber{
							{Before: 2, After: 0},
						},
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
			t.Chdir(dir)

			cmd := tc.setup(t)
			changes, err := git.Changes(cmd, "main", git.NewPathFilter(nil, nil, nil))
			if tc.err != "" {
				require.EqualError(t, err, tc.err)
				require.Nil(t, changes)
			} else {
				require.NoError(t, err)
				require.Len(t, changes, len(tc.changes))
				for i := range tc.changes {
					require.Len(t, changes[i].Commits, len(tc.changes[i].Commits), "changes[%d].Commits", i)
					require.Equal(t, tc.changes[i].Path, changes[i].Path, "changes[%d].Path", i)
					require.Equal(t, tc.changes[i].Body, changes[i].Body, "changes[%d].Body", i)
				}
				require.Len(t, changes, len(tc.changes))
			}
		})
	}
}

func TestChangesParseDiff(t *testing.T) {
	type testCaseT struct {
		title   string
		err     string
		mock    git.CommandRunner
		changes []*git.FileChange
	}

	testCases := []testCaseT{
		{
			title: "empty line in git log output",
			mock: func(args ...string) ([]byte, error) {
				if args[0] == "log" {
					// Empty line should trigger len(parts) == 0
					return []byte("abc123\n\nM\tfile.txt\n"), nil
				}
				if args[0] == "ls-tree" {
					return []byte("100644 blob abc123def456\tfile.txt\n"), nil
				}
				if args[0] == "cat-file" {
					return []byte("content"), nil
				}
				if args[0] == "diff" {
					return nil, nil
				}
				return nil, nil
			},
			changes: []*git.FileChange{
				{
					Commits: []string{"1"},
					Path: git.PathDiff{
						Before: git.Path{
							Name: "file.txt",
							Type: git.File,
						},
						After: git.Path{
							Name: "file.txt",
							Type: git.File,
						},
					},
					Body: git.BodyDiff{
						Before: []byte("content"),
						After:  []byte("content"),
						Lines:  []git.LineNumber{},
					},
				},
			},
		},
		{
			title: "ls-tree malformed line - less than 3 space-separated parts",
			mock: func(args ...string) ([]byte, error) {
				if args[0] == "log" {
					return []byte("abc123\nA\tnewfile.txt\n"), nil
				}
				if args[0] == "ls-tree" {
					// Only 2 parts instead of 3
					return []byte("100644 blob\n"), nil
				}
				if args[0] == "cat-file" {
					return []byte("content"), nil
				}
				return nil, nil
			},
			changes: []*git.FileChange{
				{
					Commits: []string{"1"},
					Path: git.PathDiff{
						Before: git.Path{
							Name: "",
							Type: git.Missing,
						},
						After: git.Path{
							Name: "newfile.txt",
							Type: git.Missing,
						},
					},
					Body: git.BodyDiff{
						Before: nil,
						After:  []byte("content"),
						Lines:  []git.LineNumber{},
					},
				},
			},
		},
		{
			title: "ls-tree line missing tab separator",
			mock: func(args ...string) ([]byte, error) {
				if args[0] == "log" {
					return []byte("abc123\nA\tnewfile.txt\n"), nil
				}
				if args[0] == "ls-tree" {
					// Has 3 space parts but no tab in third part
					return []byte("100644 blob abc123def456\n"), nil
				}
				if args[0] == "cat-file" {
					return []byte("content"), nil
				}
				return nil, nil
			},
			changes: []*git.FileChange{
				{
					Commits: []string{"1"},
					Path: git.PathDiff{
						Before: git.Path{
							Name: "",
							Type: git.Missing,
						},
						After: git.Path{
							Name: "newfile.txt",
							Type: git.Missing,
						},
					},
					Body: git.BodyDiff{
						Before: nil,
						After:  []byte("content"),
						Lines:  []git.LineNumber{},
					},
				},
			},
		},
		{
			title: "ls-tree returns different path",
			mock: func(args ...string) ([]byte, error) {
				if args[0] == "log" {
					return []byte("abc123\nA\tnewfile.txt\n"), nil
				}
				if args[0] == "ls-tree" {
					// Returns a different file path
					return []byte("100644 blob abc123def456\totherfile.txt\n"), nil
				}
				if args[0] == "cat-file" {
					return []byte("content"), nil
				}
				return nil, nil
			},
			changes: []*git.FileChange{
				{
					Commits: []string{"1"},
					Path: git.PathDiff{
						Before: git.Path{
							Name: "",
							Type: git.Missing,
						},
						After: git.Path{
							Name: "newfile.txt",
							Type: git.Missing,
						},
					},
					Body: git.BodyDiff{
						Before: nil,
						After:  []byte("content"),
						Lines:  []git.LineNumber{},
					},
				},
			},
		},
		{
			title: "ls-tree returns tag object type",
			mock: func(args ...string) ([]byte, error) {
				if args[0] == "log" {
					return []byte("abc123\nA\tnewfile.txt\n"), nil
				}
				if args[0] == "ls-tree" {
					// Object type is "tag" not "blob" or "tree"
					return []byte("100644 tag abc123def456\tnewfile.txt\n"), nil
				}
				if args[0] == "cat-file" {
					return []byte("content"), nil
				}
				return nil, nil
			},
			changes: []*git.FileChange{
				{
					Commits: []string{"1"},
					Path: git.PathDiff{
						Before: git.Path{
							Name: "",
							Type: git.Missing,
						},
						After: git.Path{
							Name: "newfile.txt",
							Type: git.Missing,
						},
					},
					Body: git.BodyDiff{
						Before: nil,
						After:  []byte("content"),
						Lines:  []git.LineNumber{},
					},
				},
			},
		},
		{
			// git diff command returns an error for a modified file.
			title: "git diff error",
			err:   "failed to run git diff for file.txt: git diff for file.txt: mock diff error",
			mock: func(args ...string) ([]byte, error) {
				if args[0] == "log" {
					return []byte("abc123\nM\tfile.txt\n"), nil
				}
				if args[0] == "ls-tree" {
					return []byte("100644 blob abc123def456\tfile.txt\n"), nil
				}
				if args[0] == "cat-file" {
					return []byte("content"), nil
				}
				if args[0] == "diff" {
					return nil, errors.New("mock diff error")
				}
				return nil, nil
			},
		},
		{
			title: "ls-tree error on before commit",
			mock: func(args ...string) ([]byte, error) {
				if args[0] == "log" {
					return []byte("abc123\nM\tfile.txt\n"), nil
				}
				if args[0] == "ls-tree" {
					if strings.Contains(strings.Join(args, " "), "abc123^") {
						return nil, errors.New("mock ls-tree error")
					}
					return []byte("100644 blob abc123def456\tfile.txt\n"), nil
				}
				if args[0] == "cat-file" {
					// Fail for the before commit blob ref.
					if strings.Contains(args[2], "abc123^") {
						return nil, errors.New("mock cat-file error")
					}
					return []byte("new content"), nil
				}
				if args[0] == "diff" {
					return nil, nil
				}
				return nil, nil
			},
			changes: []*git.FileChange{
				{
					Commits: []string{"1"},
					Path: git.PathDiff{
						Before: git.Path{
							Name: "file.txt",
							Type: git.Missing,
						},
						After: git.Path{
							Name: "file.txt",
							Type: git.File,
						},
					},
					Body: git.BodyDiff{
						Before: nil,
						After:  []byte("new content"),
						Lines: []git.LineNumber{
							{Before: 0, After: 1},
						},
					},
				},
			},
		},
		{
			// File content lines starting with "--- " or "+++ " inside a hunk
			// must be treated as deletions/additions, not as diff headers.
			// The inHunk flag ensures headers are only matched outside hunks.
			title: "hunk content with triple-dash and triple-plus lines",
			mock: func(args ...string) ([]byte, error) {
				if args[0] == "log" {
					return []byte("abc123\nM\tfile.txt\n"), nil
				}
				if args[0] == "ls-tree" {
					return []byte("100644 blob abc123def456\tfile.txt\n"), nil
				}
				if args[0] == "cat-file" {
					if strings.Contains(args[2], "abc123^") {
						return []byte("-- old\nkeep\n"), nil
					}
					return []byte("++ new\nkeep\n"), nil
				}
				if args[0] == "diff" {
					return []byte(
						"diff --git a/file.txt b/file.txt\n" +
							"--- a/file.txt\n" +
							"+++ b/file.txt\n" +
							"@@ -1,2 +1,2 @@\n" +
							"--- old\n" +
							"+++ new\n" +
							" keep\n",
					), nil
				}
				return nil, nil
			},
			changes: []*git.FileChange{
				{
					Commits: []string{"1"},
					Path: git.PathDiff{
						Before: git.Path{
							Name:          "file.txt",
							SymlinkTarget: "",
							Type:          git.File,
						},
						After: git.Path{
							Name:          "file.txt",
							SymlinkTarget: "",
							Type:          git.File,
						},
					},
					Body: git.BodyDiff{
						Before: []byte("-- old\nkeep\n"),
						After:  []byte("++ new\nkeep\n"),
						Lines: git.LineNumbers{
							{Before: 1, After: 1},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			slog.SetDefault(slogt.New(t))

			changes, err := git.Changes(tc.mock, "main", git.NewPathFilter(nil, nil, nil))
			if tc.err != "" {
				require.EqualError(t, err, tc.err)
				require.Nil(t, changes)
			} else {
				require.NoError(t, err)
				require.Len(t, changes, len(tc.changes))
				for i := range tc.changes {
					require.Len(t, changes[i].Commits, len(tc.changes[i].Commits), "changes[%d].Commits", i)
					require.Equal(t, tc.changes[i].Path, changes[i].Path, "changes[%d].Path", i)
					require.Equal(t, tc.changes[i].Body, changes[i].Body, "changes[%d].Body", i)
				}
			}
		})
	}
}

// Path filter excludes a file, so it should not appear in results.
func TestChangesPathFilterExclusion(t *testing.T) {
	slog.SetDefault(slogt.New(t))

	mock := func(args ...string) ([]byte, error) {
		if args[0] == "log" {
			return []byte("abc123\nM\tfile.txt\nM\texcluded.txt\n"), nil
		}
		if args[0] == "ls-tree" {
			return []byte("100644 blob abc123def456\t" + args[2] + "\n"), nil
		}
		if args[0] == "cat-file" {
			return []byte("content"), nil
		}
		if args[0] == "diff" {
			return nil, nil
		}
		return nil, nil
	}

	exclude := []*regexp.Regexp{regexp.MustCompile(`^excluded\.txt$`)}
	filter := git.NewPathFilter(nil, exclude, nil)
	changes, err := git.Changes(mock, "main", filter)
	require.NoError(t, err)
	require.Len(t, changes, 1)
	require.Equal(t, "file.txt", changes[0].Path.After.Name)
}

// Directory path in git log output is skipped.
func TestChangesSkipsDirectoryPath(t *testing.T) {
	slog.SetDefault(slogt.New(t))

	dir := t.TempDir()

	mock := func(args ...string) ([]byte, error) {
		if args[0] == "log" {
			return []byte("abc123\nM\t" + dir + "\n"), nil
		}
		return nil, nil
	}

	changes, err := git.Changes(mock, "main", git.NewPathFilter(nil, nil, nil))
	require.NoError(t, err)
	require.Empty(t, changes)
}

func TestLineNumberString(t *testing.T) {
	type testCaseT struct {
		title    string
		expected string
		ln       git.LineNumber
	}

	testCases := []testCaseT{
		{
			title:    "added line",
			ln:       git.LineNumber{Before: 0, After: 5},
			expected: "+5",
		},
		{
			title:    "deleted line",
			ln:       git.LineNumber{Before: 3, After: 0},
			expected: "-3",
		},
		{
			title:    "same before and after",
			ln:       git.LineNumber{Before: 7, After: 7},
			expected: "7",
		},
		{
			title:    "different before and after",
			ln:       git.LineNumber{Before: 4, After: 9},
			expected: "4->9",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			require.Equal(t, tc.expected, tc.ln.String())
		})
	}
}

func TestHasAfter(t *testing.T) {
	type testCaseT struct {
		title    string
		lns      git.LineNumbers
		line     int
		expected bool
	}

	testCases := []testCaseT{
		{
			title: "match found",
			lns: git.LineNumbers{
				{Before: 1, After: 2},
				{Before: 3, After: 5},
			},
			line:     5,
			expected: true,
		},
		{
			title: "no match",
			lns: git.LineNumbers{
				{Before: 1, After: 2},
				{Before: 3, After: 4},
			},
			line:     99,
			expected: false,
		},
		{
			title:    "empty slice",
			lns:      git.LineNumbers{},
			line:     1,
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			require.Equal(t, tc.expected, tc.lns.HasAfter(tc.line))
		})
	}
}

func TestHasBefore(t *testing.T) {
	type testCaseT struct {
		title    string
		lns      git.LineNumbers
		line     int
		expected bool
	}

	testCases := []testCaseT{
		{
			title: "match found",
			lns: git.LineNumbers{
				{Before: 1, After: 2},
				{Before: 3, After: 5},
			},
			line:     3,
			expected: true,
		},
		{
			title: "no match",
			lns: git.LineNumbers{
				{Before: 1, After: 2},
				{Before: 3, After: 4},
			},
			line:     99,
			expected: false,
		},
		{
			title:    "empty slice",
			lns:      git.LineNumbers{},
			line:     1,
			expected: false,
		},
		{
			title: "additions only, no match",
			lns: git.LineNumbers{
				{Before: 0, After: 3},
				{Before: 0, After: 4},
			},
			line:     3,
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			require.Equal(t, tc.expected, tc.lns.HasBefore(tc.line))
		})
	}
}

func TestNearestAfter(t *testing.T) {
	type testCaseT struct {
		title    string
		lns      git.LineNumbers
		target   int
		expected int
	}

	testCases := []testCaseT{
		{
			title: "exact match",
			lns: git.LineNumbers{
				{Before: 0, After: 5},
				{Before: 0, After: 10},
			},
			target:   10,
			expected: 10,
		},
		{
			title: "closer to lower",
			lns: git.LineNumbers{
				{Before: 0, After: 5},
				{Before: 0, After: 20},
			},
			target:   8,
			expected: 5,
		},
		{
			title: "closer to higher",
			lns: git.LineNumbers{
				{Before: 0, After: 5},
				{Before: 0, After: 20},
			},
			target:   18,
			expected: 20,
		},
		{
			title: "target below all",
			lns: git.LineNumbers{
				{Before: 0, After: 10},
				{Before: 0, After: 20},
			},
			target:   2,
			expected: 10,
		},
		{
			title: "target above all",
			lns: git.LineNumbers{
				{Before: 0, After: 10},
				{Before: 0, After: 20},
			},
			target:   99,
			expected: 20,
		},
		{
			title: "skips deletions",
			lns: git.LineNumbers{
				{Before: 5, After: 0},
				{Before: 0, After: 20},
			},
			target:   3,
			expected: 20,
		},
		{
			title:    "empty slice",
			lns:      git.LineNumbers{},
			target:   5,
			expected: 0,
		},
		{
			title: "all deletions",
			lns: git.LineNumbers{
				{Before: 3, After: 0},
				{Before: 5, After: 0},
			},
			target:   4,
			expected: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			require.Equal(t, tc.expected, tc.lns.NearestAfter(tc.target))
		})
	}
}

func TestNearestBefore(t *testing.T) {
	type testCaseT struct {
		title    string
		lns      git.LineNumbers
		target   int
		expected int
	}

	testCases := []testCaseT{
		{
			title: "exact match",
			lns: git.LineNumbers{
				{Before: 5, After: 0},
				{Before: 10, After: 0},
			},
			target:   10,
			expected: 10,
		},
		{
			title: "closer to lower",
			lns: git.LineNumbers{
				{Before: 5, After: 0},
				{Before: 20, After: 0},
			},
			target:   8,
			expected: 5,
		},
		{
			title: "closer to higher",
			lns: git.LineNumbers{
				{Before: 5, After: 0},
				{Before: 20, After: 0},
			},
			target:   18,
			expected: 20,
		},
		{
			title: "target below all",
			lns: git.LineNumbers{
				{Before: 10, After: 0},
				{Before: 20, After: 0},
			},
			target:   2,
			expected: 10,
		},
		{
			title: "skips additions",
			lns: git.LineNumbers{
				{Before: 0, After: 5},
				{Before: 20, After: 0},
			},
			target:   3,
			expected: 20,
		},
		{
			title:    "empty slice",
			lns:      git.LineNumbers{},
			target:   5,
			expected: 0,
		},
		{
			title: "all additions",
			lns: git.LineNumbers{
				{Before: 0, After: 3},
				{Before: 0, After: 5},
			},
			target:   4,
			expected: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			require.Equal(t, tc.expected, tc.lns.NearestBefore(tc.target))
		})
	}
}

func TestBeforeForAfter(t *testing.T) {
	type testCaseT struct {
		title    string
		lns      git.LineNumbers
		line     int
		expected int
	}

	testCases := []testCaseT{
		{
			title: "match found",
			lns: git.LineNumbers{
				{Before: 10, After: 20},
				{Before: 30, After: 40},
			},
			line:     20,
			expected: 10,
		},
		{
			// Line past the last diff entry — compute from nearest preceding entry.
			// Nearest is {Before:10, After:20}, offset = 99 - 20 = 79, so old = 10 + 79 = 89.
			title: "line past last entry uses offset",
			lns: git.LineNumbers{
				{Before: 10, After: 20},
			},
			line:     99,
			expected: 89,
		},
		{
			title:    "empty slice returns line itself",
			lns:      git.LineNumbers{},
			line:     5,
			expected: 5,
		},
		{
			title: "line before first entry",
			lns: git.LineNumbers{
				{Before: 10, After: 20},
			},
			line:     5,
			expected: 5,
		},
		{
			// Line between two entries — compute from nearest preceding entry.
			// Nearest is {Before:5, After:10}, offset = 15 - 10 = 5, so old = 5 + 5 = 10.
			title: "line between entries",
			lns: git.LineNumbers{
				{Before: 5, After: 10},
				{Before: 25, After: 30},
			},
			line:     15,
			expected: 10,
		},
		{
			// Added lines (Before==0) are skipped when finding nearest reference.
			// Only {Before:5, After:10} is usable, offset = 20 - 10 = 10, so old = 5 + 10 = 15.
			title: "skips added-only entries",
			lns: git.LineNumbers{
				{Before: 5, After: 10},
				{Before: 0, After: 12},
				{Before: 0, After: 13},
			},
			line:     20,
			expected: 15,
		},
		{
			title: "all entries are added",
			lns: git.LineNumbers{
				{Before: 0, After: 3},
				{Before: 0, After: 4},
			},
			line:     10,
			expected: 10,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			require.Equal(t, tc.expected, tc.lns.BeforeForAfter(tc.line))
		})
	}
}

func TestMakeLineRange(t *testing.T) {
	type testCaseT struct {
		title    string
		expected git.LineNumbers
		n        int
		side     git.LineRangeSide
	}

	testCases := []testCaseT{
		{
			title: "before only",
			n:     2,
			side:  git.LinesBefore,
			expected: git.LineNumbers{
				{Before: 1, After: 0},
				{Before: 2, After: 0},
			},
		},
		{
			title: "after only",
			n:     2,
			side:  git.LinesAfter,
			expected: git.LineNumbers{
				{Before: 0, After: 1},
				{Before: 0, After: 2},
			},
		},
		{
			title: "both",
			n:     3,
			side:  git.LinesBoth,
			expected: git.LineNumbers{
				{Before: 1, After: 1},
				{Before: 2, After: 2},
				{Before: 3, After: 3},
			},
		},
		{
			title:    "zero count",
			n:        0,
			side:     git.LinesBoth,
			expected: git.LineNumbers{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			require.Equal(t, tc.expected, git.MakeLineRange(tc.n, tc.side))
		})
	}
}

func TestMakeLineRangeFromTo(t *testing.T) {
	type testCaseT struct {
		title    string
		expected git.LineNumbers
		first    int
		last     int
		side     git.LineRangeSide
	}

	testCases := []testCaseT{
		{
			title: "before only",
			first: 5,
			last:  7,
			side:  git.LinesBefore,
			expected: git.LineNumbers{
				{Before: 5, After: 0},
				{Before: 6, After: 0},
				{Before: 7, After: 0},
			},
		},
		{
			title: "after only",
			first: 3,
			last:  4,
			side:  git.LinesAfter,
			expected: git.LineNumbers{
				{Before: 0, After: 3},
				{Before: 0, After: 4},
			},
		},
		{
			title: "both",
			first: 10,
			last:  12,
			side:  git.LinesBoth,
			expected: git.LineNumbers{
				{Before: 10, After: 10},
				{Before: 11, After: 11},
				{Before: 12, After: 12},
			},
		},
		{
			title:    "negative range",
			first:    5,
			last:     3,
			side:     git.LinesBoth,
			expected: git.LineNumbers{},
		},
		{
			title: "single element",
			first: 4,
			last:  4,
			side:  git.LinesAfter,
			expected: git.LineNumbers{
				{Before: 0, After: 4},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			require.Equal(
				t, tc.expected,
				git.MakeLineRangeFromTo(tc.first, tc.last, tc.side),
			)
		})
	}
}
