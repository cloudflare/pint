package discovery

import (
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"

	"github.com/cloudflare/pint/internal/git"
)

func NewGitBlameFinder(
	gitCmd git.CommandRunner,
	include []*regexp.Regexp,
	exclude []*regexp.Regexp,
	baseBranch string,
	maxCommits int,
	relaxed []*regexp.Regexp,
) GitBlameFinder {
	return GitBlameFinder{
		gitCmd:     gitCmd,
		include:    include,
		exclude:    exclude,
		baseBranch: baseBranch,
		maxCommits: maxCommits,
		relaxed:    relaxed,
	}
}

type GitBlameFinder struct {
	gitCmd     git.CommandRunner
	include    []*regexp.Regexp
	exclude    []*regexp.Regexp
	baseBranch string
	maxCommits int
	relaxed    []*regexp.Regexp
}

func (f GitBlameFinder) Find() (entries []Entry, err error) {
	cr, err := git.CommitRange(f.gitCmd, f.baseBranch)
	if err != nil {
		return nil, fmt.Errorf("failed to get the list of commits to scan: %w", err)
	}

	slog.Debug("Got commit range from git", slog.String("from", cr.From), slog.String("to", cr.To))

	if f.maxCommits > 0 && len(cr.Commits) > f.maxCommits {
		return nil, fmt.Errorf("number of commits to check (%d) is higher than maxCommits (%d), exiting", len(cr.Commits), f.maxCommits)
	}

	out, err := f.gitCmd("log", "--reverse", "--no-merges", "--pretty=format:%H", "--name-status", cr.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get the list of modified files from git: %w", err)
	}

	pathCommits := map[string]map[string]struct{}{}
	var commit, msg string

	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.Split(removeRedundantSpaces(line), " ")
		if len(parts) == 1 && parts[0] != "" {
			commit = parts[0]
		} else if len(parts) >= 2 {
			op := parts[0]
			srcPath := parts[1]
			dstPath := parts[len(parts)-1]
			slog.Debug("Git file change", slog.String("change", parts[0]), slog.String("path", dstPath), slog.String("commit", commit))
			if !f.isPathAllowed(dstPath) {
				slog.Debug("Skipping file due to include/exclude rules", slog.String("path", dstPath))
				continue
			}

			msg, err = git.CommitMessage(f.gitCmd, commit)
			if err != nil {
				return nil, fmt.Errorf("failed to get commit message for %s: %w", commit, err)
			}
			if strings.Contains(msg, "[skip ci]") {
				slog.Info("Found a commit with '[skip ci]', skipping all checks", slog.String("commit", commit))
				return []Entry{}, nil
			}
			if strings.Contains(msg, "[no ci]") {
				slog.Info("Found a commit with '[no ci]', skipping all checks", slog.String("commit", commit))
				return []Entry{}, nil
			}

			// ignore directories
			if isDir, _ := isDirectoryPath(dstPath); isDir {
				slog.Debug("Skipping directory entry change", slog.String("path", dstPath))
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

	var lbs git.LineBlames
	var els []Entry
	for path, commits := range pathCommits {
		lbs, err = git.Blame(f.gitCmd, path)
		if err != nil {
			return nil, fmt.Errorf("failed to run git blame for %s: %w", path, err)
		}

		allowedLines := []int{}
		for _, lb := range lbs {
			// skip commits that are not part of our diff
			if _, ok := commits[lb.Commit]; !ok {
				continue
			}
			allowedLines = append(allowedLines, lb.Line)
		}

		var fd *os.File
		fd, err = os.Open(path)
		if err != nil {
			return nil, err
		}

		els, err = readRules(path, path, fd, !matchesAny(f.relaxed, path))
		if err != nil {
			fd.Close()
			return nil, err
		}
		fd.Close()

		for _, e := range els {
			e.ModifiedLines = getOverlap(e.Rule.Lines(), allowedLines)
			if len(e.ModifiedLines) == 0 && e.PathError != nil {
				e.ModifiedLines = allowedLines
			}
			if isOverlap(allowedLines, e.Rule.Lines()) || isOverlap(allowedLines, e.ModifiedLines) {
				entries = append(entries, e)
			}
		}
	}

	symlinks, err := addSymlinkedEntries(entries)
	if err != nil {
		return nil, err
	}

	for _, entry := range symlinks {
		if f.isPathAllowed(entry.SourcePath) {
			entries = append(entries, entry)
		}
	}

	return entries, nil
}

func (f GitBlameFinder) isPathAllowed(path string) bool {
	if len(f.include) == 0 && len(f.exclude) == 0 {
		return true
	}

	for _, pattern := range f.exclude {
		if pattern.MatchString(path) {
			return false
		}
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

func getOverlap(a, b []int) (o []int) {
	for _, i := range a {
		for _, j := range b {
			if i == j {
				o = append(o, i)
			}
		}
	}
	return o
}

func isDirectoryPath(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	return fileInfo.IsDir(), err
}
