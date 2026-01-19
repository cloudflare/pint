package git

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path"
	"slices"
	"strings"
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

type PathType uint8

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
	SymlinkTarget string
	Type          PathType
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
	Path    PathDiff
	Body    BodyDiff
	Commits []string
	Status  FileStatus
}

func Changes(cmd CommandRunner, baseBranch string, filter PathFilter) ([]*FileChange, error) {
	out, err := cmd("log", "--reverse", "--no-merges", "--first-parent", "--format=%H", "--name-status", baseBranch+"..HEAD")
	if err != nil {
		return nil, fmt.Errorf("failed to get the list of modified files from git: %w", err)
	}

	var changes []*FileChange
	var commit string
	s := bufio.NewScanner(bytes.NewReader(out))
	for s.Scan() {
		line := s.Text()

		parts := strings.Split(line, "\t")
		// Split always returns at least 1 element slice.
		if len(parts) == 1 {
			if parts[0] != "" {
				commit = parts[0]
			}
			continue
		}

		status := FileStatus(parts[0][0])
		srcPath := parts[1]
		dstPath := parts[len(parts)-1]
		slog.LogAttrs(context.Background(), slog.LevelDebug, "Git file change", slog.String("change", parts[0]), slog.String("path", dstPath), slog.String("commit", commit))

		if !filter.IsPathAllowed(dstPath) {
			slog.LogAttrs(context.Background(), slog.LevelDebug, "Skipping file due to include/exclude rules", slog.String("path", dstPath))
			continue
		}

		// This should never really happen since git doesn't track directories, only files.
		if isDir, _ := isDirectoryPath(dstPath); isDir {
			slog.LogAttrs(context.Background(), slog.LevelDebug, "Skipping directory entry change", slog.String("path", dstPath))
			continue
		}

		// Rest is populated inside the next loop.
		change := &FileChange{ // nolint: exhaustruct
			Status: status,
			Path: PathDiff{ // nolint: exhaustruct
				After: Path{ // nolint: exhaustruct
					Name: dstPath,
				},
			},
		}

		prev := getChangeByPath(changes, srcPath)
		slog.LogAttrs(context.Background(), slog.LevelDebug, "Looking for previous changes",
			slog.String("src", srcPath),
			slog.String("dst", dstPath),
			slog.String("commit", commit),
		)
		if prev != nil {
			slog.LogAttrs(context.Background(), slog.LevelDebug, "Found a previous change",
				slog.Any("commits", prev.Commits),
				slog.String("status", string(prev.Status)),
				slog.String("path", prev.Path.Before.Name),
				slog.String("target", prev.Path.Before.SymlinkTarget),
				slog.Any("type", prev.Path.Before.Type),
			)
			change.Commits = append(change.Commits, prev.Commits...)
			change.Path.Before = prev.Path.Before
			// Remove any changes for "BEFORE" path we might already have
			changes = changesWithout(changes, srcPath)
		} else {
			slog.LogAttrs(context.Background(), slog.LevelDebug, "No previous change found")
			switch change.Status {
			case FileAdded, FileCopied:
				change.Path.Before.Name = ""
				change.Path.Before.SymlinkTarget = ""
				// If a path changed type we'll see A but we can still query for old type.
				change.Path.Before.Type = getTypeForPath(cmd, commit+"^", srcPath)
				if change.Path.Before.Type != Missing {
					// If it was a type change then
					change.Path.Before.Name = srcPath
					change.Path.Before.Type = getTypeForPath(cmd, commit+"^", srcPath)
				}
			case FileDeleted, FileRenamed, FileModified, FileTypeChanged:
				change.Path.Before.Name = srcPath
				change.Path.Before.Type = getTypeForPath(cmd, commit+"^", srcPath)
				change.Path.Before.SymlinkTarget = resolveSymlinkTarget(cmd, commit+"^", srcPath, change.Path.Before.Type)
			}
		}

		change.Commits = append(change.Commits, commit)

		changes = append(changes, change)
	}

	slog.LogAttrs(context.Background(), slog.LevelDebug, "Parsed git log", slog.Int("changes", len(changes)))

	for _, change := range changes {
		slog.LogAttrs(context.Background(), slog.LevelDebug,
			"File change",
			slog.Any("commits", change.Commits),
			slog.String("status", string(change.Status)),
			slog.String("before", change.Path.Before.Name),
			slog.String("after", change.Path.After.Name),
		)

		if change.Path.Before.Name != "" {
			change.Path.Before.Type = getTypeForPath(cmd, change.Commits[0]+"^", change.Path.Before.Name)
			change.Path.Before.SymlinkTarget = resolveSymlinkTarget(cmd, change.Commits[0]+"^", change.Path.Before.Name, change.Path.Before.Type)
			change.Body.Before = getContentAtCommit(cmd, change.Commits[0]+"^", change.Path.Before.EffectivePath())
		}

		lastCommit := change.Commits[len(change.Commits)-1]
		if change.Path.After.Name != "" && change.Status != FileDeleted {
			change.Path.After.Type = getTypeForPath(cmd, lastCommit, change.Path.After.Name)
			change.Path.After.SymlinkTarget = resolveSymlinkTarget(cmd, lastCommit, change.Path.After.Name, change.Path.After.Type)
			change.Body.After = getContentAtCommit(cmd, lastCommit, change.Path.After.EffectivePath())
		}

		slog.LogAttrs(context.Background(), slog.LevelDebug,
			"Updated file change",
			slog.Any("commits", change.Commits),
			slog.String("before.path", change.Path.Before.Name),
			slog.String("before.target", change.Path.Before.SymlinkTarget),
			slog.Any("before.type", change.Path.Before.Type),
			slog.String("before.body", string(change.Body.Before)),
			slog.String("after.path", change.Path.After.Name),
			slog.String("after.target", change.Path.After.SymlinkTarget),
			slog.Any("after.type", change.Path.After.Type),
			slog.String("after.body", string(change.Body.After)),
			slog.Any("modifiedLines", change.Body.ModifiedLines),
		)

		switch {
		case change.Path.Before.Type != Missing && change.Path.Before.Type != Symlink && change.Path.After.Type == Symlink:
			slog.LogAttrs(context.Background(), slog.LevelDebug, "Path was turned into a symlink", slog.String("path", change.Path.After.Name))
			change.Body.ModifiedLines = CountLines(change.Body.After)
		case change.Path.Before.Type != Missing && change.Path.After.Type != Missing && change.Path.After.Type != Symlink:
			change.Body.ModifiedLines, err = getModifiedLines(cmd, change.Commits, change.Path.After.EffectivePath(), lastCommit, change.Body.Before, change.Body.After)
			if err != nil {
				return nil, fmt.Errorf("failed to run git blame for %s: %w", change.Path.After.EffectivePath(), err)
			}
			if len(change.Body.ModifiedLines) == 0 && change.Path.Before.EffectivePath() != change.Path.After.EffectivePath() {
				// File was moved or renamed. Mark it all as modified.
				change.Body.ModifiedLines = CountLines(change.Body.After)
				slog.LogAttrs(context.Background(), slog.LevelDebug, "File was moved or renamed", slog.String("path", change.Path.After.Name))
			} else {
				slog.LogAttrs(context.Background(), slog.LevelDebug, "File was modified", slog.String("path", change.Path.After.Name), slog.Any("lines", change.Body.ModifiedLines))
			}
		case change.Path.Before.Type == Symlink && change.Path.After.Type == Symlink:
			slog.LogAttrs(context.Background(), slog.LevelDebug, "Symlink was modified", slog.String("path", change.Path.After.Name))
			// symlink was modified, every source line is modification
			change.Body.ModifiedLines = CountLines(change.Body.After)
		case change.Path.Before.Type == Missing && change.Path.After.Type != Missing:
			slog.LogAttrs(context.Background(), slog.LevelDebug, "File was added", slog.String("path", change.Path.After.Name))
			// old file body is empty, meaning that every line was modified
			change.Body.ModifiedLines = CountLines(change.Body.After)
		case change.Path.Before.Type != Missing && change.Path.After.Type == Missing:
			slog.LogAttrs(context.Background(), slog.LevelDebug, "File was removed", slog.String("path", change.Path.After.Name))
			// new file body is empty, meaning that every line was modified
			change.Body.ModifiedLines = CountLines(change.Body.Before)
		case change.Path.Before.Type == Missing && change.Path.After.Type == Missing:
			slog.LogAttrs(context.Background(), slog.LevelDebug, "File was added and removed", slog.String("path", change.Path.After.Name))
			// file was added and then removed
			change.Body.ModifiedLines = []int{}
		default:
			slog.LogAttrs(context.Background(), slog.LevelWarn, "Unhandled change", slog.String("change", fmt.Sprintf("+%v", change)))
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

func changesWithout(changes []*FileChange, fpath string) []*FileChange {
	return slices.DeleteFunc(changes, func(e *FileChange) bool {
		return e.Path.After.Name == fpath
	})
}

func getChangeByPath(changes []*FileChange, fpath string) *FileChange {
	for _, c := range changes {
		if c.Path.After.Name == fpath {
			return c
		}
	}
	return nil
}

func getModifiedLines(cmd CommandRunner, commits []string, fpath, atCommit string, bodyBefore, bodyAfter []byte) ([]int, error) {
	slog.LogAttrs(context.Background(), slog.LevelDebug, "Getting list of modified lines",
		slog.Any("commits", commits),
		slog.String("path", fpath),
	)
	lines, err := Blame(cmd, fpath, atCommit)
	if err != nil {
		return nil, err
	}

	linesBefore := bytes.Split(bodyBefore, []byte("\n"))
	linesAfter := bytes.Split(bodyAfter, []byte("\n"))
	slog.LogAttrs(context.Background(), slog.LevelDebug, "Number of lines", slog.Int("before", len(linesBefore)), slog.Int("after", len(linesAfter)))

	modLines := make([]int, 0, len(lines))
	for _, line := range lines {
		slog.LogAttrs(context.Background(), slog.LevelDebug, "Checking line", slog.String("commit", line.Commit), slog.Int("prev", line.PrevLine), slog.Int("line", line.Line))
		if !slices.Contains(commits, line.Commit) {
			continue
		}

		if line.PrevLine <= len(linesBefore) && line.Line <= len(linesAfter) {
			slog.LogAttrs(context.Background(), slog.LevelDebug, "Checking line content", slog.String("before", string(linesBefore[line.PrevLine-1])), slog.String("after", string(linesAfter[line.Line-1])))
			if bytes.Equal(linesBefore[line.PrevLine-1], linesAfter[line.Line-1]) {
				continue
			}
		}

		modLines = append(modLines, line.Line)
	}
	slog.LogAttrs(context.Background(), slog.LevelDebug, "List of modified lines",
		slog.Any("commits", commits),
		slog.String("path", fpath),
		slog.Any("lines", modLines),
	)
	return modLines, nil
}

func getTypeForPath(cmd CommandRunner, commit, fpath string) PathType {
	args := []string{"ls-tree", commit, fpath}
	out, err := cmd(args...)
	if err != nil {
		slog.LogAttrs(context.Background(), slog.LevelDebug, "git command returned an error", slog.Any("err", err), slog.String("args", fmt.Sprint(args)))
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

		parts = strings.SplitN(parts[2], "\t", 2)
		if len(parts) != 2 {
			continue
		}
		objpath := parts[1]
		slog.LogAttrs(context.Background(), slog.LevelDebug, "ls-tree line",
			slog.String("mode", objmode),
			slog.String("type", objtype),
			slog.String("path", objpath),
		)

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

// recursively find the final target of a symlink.
func resolveSymlinkTarget(cmd CommandRunner, commit, fpath string, pathType PathType) string {
	if pathType != Symlink {
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
		slog.LogAttrs(context.Background(), slog.LevelDebug, "git command returned an error", slog.Any("err", err), slog.String("args", fmt.Sprint(args)))
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
