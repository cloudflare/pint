package discovery

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/cloudflare/pint/internal/git"

	"github.com/rs/zerolog/log"
)

func NewGitBranchFinder(gitCmd git.CommandRunner, include []*regexp.Regexp, baseBranch string, maxCommits int) GitBranchFinder {
	return GitBranchFinder{
		gitCmd:     gitCmd,
		include:    include,
		baseBranch: baseBranch,
		maxCommits: maxCommits,
	}
}

type GitBranchFinder struct {
	gitCmd     git.CommandRunner
	include    []*regexp.Regexp
	baseBranch string
	maxCommits int
}

func (f GitBranchFinder) Find() (entries []Entry, err error) {
	cr, err := git.CommitRange(f.gitCmd, f.baseBranch)
	if err != nil {
		return nil, fmt.Errorf("failed to get the list of commits to scan: %w", err)
	}

	log.Debug().Str("from", cr.From).Str("to", cr.To).Msg("Got commit range from git")

	if f.maxCommits > 0 && len(cr.Commits) > f.maxCommits {
		return nil, fmt.Errorf("number of commits to check (%d) is higher than maxCommits (%d), exiting", len(cr.Commits), f.maxCommits)
	}

	out, err := f.gitCmd("log", "--reverse", "--no-merges", "--pretty=format:%H", "--name-status", cr.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get the list of modified files from git: %w", err)
	}

	pathCommits := map[string]map[string]struct{}{}
	var commit string

	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.Split(removeRedundantSpaces(line), " ")
		if len(parts) == 1 && parts[0] != "" {
			commit = parts[0]
		} else if len(parts) >= 2 {
			op := parts[0]
			srcPath := parts[1]
			dstPath := parts[len(parts)-1]
			log.Debug().
				Str("path", dstPath).
				Str("commit", commit).
				Bool("allowed", f.isPathAllowed(dstPath)).
				Msg("Git file change")
			if !f.isPathAllowed(dstPath) {
				continue
			}
			if _, ok := pathCommits[dstPath]; !ok {
				pathCommits[dstPath] = map[string]struct{}{}
			}
			// check if we're dealing with a rename and if so we need to
			// rename results in pathCommits
			if strings.HasPrefix(op, "R") {
				if commits, ok := pathCommits[srcPath]; ok {
					for c := range commits {
						pathCommits[dstPath][c] = struct{}{}
					}
					delete(pathCommits, srcPath)
				}
			}
			// check if file is being removed, if so drop it from the results
			if strings.HasPrefix(op, "D") {
				delete(pathCommits, srcPath)
				continue
			}
			pathCommits[dstPath][commit] = struct{}{}
		}
	}

	for path, commits := range pathCommits {
		lbs, err := git.Blame(path, f.gitCmd)
		if err != nil {
			return nil, fmt.Errorf("failed to run git blame for %s: %w", path, err)
		}

		alloweLines := []int{}
		for _, lb := range lbs {
			// skip commits that are not part of our diff
			if _, ok := commits[lb.Commit]; !ok {
				continue
			}
			alloweLines = append(alloweLines, lb.Line)
		}

		els, err := readFile(path)
		if err != nil {
			return nil, err
		}
		for _, e := range els {
			e.ModifiedLines = alloweLines
			if isOverlap(alloweLines, e.Rule.Lines()) {
				entries = append(entries, e)
			}
		}
	}

	return entries, nil
}

func (f GitBranchFinder) isPathAllowed(path string) bool {
	if len(f.include) == 0 {
		return true
	}

	for _, pattern := range f.include {
		if pattern.MatchString(path) {
			return true
		}
	}
	return false
}

func removeRedundantSpaces(line string) string {
	return strings.Join(strings.Fields(line), " ")
}

func isOverlap(a, b []int) bool {
	for _, i := range a {
		for _, j := range b {
			if i == j {
				return true
			}
		}
	}
	return false
}
