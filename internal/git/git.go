package git

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os/exec"
	"strings"
)

type CommandRunner func(args ...string) ([]byte, error)

func RunGit(args ...string) (content []byte, err error) {
	slog.LogAttrs(context.Background(), slog.LevelDebug, "Running git command", slog.Any("args", args))
	cmd := exec.Command("git", args...)
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

func HeadCommit(cmd CommandRunner) (string, error) {
	commit, err := cmd("rev-parse", "--verify", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.Trim(string(commit), "\n"), nil
}

func CurrentBranch(cmd CommandRunner) (string, error) {
	commit, err := cmd("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.Trim(string(commit), "\n"), nil
}

func CommitMessage(cmd CommandRunner, sha string) (string, error) {
	msg, err := cmd("show", "-s", "--format=%B", sha)
	if err != nil {
		return "", err
	}
	return string(msg), err
}
