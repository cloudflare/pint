package discovery

import (
	"regexp"
	"strings"

	"github.com/cloudflare/pint/internal/git"

	"github.com/rs/zerolog/log"
)

func NewGitBranchFileFinder(gitCmd git.CommandRunner, include []*regexp.Regexp, baseBranch string) GitBranchFileFinder {
	return GitBranchFileFinder{gitCmd: gitCmd, include: include, baseBranch: baseBranch}
}

type GitBranchFileFinder struct {
	gitCmd     git.CommandRunner
	include    []*regexp.Regexp
	baseBranch string
}

func (gd GitBranchFileFinder) Find(pattern ...string) (FileFindResults, error) {
	cr, err := git.CommitRange(gd.gitCmd, gd.baseBranch)
	if err != nil {
		return nil, err
	}

	log.Debug().Str("from", cr.From).Str("to", cr.To).Msg("Got commit range from git")

	out, err := gd.gitCmd("log", "--reverse", "--no-merges", "--pretty=format:%H", "--name-status", cr.String())
	if err != nil {
		return nil, err
	}

	results := FileCommits{
		pathCommits: map[string][]string{},
	}

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
				Bool("allowed", gd.isPathAllowed(dstPath)).
				Msg("Git file change")
			if !gd.isPathAllowed(dstPath) {
				continue
			}
			if _, ok := results.pathCommits[dstPath]; !ok {
				results.pathCommits[dstPath] = []string{}
			}
			// check if we're dealing with a rename and if so we need to
			// rename results in pathCommits
			if strings.HasPrefix(op, "R") {
				if v, ok := results.pathCommits[srcPath]; ok {
					results.pathCommits[dstPath] = append(results.pathCommits[dstPath], v...)
					delete(results.pathCommits, srcPath)
				}
			}
			// check if file is being removed, if so drop it from the results
			if strings.HasPrefix(op, "D") {
				delete(results.pathCommits, srcPath)
				continue
			}
			results.pathCommits[dstPath] = append(results.pathCommits[dstPath], commit)
		}
	}

	return results, nil
}

func (gd GitBranchFileFinder) isPathAllowed(path string) bool {
	if len(gd.include) == 0 {
		return true
	}

	for _, pattern := range gd.include {
		if pattern.MatchString(path) {
			return true
		}
	}
	return false
}

func removeRedundantSpaces(line string) string {
	return strings.Join(strings.Fields(line), " ")
}
