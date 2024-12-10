package discovery

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"

	"github.com/cloudflare/pint/internal/git"
	"github.com/cloudflare/pint/internal/parser"
)

func NewGlobFinder(patterns []string, filter git.PathFilter, schema parser.Schema, allowedOwners []*regexp.Regexp) GlobFinder {
	return GlobFinder{
		patterns:      patterns,
		filter:        filter,
		schema:        schema,
		allowedOwners: allowedOwners,
	}
}

type GlobFinder struct {
	filter        git.PathFilter
	patterns      []string
	allowedOwners []*regexp.Regexp
	schema        parser.Schema
}

func (f GlobFinder) Find() (entries []Entry, err error) {
	paths := filePaths{}
	for _, p := range f.patterns {
		matches, err := filepath.Glob(p)
		if err != nil {
			return nil, fmt.Errorf("failed to expand file path pattern %s: %w", p, err)
		}

		for _, path := range matches {
			if path == ".git" && isDir(path) {
				slog.Debug(
					"Excluding git directory from glob results",
					slog.String("path", path),
					slog.String("glob", p),
				)
				continue
			}

			subpaths, err := findFiles(path)
			if err != nil {
				return nil, err
			}
			for _, subpath := range subpaths {
				if !paths.hasPath(subpath.path) {
					paths = append(paths, subpath)
				}
			}
		}
	}

	if len(paths) == 0 {
		return nil, errors.New("no matching files")
	}

	for _, fp := range paths {
		if !f.filter.IsPathAllowed(fp.path) {
			continue
		}

		fd, err := os.Open(fp.path)
		if err != nil {
			return nil, err
		}
		el, err := readRules(fp.target, fp.path, fd, !f.filter.IsRelaxed(fp.target), f.schema, f.allowedOwners)
		if err != nil {
			fd.Close()
			return nil, fmt.Errorf("invalid file syntax: %w", err)
		}
		fd.Close()
		for _, e := range el {
			e.State = Noop
			if len(e.ModifiedLines) == 0 {
				e.ModifiedLines = e.Rule.Lines.Expand()
			}
			entries = append(entries, e)
		}
	}

	slog.Debug("Glob finder completed", slog.Int("count", len(entries)))
	return entries, nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

type filePath struct {
	path   string
	target string
}

type filePaths []filePath

func (fps filePaths) hasPath(p string) bool {
	for _, fp := range fps {
		if fp.path == p {
			return true
		}
	}
	return false
}

func findFiles(path string) (paths filePaths, err error) {
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, fmt.Errorf("%s is a symlink but target file cannot be evaluated: %w", path, err)
	}

	s, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	// nolint: exhaustive
	switch {
	case s.IsDir():
		subpaths, err := walkDir(path)
		if err != nil {
			return nil, err
		}
		paths = append(paths, subpaths...)
	default:
		paths = append(paths, filePath{path: path, target: target})
	}

	return paths, nil
}

func walkDir(dirname string) (paths filePaths, err error) {
	err = filepath.WalkDir(dirname,
		func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			// nolint: exhaustive
			switch d.Type() {
			case fs.ModeDir:
				return nil
			default:
				dest, err := filepath.EvalSymlinks(path)
				if err != nil {
					return fmt.Errorf("%s is a symlink but target file cannot be evaluated: %w", path, err)
				}

				s, err := os.Stat(dest)
				if err != nil {
					return err
				}
				if s.IsDir() {
					subpaths, err := findFiles(dest)
					if err != nil {
						return err
					}
					paths = append(paths, subpaths...)
				} else {
					paths = append(paths, filePath{path: path, target: dest})
				}
			}

			return nil
		})

	return paths, err
}
