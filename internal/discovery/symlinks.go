package discovery

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
)

type symlink struct {
	from string
	to   string
}

func findSymlinks() (slinks []symlink, err error) {
	err = filepath.WalkDir(".",
		func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if d.Type() != fs.ModeSymlink {
				return nil
			}

			dest, err := filepath.EvalSymlinks(path)
			if err != nil {
				return fmt.Errorf("%s is a symlink but target file cannot be evaluated: %w", path, err)
			}

			info, err := os.Stat(dest)
			if err != nil {
				return fmt.Errorf("%s is a symlink but target file cannot be read: %w", path, err)
			}

			if !info.IsDir() {
				slinks = append(slinks, symlink{from: path, to: dest})
			}

			return nil
		})

	return slinks, err
}

func addSymlinkedEntries(entries []Entry) ([]Entry, error) {
	slinks, err := findSymlinks()
	if err != nil {
		return nil, err
	}

	nentries := []Entry{}
	for _, entry := range entries {
		if entry.State == Removed {
			continue
		}
		if entry.PathError != nil {
			continue
		}
		if entry.Rule.Error.Err != nil {
			continue
		}
		if entry.Path.Name != entry.Path.SymlinkTarget {
			continue
		}

		for _, sl := range slinks {
			if sl.to == entry.Path.Name {
				slog.Debug("Found a symlink", slog.String("to", sl.to), slog.String("from", sl.from))
				nentries = append(nentries, Entry{
					State: entry.State,
					Path: Path{
						Name:          sl.from,
						SymlinkTarget: sl.to,
					},
					ModifiedLines:  entry.ModifiedLines,
					Rule:           entry.Rule,
					Owner:          entry.Owner,
					DisabledChecks: entry.DisabledChecks,
				})
			}
		}
	}

	return nentries, nil
}
