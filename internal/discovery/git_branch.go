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

	results := FileCommits{
		pathCommits: map[string][]string{},
	}
	if cr.IsEmpty() {
		log.Warn().Msg("Empty commit range, nothing to do")
		return results, nil
	}

	log.Debug().Str("from", cr.From).Str("to", cr.To).Msg("Got commit range from git")

	out, err := gd.gitCmd("log", "--no-merges", "--pretty=format:%H", "--name-status", "--diff-filter=d", cr.String())
	if err != nil {
		return nil, err
	}

	var commit string
	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.Split(removeRedundantSpaces(line), " ")
		if len(parts) == 1 && parts[0] != "" {
			commit = parts[0]
		} else if len(parts) >= 2 {
			path := parts[len(parts)-1]
			log.Debug().
				Str("path", path).
				Str("commit", commit).
				Bool("allowed", gd.isPathAllowed(path)).
				Msg("Git file change")
			if !gd.isPathAllowed(path) {
				continue
			}
			if _, ok := results.pathCommits[path]; !ok {
				results.pathCommits[path] = []string{}
			}
			results.pathCommits[path] = append(results.pathCommits[path], commit)
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
