package discovery

import "sort"

func NewFileCommitsFromMap(m map[string][]string) FileCommits {
	return FileCommits{pathCommits: m}
}

type FileCommits struct {
	pathCommits map[string][]string
}

func (fc FileCommits) Results() (results []File) {
	for path, commits := range fc.pathCommits {
		results = append(results, File{Path: path, Commits: commits})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Path < results[j].Path
	})
	return
}

func (fc FileCommits) Paths() (paths []string) {
	for path := range fc.pathCommits {
		paths = append(paths, path)
	}

	sort.Strings(paths)
	return
}

func (fc FileCommits) Commits() (commits []string) {
	cm := map[string]struct{}{}
	for _, cs := range fc.pathCommits {
		for _, c := range cs {
			cm[c] = struct{}{}
		}
	}

	for c := range cm {
		commits = append(commits, c)
	}

	sort.Strings(commits)
	return
}

func (fc FileCommits) HasCommit(commit string) bool {
	for _, commits := range fc.pathCommits {
		for _, c := range commits {
			if c == commit {
				return true
			}
		}
	}
	return false
}
