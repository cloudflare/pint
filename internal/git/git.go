package git

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
)

type LineBlame struct {
	Filename string
	Line     int
	Commit   string
}

type LineBlames []LineBlame

func (lbs LineBlames) GetCommit(line int) string {
	for _, lb := range lbs {
		if lb.Line == line {
			return lb.Commit
		}
	}
	return ""
}

type FileBlames map[string]LineBlames

type CommandRunner func(args ...string) ([]byte, error)

func RunGit(args ...string) (content []byte, err error) {
	log.Debug().Strs("args", args).Msg("Running git command")
	content, err = exec.Command("git", args...).Output()
	return content, err
}

func Blame(path string, cmd CommandRunner) (lines LineBlames, err error) {
	output, err := cmd("blame", "--line-porcelain", "--", path)
	if err != nil {
		return nil, err
	}

	buf := bytes.NewReader(output)
	scanner := bufio.NewScanner(buf)
	var line string
	var cl LineBlame
	for scanner.Scan() {
		line = scanner.Text()

		if strings.HasPrefix(line, "author") {
			continue
		} else if strings.HasPrefix(line, "committer") {
			continue
		} else if strings.HasPrefix(line, "summary") {
			continue
		} else if strings.HasPrefix(line, "filename") {
			cl.Filename = strings.Split(line, " ")[1]
		} else if strings.HasPrefix(line, "previous") {
			continue
		} else if strings.HasPrefix(line, "boundary") {
			continue
		} else if strings.HasPrefix(line, "\t") {
			if cl.Filename == path {
				lines = append(lines, cl)
			}
		} else {
			parts := strings.Split(line, " ")
			if len(parts) < 3 {
				return nil, fmt.Errorf("failed to parse line number from line: %q", line)
			}
			cl.Commit = parts[0]
			cl.Line, err = strconv.Atoi(parts[2])
			if err != nil {
				return nil, fmt.Errorf("failed to parse line number from %q: %s", line, err)
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
	From string
	To   string
}

func (gcr CommitRangeResults) String() string {
	return fmt.Sprintf("%s^..%s", gcr.From, gcr.To)
}

func CommitRange(cmd CommandRunner, baseBranch string) (cr CommitRangeResults, err error) {
	out, err := cmd("log", "--format=%H", "--no-abbrev-commit", "--reverse", fmt.Sprintf("%s..HEAD", baseBranch))
	if err != nil {
		return cr, err
	}

	lines := []string{}
	for _, line := range strings.Split(strings.TrimSuffix(string(out), "\n"), "\n") {
		if line != "" {
			lines = append(lines, line)
			log.Debug().Str("commit", line).Msg("Found commit to scan")
		}
	}

	if len(lines) == 0 {
		return cr, fmt.Errorf("empty commit range")
	}

	cr.From = lines[0]
	cr.To = lines[len(lines)-1]

	return
}

func CurrentBranch(cmd CommandRunner) (string, error) {
	commit, err := cmd("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.Trim(string(commit), "\n"), nil
}
