package discovery

import (
	"context"
	"io/fs"
	"log/slog"
	"path/filepath"
)

type symlink struct {
	err  error
	from string
	to   string
}

func findSymlinks() []symlink {
	var slinks []symlink
	_ = filepath.WalkDir(".",
		func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				slinks = append(slinks, symlink{from: path, to: "", err: err})
				return nil
			}

			if d.Type() != fs.ModeSymlink {
				return nil
			}

			dest, info, err := resolveFileInfo(path, path)
			if err != nil {
				slinks = append(slinks, symlink{from: path, to: "", err: err})
				return nil
			}

			if !info.IsDir() {
				slinks = append(slinks, symlink{from: path, to: dest, err: nil})
			}

			return nil
		})

	return slinks
}

func addSymlinkedEntries(entries []*Entry) []*Entry {
	slinks := findSymlinks()

	nentries := []*Entry{}
	for _, sl := range slinks {
		// Broken symlink or unreadable target, report as a file error.
		if sl.err != nil {
			nentries = append(nentries, &Entry{
				State: Modified,
				Path: Path{
					Name:          sl.from,
					SymlinkTarget: sl.from,
				},
				PathError: sl.err,
			})
			continue
		}
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
			if sl.to == entry.Path.Name {
				slog.LogAttrs(context.Background(), slog.LevelDebug, "Found a symlink", slog.String("to", sl.to), slog.String("from", sl.from))
				nentries = append(nentries, &Entry{
					State: entry.State,
					Path: Path{
						Name:          sl.from,
						SymlinkTarget: sl.to,
					},
					Changes:        entry.Changes,
					Rule:           entry.Rule,
					Owner:          entry.Owner,
					DisabledChecks: entry.DisabledChecks,
				})
			}
		}
	}

	return nentries
}
