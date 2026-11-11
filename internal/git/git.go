package git

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os/exec"
	"strings"
)

type CommandRunner func(ctx context.Context, args ...string) ([]byte, error)

func RunGit(ctx context.Context, args ...string) (content []byte, err error) {
	slog.LogAttrs(ctx, slog.LevelDebug, "Running git command", slog.Any("args", args))
	cmd := exec.CommandContext(ctx, "git", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err = cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return nil, errors.New(stderr.String())
		}
		return nil, err
	}
	return stdout.Bytes(), nil
}

type Info struct {
	HeadCommit    string
	CurrentBranch string
}

// Describe returns the current branch name and its last commit.
// targetBranch is the branch the current branch will be merged into. It is used
// to find the last commit when HEAD is a merge into targetBranch.
func Describe(ctx context.Context, cmd CommandRunner, targetBranch string) (Info, error) {
	commit, err := branchHead(ctx, cmd, targetBranch)
	if err != nil {
		return Info{}, err
	}

	branch, err := cmd(ctx, "rev-parse", "--abbrev-ref", HeadRef)
	if err != nil {
		return Info{}, err
	}

	return Info{
		HeadCommit:    commit,
		CurrentBranch: strings.Trim(string(branch), "\n"),
	}, nil
}

// branchHead returns HEAD, or its second parent if HEAD merges the branch into
// targetBranch, as in GitLab merged-result and GitHub pull request jobs.
func branchHead(ctx context.Context, cmd CommandRunner, targetBranch string) (string, error) {
	out, err := cmd(ctx, "rev-list", "--parents", "-n", "1", HeadRef)
	if err != nil {
		return "", err
	}
	commits := strings.Fields(string(out)) // HEAD followed by its parents.
	if len(commits) == 0 {
		return "", errors.New("no commit found for HEAD")
	}
	if len(commits) != 3 {
		return commits[0], nil
	}
	if _, err = cmd(ctx, "merge-base", "--is-ancestor", commits[1], targetBranch); err != nil {
		return commits[0], nil
	}
	slog.LogAttrs(ctx, slog.LevelDebug, "HEAD merges the current branch into the target branch, using its second parent", slog.String("commit", commits[2]))
	return commits[2], nil
}

func CommitMessage(ctx context.Context, cmd CommandRunner, sha string) (string, error) {
	msg, err := cmd(ctx, "show", "-s", "--format=%B", sha)
	if err != nil {
		return "", err
	}
	return string(msg), err
}
