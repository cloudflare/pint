package git

import (
	"bufio"
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path"
	"strings"

	"golang.org/x/exp/slices"
)

type FileStatus rune

const (
	FileAdded       FileStatus = 'A'
	FileCopied      FileStatus = 'C'
	FileDeleted     FileStatus = 'D'
	FileRenamed     FileStatus = 'R'
	FileModified    FileStatus = 'M'
	FileTypeChanged FileStatus = 'T'
)

type PathType int

const (
	Missing PathType = iota
	Dir
	File
	Symlink
)

type TypeDiff struct {
	Before PathType
	After  PathType
}

type BodyDiff struct {
	Before        []byte
	After         []byte
	ModifiedLines []int
}

type Path struct {
	Name          string
	Type          PathType
	SymlinkTarget string
}

func (p Path) EffectivePath() string {
	if p.SymlinkTarget != "" && p.Name != p.SymlinkTarget {
		return p.SymlinkTarget
	}
	return p.Name
}

type PathDiff struct {
	Before Path
	After  Path
}

type FileChange struct {
	Commits []string
	Path    PathDiff
	Body    BodyDiff
}

func Changes(cmd CommandRunner, cr CommitRangeResults) ([]*FileChange, error) {
	out, err := cmd("log", "--reverse", "--no-merges", "--format=%H", "--name-status", cr.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get the list of modified files from git: %w", err)
	}

	var changes []*FileChange
	var commit string
	s := bufio.NewScanner(bytes.NewReader(out))
	for s.Scan() {
		line := s.Text()

		parts := strings.Split(line, "\t")

		if len(parts) == 0 {
			continue
		}

		if len(parts) == 1 {
			if parts[0] != "" {
				commit = parts[0]
			}
			continue
		}

		status := FileStatus(parts[0][0])
		srcPath := parts[1]
		dstPath := parts[len(parts)-1]
		slog.Debug("Git file change", slog.String("change", parts[0]), slog.String("path", dstPath), slog.String("commit", commit))

		// ignore directories
		if isDir, _ := isDirectoryPath(dstPath); isDir {
			slog.Debug("Skipping directory entry change", slog.String("path", dstPath))
			continue
		}

		change := getChangeByPath(changes, dstPath)
		if change == nil {
			beforeType := getTypeForPath(cmd, commit+"^", srcPath)
			change = &FileChange{
				Path: PathDiff{
					Before: Path{
						Name:          srcPath,
						Type:          beforeType,
						SymlinkTarget: resolveSymlinkTarget(cmd, commit+"^", srcPath, beforeType),
					},
					After: Path{
						Name: dstPath,
					},
				},
			}
			switch status {
			case FileAdded:
				// newly added file, there's no "BEFORE" version
			case FileCopied:
				// file copied from other location, there's no "BEFORE" version
			case FileDeleted:
				// delete file, there's no "AFTER" version
				change.Body.Before = getContentAtCommit(cmd, commit+"^", change.Path.Before.SymlinkTarget)
			case FileModified:
				// modified file, there's both "BEFORE" and "AFTER"
				change.Body.Before = getContentAtCommit(cmd, commit+"^", change.Path.Before.SymlinkTarget)
			case FileRenamed:
				// rename could be only partial so there's both "BEFORE" and "AFTER"
				change.Body.Before = getContentAtCommit(cmd, commit+"^", change.Path.Before.SymlinkTarget)
			case FileTypeChanged:
				// type change, could be file -> dir or symlink -> file
				// so there's both "BEFORE" and "AFTER"
				change.Body.Before = getContentAtCommit(cmd, commit+"^", change.Path.Before.SymlinkTarget)
			default:
				slog.Debug("Unknown git change", slog.String("path", dstPath), slog.String("commit", commit), slog.String("change", parts[0]))
			}
			changes = append(changes, change)
		}
		change.Commits = append(change.Commits, commit)
	}
	slog.Debug("Parsed git log", slog.Int("changes", len(changes)))

	for _, change := range changes {
		lastCommit := change.Commits[len(change.Commits)-1]

		change.Path.After.Type = getTypeForPath(cmd, lastCommit, change.Path.After.Name)
		change.Path.After.SymlinkTarget = resolveSymlinkTarget(cmd, lastCommit, change.Path.After.Name, change.Path.After.Type)
		change.Body.After = getContentAtCommit(cmd, lastCommit, change.Path.After.EffectivePath())

		switch {
		case change.Path.Before.Type != Missing && change.Path.After.Type == Symlink:
			// file was turned into a symlink, every source line is modification
			change.Body.ModifiedLines = CountLines(change.Body.After)
		case change.Path.Before.Type != Missing && change.Path.After.Type != Missing && change.Path.After.Type != Symlink:
			change.Body.ModifiedLines, err = getModifiedLines(cmd, change.Commits, change.Path.After.EffectivePath())
			if err != nil {
				return nil, fmt.Errorf("failed to run git blame for %s: %w", change.Path.After.EffectivePath(), err)
			}
		case change.Path.Before.Type == Symlink && change.Path.After.Type == Symlink:
			// symlink was modified, every source line is modification
			change.Body.ModifiedLines = CountLines(change.Body.After)
		case change.Path.Before.Type == Missing && change.Path.After.Type != Missing:
			// old file body is empty, meaning that every line was modified
			change.Body.ModifiedLines = CountLines(change.Body.After)
		case change.Path.Before.Type != Missing && change.Path.After.Type == Missing:
			// new file body is empty, meaning that every line was modified
			change.Body.ModifiedLines = CountLines(change.Body.Before)
		default:
			slog.Debug("Unhandled change", slog.String("change", fmt.Sprintf("+%v", change)))
		}

		if change.Path.Before.Name == change.Path.Before.SymlinkTarget {
			change.Path.Before.SymlinkTarget = ""
		}
		if change.Path.After.Name == change.Path.After.SymlinkTarget {
			change.Path.After.SymlinkTarget = ""
		}
	}

	return changes, nil
}

func getChangeByPath(changes []*FileChange, fpath string) *FileChange {
	for _, c := range changes {
		if c.Path.After.Name == fpath {
			return c
		}
	}
	return nil
}

func getModifiedLines(cmd CommandRunner, commits []string, fpath string) ([]int, error) {
	slog.Debug("Getting list of modified lines", slog.String("commits", fmt.Sprint(commits)), slog.String("path", fpath))
	lines, err := Blame(cmd, fpath)
	if err != nil {
		return nil, err
	}

	modLines := make([]int, 0, len(lines))
	for _, line := range lines {
		if !slices.Contains(commits, line.Commit) {
			continue
		}
		modLines = append(modLines, line.Line)
	}
	return modLines, nil
}

func getTypeForPath(cmd CommandRunner, commit, fpath string) PathType {
	args := []string{"ls-tree", "--format=%(objectmode) %(objecttype) %(path)", commit, fpath}
	out, err := cmd(args...)
	if err != nil {
		slog.Debug("git command returned an error", slog.Any("err", err), slog.String("args", fmt.Sprint(args)))
		return Missing
	}

	s := bufio.NewScanner(bytes.NewReader(out))
	for s.Scan() {
		parts := strings.SplitN(s.Text(), " ", 3)
		if len(parts) != 3 {
			continue
		}
		objmode := parts[0]
		objtype := parts[1]
		objpath := parts[2]

		// not our file
		if objpath != fpath {
			continue
		}
		if objtype == "tree" {
			return Dir
		}
		// not a blob - could be a tree or a tag
		if objtype != "blob" {
			continue
		}

		if objmode == "120000" {
			return Symlink
		}

		return File
	}

	return Missing
}

// recursively find the final target of a symlink
func resolveSymlinkTarget(cmd CommandRunner, commit, fpath string, typ PathType) string {
	if typ != Symlink {
		return fpath
	}
	raw := string(getContentAtCommit(cmd, commit, fpath))
	spath := path.Clean(path.Join(path.Dir(fpath), raw))
	stype := getTypeForPath(cmd, commit, spath)
	return resolveSymlinkTarget(cmd, commit, spath, stype)
}

func getContentAtCommit(cmd CommandRunner, commit, fpath string) []byte {
	args := []string{"cat-file", "blob", fmt.Sprintf("%s:%s", commit, fpath)}
	body, err := cmd(args...)
	if err != nil {
		slog.Debug("git command returned an error", slog.Any("err", err), slog.String("args", fmt.Sprint(args)))
		return nil
	}
	return body
}

func CountLines(body []byte) (lines []int) {
	var line int
	s := bufio.NewScanner(bytes.NewReader(body))
	for s.Scan() {
		line++
		lines = append(lines, line)
	}
	return lines
}

func isDirectoryPath(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	return fileInfo.IsDir(), err
}
