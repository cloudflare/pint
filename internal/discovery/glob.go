package discovery

import (
	"os"
	"path/filepath"
)

func NewGlobFileFinder() GlobFileFinder {
	return GlobFileFinder{}
}

type GlobFileFinder struct {
}

func (gd GlobFileFinder) Find(pattern ...string) (FileFindResults, error) {
	results := FileCommits{
		pathCommits: map[string][]string{},
	}

	for _, p := range pattern {
		matches, err := filepath.Glob(p)
		if err != nil {
			return nil, err
		}

		for _, path := range matches {
			s, err := os.Stat(path)
			if err != nil {
				return nil, err
			}
			if s.IsDir() {
				err = filepath.Walk(path,
					func(path string, info os.FileInfo, err error) error {
						if err != nil {
							return err
						}

						if info.IsDir() {
							return nil
						}

						results.pathCommits[path] = nil

						return nil
					})
				if err != nil {
					return nil, err
				}
			} else {
				results.pathCommits[path] = nil
			}
		}
	}

	return results, nil
}
