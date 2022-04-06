package discovery

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
)

func NewGlobFinder(patterns []string, relaxed []*regexp.Regexp) GlobFinder {
	return GlobFinder{
		patterns: patterns,
		relaxed:  relaxed,
	}
}

type GlobFinder struct {
	patterns []string
	relaxed  []*regexp.Regexp
}

func (f GlobFinder) Find() (entries []Entry, err error) {
	paths := []string{}
	for _, p := range f.patterns {
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
				subpaths, err := walkDir(path)
				if err != nil {
					return nil, err
				}
				paths = append(paths, subpaths...)
			} else {
				paths = append(paths, path)
			}
		}
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf("no matching files")
	}

	for _, path := range paths {
		el, err := readFile(path, !matchesAny(f.relaxed, path))
		if err != nil {
			return nil, fmt.Errorf("invalid file syntax: %w", err)
		}
		for _, e := range el {
			if len(e.ModifiedLines) == 0 {
				e.ModifiedLines = e.Rule.Lines()
			}
			entries = append(entries, e)
		}
	}

	return entries, nil
}

func walkDir(dirname string) (paths []string, err error) {
	err = filepath.WalkDir(dirname,
		func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if d.IsDir() {
				return nil
			}

			paths = append(paths, path)
			return nil
		})

	return
}
