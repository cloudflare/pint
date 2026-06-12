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

func Describe(ctx context.Context, cmd CommandRunner) (Info, error) {
	commit, err := cmd(ctx, "rev-parse", "--verify", "HEAD")
	if err != nil {
		return Info{}, err
	}

	branch, err := cmd(ctx, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return Info{}, err
	}

	return Info{
		HeadCommit:    strings.Trim(string(commit), "\n"),
		CurrentBranch: strings.Trim(string(branch), "\n"),
	}, nil
}

func CommitMessage(ctx context.Context, cmd CommandRunner, sha string) (string, error) {
	msg, err := cmd(ctx, "show", "-s", "--format=%B", sha)
	if err != nil {
		return "", err
	}
	return string(msg), err
}
