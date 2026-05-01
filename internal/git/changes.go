package git

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path"
	"regexp"
	"slices"
	"strconv"
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

type LineNumber struct {
	Before int
	After  int
}

func (ln LineNumber) String() string {
	switch {
	case ln.Before == 0 && ln.After > 0:
		return fmt.Sprintf("+%d", ln.After)
	case ln.Before > 0 && ln.After == 0:
		return fmt.Sprintf("-%d", ln.Before)
	case ln.Before == ln.After:
		return strconv.Itoa(ln.Before)
	default:
		return fmt.Sprintf("%d->%d", ln.Before, ln.After)
	}
}

type LineNumbers []LineNumber

func (lns LineNumbers) String() string {
	parts := make([]string, len(lns))
	for i, ln := range lns {
		parts[i] = ln.String()
	}
	return strings.Join(parts, " ")
}

func (lns LineNumbers) HasAfter(line int) bool {
	for _, ln := range lns {
		if ln.After == line {
			return true
		}
	}
	return false
}

// BeforeForAfter returns the old (before the change) line number for a given
// new (after the change) line number.
//
// We only have line mappings for lines that were changed (added, removed, or
// modified). Unchanged lines may or may not appear in the diff output.
// Even if a line is unchanged, its old line number might differ from the
// new one because additions or removals earlier in the file shift all
// subsequent lines up or down.
//
// Returns:
//   - Before value if the line is directly present in the diff.
//   - 0 if the line is an added line (Before==0 in the diff).
//   - Computed old line number for lines not in the diff, using the offset
//     between old and new line numbers from the closest preceding diff entry.
//     For example if the last known entry is {Before:5, After:10} and we
//     query line 15, the offset is 10-5=5, so old line is 15-5=10.
//   - Line itself if there are no preceding diff entries (no shift yet).
func (lns LineNumbers) BeforeForAfter(line int) int {
	for _, ln := range lns {
		if ln.After == line {
			return ln.Before
		}
	}

	// Line is not directly in the diff -- find the closest preceding entry
	// that has a valid old line number (Before > 0, meaning it's not a
	// pure addition) and use its old->new offset to calculate old_line.
	var (
		nearest LineNumber
		found   bool
	)
	for _, ln := range lns {
		if ln.After > 0 && ln.After < line && ln.Before > 0 {
			nearest = ln
			found = true
		}
	}
	if found {
		return nearest.Before + (line - nearest.After)
	}

	// No preceding entry -- line is before any changes, old == new.
	return line
}

type LineRangeSide uint8

const (
	LinesBefore LineRangeSide = iota
	LinesAfter
	LinesBoth
)

type BodyDiff struct {
	Before []byte
	After  []byte
	Lines  LineNumbers
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
			slog.Any("lines", change.Body.Lines),
		)

		switch {
		case change.Path.Before.Type != Missing && change.Path.Before.Type != Symlink && change.Path.After.Type == Symlink:
			slog.LogAttrs(context.Background(), slog.LevelDebug, "Path was turned into a symlink", slog.String("path", change.Path.After.Name))
			change.Body.Lines = MakeLineRange(CountLines(change.Body.After), LinesAfter)
		case change.Path.Before.Type != Missing && change.Path.After.Type != Missing && change.Path.After.Type != Symlink:
			change.Body.Lines, err = getModifiedLines(cmd, change.Commits, change.Path.Before.EffectivePath(), change.Path.After.EffectivePath())
			if err != nil {
				return nil, fmt.Errorf("failed to run git diff for %s: %w", change.Path.After.EffectivePath(), err)
			}
			slog.LogAttrs(context.Background(), slog.LevelDebug, "File was modified", slog.String("path", change.Path.After.Name), slog.Any("lines", change.Body.Lines))
		case change.Path.Before.Type == Symlink && change.Path.After.Type == Symlink:
			slog.LogAttrs(context.Background(), slog.LevelDebug, "Symlink was modified", slog.String("path", change.Path.After.Name))
			// symlink was modified, every source line is modification
			change.Body.Lines = MakeLineRange(CountLines(change.Body.After), LinesAfter)
		case change.Path.Before.Type == Missing && change.Path.After.Type != Missing:
			slog.LogAttrs(context.Background(), slog.LevelDebug, "File was added", slog.String("path", change.Path.After.Name))
			// old file body is empty, meaning that every line was modified
			change.Body.Lines = MakeLineRange(CountLines(change.Body.After), LinesAfter)
		case change.Path.Before.Type != Missing && change.Path.After.Type == Missing:
			slog.LogAttrs(context.Background(), slog.LevelDebug, "File was removed", slog.String("path", change.Path.After.Name))
			// new file body is empty, meaning that every line was modified
			change.Body.Lines = MakeLineRange(CountLines(change.Body.Before), LinesBefore)
		case change.Path.Before.Type == Missing && change.Path.After.Type == Missing:
			slog.LogAttrs(context.Background(), slog.LevelDebug, "File was added and removed", slog.String("path", change.Path.After.Name))
			// file was added and then removed
			change.Body.Lines = []LineNumber{}
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

func getModifiedLines(cmd CommandRunner, commits []string, beforePath, afterPath string) (LineNumbers, error) {
	slog.LogAttrs(context.Background(), slog.LevelDebug, "Getting list of modified lines",
		slog.Any("commits", commits),
		slog.String("beforePath", beforePath),
		slog.String("afterPath", afterPath),
	)

	output, err := cmd("diff", "-M", commits[0]+"^.."+commits[len(commits)-1], "--", beforePath, afterPath)
	if err != nil {
		return nil, fmt.Errorf("git diff for %s: %w", afterPath, err)
	}

	lineNumbers := parseDiff(output, afterPath)

	slog.LogAttrs(context.Background(), slog.LevelDebug, "List of modified lines",
		slog.Any("commits", commits),
		slog.String("beforePath", beforePath),
		slog.String("afterPath", afterPath),
		slog.Any("lines", lineNumbers),
	)
	return lineNumbers, nil
}

var diffHunkRe = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

// parseDiff parses unified diff output from git and returns a list of line
// number mappings between old and new file versions.
//
// A unified diff contains one or more hunks, each starting with a header:
//
//	@@ -oldStart,oldCount +newStart,newCount @@
//
// Lines in the hunk body are prefixed with:
//   - " " (space) -- unchanged context line, present in both old and new file.
//   - "-" -- line removed from the old file.
//   - "+" -- line added in the new file.
//
// When a line is modified, git represents it as a deletion followed by an
// addition. We pair consecutive delete/add sequences to detect modifications:
//   - Paired delete+add -> modification: {Before: oldLine, After: newLine}.
//   - Unpaired delete   -> pure removal: {Before: oldLine, After: 0}.
//   - Unpaired add      -> pure addition: {Before: 0, After: newLine}.
//
// Context lines are not tracked because BeforeForAfter() can compute the
// old line number for any untracked line using the offset from the nearest
// tracked entry.
//
// When the diff contains multiple files (e.g. renames), only the block
// matching targetPath is processed.
func parseDiff(diff []byte, targetPath string) LineNumbers {
	var (
		oldLine, newLine int
		lineNumbers      = LineNumbers{}
		// Pending deleted lines waiting to be paired with additions.
		pendingDeletes []int
		// Only process hunks from the diff block matching targetPath.
		inTargetBlock bool
		// True once we've entered a hunk (after @@). Reset on "diff "
		// which starts a new file. Inside a hunk, all lines are content
		// -- we must not match diff/+++/--- headers.
		inHunk bool
	)

	// flushDeletes emits any pending deleted lines that were not paired
	// with a subsequent addition. Unpaired deletes are pure removals
	// (Before set, After=0). This is called when we hit a boundary that
	// ends a contiguous delete/add block: a new hunk, a context line,
	// or a new diff section.
	flushDeletes := func() {
		for _, d := range pendingDeletes {
			lineNumbers = append(lineNumbers, LineNumber{Before: d, After: 0})
		}
		pendingDeletes = pendingDeletes[:0]
	}

	sc := bufio.NewScanner(bytes.NewReader(diff))
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "diff "):
			flushDeletes()
			inTargetBlock = false
			inHunk = false
		case !inHunk && strings.HasPrefix(line, "+++ "):
			// +++ b/path or +++ /dev/null
			p := strings.TrimPrefix(line, "+++ b/")
			inTargetBlock = p == targetPath
		case !inTargetBlock:
			continue
		case strings.HasPrefix(line, "@@"):
			flushDeletes()
			inHunk = true
			// Parse hunk header: @@ -oldStart,oldCount +newStart,newCount @@
			// We decrement by 1 because line numbers in the hunk body are
			// incremented before use, so the first line processed will be
			// at the correct starting offset.
			matches := diffHunkRe.FindStringSubmatch(line)
			if len(matches) >= 4 {
				oldLine, _ = strconv.Atoi(matches[1])
				oldLine--
				newLine, _ = strconv.Atoi(matches[3])
				newLine--
			}
		case strings.HasPrefix(line, "-"):
			// Deleted line -- don't emit yet, queue it so it can be paired
			// with a subsequent addition to form a modification.
			oldLine++
			pendingDeletes = append(pendingDeletes, oldLine)
		case strings.HasPrefix(line, "+"):
			// Added line -- if there's a pending delete, pair them as a
			// modification (old line replaced by new line). Otherwise it's
			// a pure addition (Before=0).
			newLine++
			if len(pendingDeletes) > 0 {
				lineNumbers = append(lineNumbers, LineNumber{
					Before: pendingDeletes[0],
					After:  newLine,
				})
				pendingDeletes = pendingDeletes[1:]
			} else {
				lineNumbers = append(lineNumbers, LineNumber{Before: 0, After: newLine})
			}
		default:
			// Context (unchanged) line -- flush any unpaired deletes and
			// advance both counters. We don't emit context lines because
			// BeforeForAfter() can interpolate old line numbers for any
			// untracked line using the offset from the nearest entry.
			flushDeletes()
			oldLine++
			newLine++
		}
	}
	flushDeletes()

	return lineNumbers
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

func CountLines(body []byte) int {
	var count int
	s := bufio.NewScanner(bytes.NewReader(body))
	for s.Scan() {
		count++
	}
	return count
}

func MakeLineRange(n int, side LineRangeSide) LineNumbers {
	lineNumbers := make(LineNumbers, n)
	for i := range n {
		switch side {
		case LinesBefore:
			lineNumbers[i] = LineNumber{Before: i + 1, After: 0}
		case LinesAfter:
			lineNumbers[i] = LineNumber{Before: 0, After: i + 1}
		case LinesBoth:
			lineNumbers[i] = LineNumber{Before: i + 1, After: i + 1}
		}
	}
	return lineNumbers
}

func MakeLineRangeFromTo(first, last int, side LineRangeSide) LineNumbers {
	n := last - first + 1
	if n <= 0 {
		return LineNumbers{}
	}
	lineNumbers := make(LineNumbers, n)
	for i := range n {
		l := first + i
		switch side {
		case LinesBefore:
			lineNumbers[i] = LineNumber{Before: l, After: 0}
		case LinesAfter:
			lineNumbers[i] = LineNumber{Before: 0, After: l}
		case LinesBoth:
			lineNumbers[i] = LineNumber{Before: l, After: l}
		}
	}
	return lineNumbers
}

func isDirectoryPath(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	return fileInfo.IsDir(), err
}
