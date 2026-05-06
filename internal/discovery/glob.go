package discovery

import (
	"context"
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

func NewGlobFinder(patterns []string, filter git.PathFilter, opts parser.Options, allowedOwners []*regexp.Regexp) GlobFinder {
	return GlobFinder{
		patterns:      patterns,
		filter:        filter,
		opts:          opts,
		allowedOwners: allowedOwners,
	}
}

type GlobFinder struct {
	filter        git.PathFilter
	patterns      []string
	allowedOwners []*regexp.Regexp
	opts          parser.Options
}

func (f GlobFinder) Find() (entries []*Entry, err error) {
	// Collect unique file paths from all glob patterns.
	paths := filePaths{}
	for _, p := range f.patterns {
		matches, err := filepath.Glob(p)
		if err != nil {
			return nil, fmt.Errorf("failed to expand file path pattern %s: %w", p, err)
		}

		for _, path := range matches {
			if path == ".git" && isDir(path) {
				slog.LogAttrs(
					context.Background(), slog.LevelDebug,
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

	// Parse each allowed file and extract rule entries.
	for _, fp := range paths {
		if !f.filter.IsPathAllowed(fp.path) {
			continue
		}

		if fp.err != nil {
			entries = append(entries, &Entry{
				State: Noop,
				Path: Path{
					Name:          fp.path,
					SymlinkTarget: fp.path,
				},
				PathError: fp.err,
			})
			continue
		}

		fd, err := os.Open(fp.path)
		if err != nil {
			entries = append(entries, &Entry{
				State: Noop,
				Path: Path{
					Name:          fp.path,
					SymlinkTarget: fp.path,
				},
				PathError: err,
			})
			continue
		}
		p := parser.NewParser(f.opts.WithStrict(!f.filter.IsRelaxed(fp.target)))
		entries = append(entries, readRules(fp.target, fp.path, fd, p, f.allowedOwners, nil)...)
		fd.Close()
	}

	slog.LogAttrs(context.Background(), slog.LevelDebug, "Glob finder completed", slog.Int("count", len(entries)))
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
	err    error
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

// resolveFileInfo resolves symlinks and stats the given path.
// Returns the resolved target path and file info.
// statPath is the path to os.Stat — it can differ from the EvalSymlinks input
// (e.g. walkDir stats the resolved dest, findFiles stats the original path).
func resolveFileInfo(evalPath, statPath string) (target string, info os.FileInfo, err error) {
	target, err = filepath.EvalSymlinks(evalPath)
	if err != nil {
		return "", nil, fmt.Errorf("this is a symlink but target file cannot be evaluated: %w", err)
	}

	info, err = os.Stat(statPath)
	if err != nil {
		return "", nil, err
	}

	return target, info, nil
}

func findFiles(path string) (filePaths, error) {
	target, info, err := resolveFileInfo(path, path)
	if err != nil {
		return filePaths{{
			err:    err,
			path:   path,
			target: "",
		}}, nil
	}

	var paths filePaths

	// nolint: exhaustive
	switch {
	case info.IsDir():
		subpaths, err := walkDir(path)
		if err != nil {
			return nil, err
		}
		paths = append(paths, subpaths...)
	default:
		paths = append(paths, filePath{err: nil, path: path, target: target})
	}

	return paths, nil
}

func walkDir(dirname string) (paths filePaths, err error) {
	err = filepath.WalkDir(dirname,
		func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			// Skip directories - WalkDir recurses into them automatically.
			if d.Type() == fs.ModeDir {
				return nil
			}

			dest, info, err := resolveFileInfo(path, path)
			if err != nil {
				paths = append(paths, filePath{
					err:    err,
					path:   path,
					target: "",
				})
				return nil
			}
			// Symlink pointing to a directory, recurse manually since WalkDir won't follow symlinks.
			if info.IsDir() {
				subpaths, err := findFiles(dest)
				if err != nil {
					return err
				}
				paths = append(paths, subpaths...)
			} else {
				paths = append(paths, filePath{err: nil, path: path, target: dest})
			}

			return nil
		})

	return paths, err
}
