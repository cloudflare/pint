package discovery

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func NewGlobFinder(patterns ...string) GlobFinder {
	return GlobFinder{
		patterns: patterns,
	}
}

type GlobFinder struct {
	patterns []string
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
		e, err := readFile(path)
		if err != nil {
			return nil, err
		}
		entries = append(entries, e...)
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
