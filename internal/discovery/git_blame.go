package discovery

import (
	"fmt"
	"sort"

	"github.com/cloudflare/pint/internal/git"
)

type GitBlameLines struct {
	lines []Line
}

func (gbl GitBlameLines) Results() []Line {
	return gbl.lines
}

func (gbl GitBlameLines) HasLines(lines []int) bool {
	for _, line := range lines {
		for _, lc := range gbl.lines {
			if lc.Position == line {
				return true
			}
		}
	}
	return false
}

func NewGitBlameLineFinder(cmd git.CommandRunner, allowedCommits []string) *GitBlameLineFinder {
	return &GitBlameLineFinder{gitCmd: cmd, allowedCommits: allowedCommits}
}

type GitBlameLineFinder struct {
	gitCmd         git.CommandRunner
	allowedCommits []string
}

func (gbd *GitBlameLineFinder) Find(path string) (LineFindResults, error) {
	results := GitBlameLines{}

	lbs, err := git.Blame(path, gbd.gitCmd)
	if err != nil {
		return nil, fmt.Errorf("failed to run git blame for %s: %s", path, err)
	}

	for _, lb := range lbs {
		if !gbd.isCommitAllowed(lb.Commit) {
			continue
		}
		results.lines = append(results.lines, Line{Path: path, Position: lb.Line, Commit: lb.Commit})
	}

	sort.Slice(results.lines, func(i, j int) bool {
		return results.lines[i].Position < results.lines[j].Position
	})

	return results, nil
}

func (gbd *GitBlameLineFinder) isCommitAllowed(commit string) bool {
	for _, c := range gbd.allowedCommits {
		if c == commit {
			return true
		}
	}
	return false
}
