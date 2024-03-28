package git

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
)

type LineBlame struct {
	Filename string
	Commit   string
	PrevLine int
	Line     int
}

type LineBlames []LineBlame

type FileBlames map[string]LineBlames

type CommandRunner func(args ...string) ([]byte, error)

func RunGit(args ...string) (content []byte, err error) {
	slog.Debug("Running git command", slog.Any("args", args))
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

func Blame(cmd CommandRunner, path, commit string) (lines LineBlames, err error) {
	slog.Debug("Running git blame", slog.String("path", path))
	output, err := cmd("blame", "--line-porcelain", commit, "--", path)
	if err != nil {
		return nil, err
	}

	buf := bytes.NewReader(output)
	scanner := bufio.NewScanner(buf)
	var line string
	var cl LineBlame
	for scanner.Scan() {
		line = scanner.Text()

		switch {
		case strings.HasPrefix(line, "author"):
			continue
		case strings.HasPrefix(line, "committer"):
			continue
		case strings.HasPrefix(line, "summary"):
			continue
		case strings.HasPrefix(line, "filename"):
			cl.Filename = strings.Split(line, " ")[1]
		case strings.HasPrefix(line, "previous"):
			continue
		case strings.HasPrefix(line, "boundary"):
			continue
		case strings.HasPrefix(line, "\t"):
			lines = append(lines, cl)
			cl.PrevLine = 0
		default:
			parts := strings.Split(line, " ")
			if len(parts) < 3 {
				return nil, fmt.Errorf("failed to parse line number from line: %q", line)
			}
			cl.Commit = parts[0]
			if cl.PrevLine, err = strconv.Atoi(parts[1]); err != nil {
				return nil, fmt.Errorf("failed to parse line number from %q: %w", line, err)
			}
			if cl.Line, err = strconv.Atoi(parts[2]); err != nil {
				return nil, fmt.Errorf("failed to parse line number from %q: %w", line, err)
			}
		}
	}
	if err = scanner.Err(); err != nil {
		return nil, err
	}

	return lines, nil
}

func HeadCommit(cmd CommandRunner) (string, error) {
	commit, err := cmd("rev-parse", "--verify", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.Trim(string(commit), "\n"), nil
}

type CommitRangeResults struct {
	From    string
	To      string
	Commits []string
}

func (gcr CommitRangeResults) String() string {
	return fmt.Sprintf("%s^..%s", gcr.From, gcr.To)
}

func CommitRange(cmd CommandRunner, baseBranch string) (CommitRangeResults, error) {
	cr := CommitRangeResults{Commits: []string{}}

	out, err := cmd("log", "--format=%H", "--no-abbrev-commit", "--reverse", fmt.Sprintf("%s..HEAD", baseBranch))
	if err != nil {
		return cr, err
	}

	for _, line := range strings.Split(strings.TrimSuffix(string(out), "\n"), "\n") {
		if line != "" {
			cr.Commits = append(cr.Commits, line)
			if cr.From == "" {
				cr.From = line
			}
			cr.To = line
			slog.Debug("Found commit to scan", slog.String("commit", line))
		}
	}

	if len(cr.Commits) == 0 {
		return cr, fmt.Errorf("empty commit range")
	}

	return cr, nil
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
