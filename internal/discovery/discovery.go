package discovery

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"slices"
	"time"

	"github.com/cloudflare/pint/internal/comments"
	"github.com/cloudflare/pint/internal/parser"
)

const (
	FileOwnerComment         = "file/owner"
	FileDisabledCheckComment = "file/disable"
	FileSnoozeCheckComment   = "file/snooze"
	RuleOwnerComment         = "rule/owner"
)

type FileIgnoreError struct {
	Err  error
	Line int
}

func (fe FileIgnoreError) Error() string {
	return fe.Err.Error()
}

type ChangeType uint8

func (c ChangeType) String() string {
	switch c {
	case Unknown:
		return "unknown"
	case Noop:
		return "noop"
	case Added:
		return "added"
	case Modified:
		return "modified"
	case Removed:
		return "removed"
	case Moved:
		return "moved"
	default:
		return "---"
	}
}

func (c *ChangeType) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.String())
}

const (
	Unknown ChangeType = iota
	Noop
	Added
	Modified
	Removed
	Moved
)

type Path struct {
	Name          string // File path, it can be symlink.
	SymlinkTarget string // Symlink target, or the same as name if not a symlink.
}

func (p Path) String() string {
	if p.Name == p.SymlinkTarget {
		return p.Name
	}
	return fmt.Sprintf("%s ~> %s", p.Name, p.SymlinkTarget)
}

type Entry struct {
	PathError      error
	Path           Path
	Owner          string
	ModifiedLines  []int
	DisabledChecks []string
	Rule           parser.Rule
	State          ChangeType
}

func readRules(reportedPath, sourcePath string, r io.Reader, p parser.Parser, allowedOwners []*regexp.Regexp) (entries []Entry, err error) {
	content, err := parser.ReadContent(r)
	if err != nil {
		return nil, err
	}

	contentLines := parser.LineRange{
		First: min(content.TotalLines, 1),
		Last:  content.TotalLines,
	}

	var badOwners []comments.Comment
	var fileOwner string
	var disabledChecks []string
	for _, comment := range content.FileComments {
		// nolint:exhaustive
		switch comment.Type {
		case comments.FileOwnerType:
			owner := comment.Value.(comments.Owner)
			if isValidOwner(owner.Name, allowedOwners) {
				fileOwner = owner.Name
			} else {
				badOwners = append(badOwners, comment)
			}
		case comments.FileDisableType:
			disable := comment.Value.(comments.Disable)
			if !slices.Contains(disabledChecks, disable.Match) {
				disabledChecks = append(disabledChecks, disable.Match)
			}
		case comments.FileSnoozeType:
			snooze := comment.Value.(comments.Snooze)
			if !snooze.Until.After(time.Now()) {
				continue
			}
			if !slices.Contains(disabledChecks, snooze.Match) {
				disabledChecks = append(disabledChecks, snooze.Match)
			}
			slog.Debug(
				"Check snoozed by comment",
				slog.String("check", snooze.Match),
				slog.String("match", snooze.Match),
				slog.Time("until", snooze.Until),
			)
		case comments.InvalidComment:
			entries = append(entries, Entry{
				Path: Path{
					Name:          sourcePath,
					SymlinkTarget: reportedPath,
				},
				PathError:     comment.Value.(comments.Invalid).Err,
				Owner:         fileOwner,
				ModifiedLines: contentLines.Expand(),
			})
		}
	}

	if content.Ignored {
		entries = append(entries, Entry{
			Path: Path{
				Name:          sourcePath,
				SymlinkTarget: reportedPath,
			},
			PathError: FileIgnoreError{
				Line: content.IgnoreLine,
				// nolint:revive
				Err: errors.New("This file was excluded from pint checks."),
			},
			Owner:         fileOwner,
			ModifiedLines: contentLines.Expand(),
		})
		return entries, nil
	}

	rules, err := p.Parse(content.Body)
	if err != nil {
		slog.Warn(
			"Failed to parse file content",
			slog.Any("err", err),
			slog.String("path", sourcePath),
			slog.String("lines", contentLines.String()),
		)
		entries = append(entries, Entry{
			Path: Path{
				Name:          sourcePath,
				SymlinkTarget: reportedPath,
			},
			PathError:     err,
			Owner:         fileOwner,
			ModifiedLines: contentLines.Expand(),
		})
		return entries, nil
	}

	for _, rule := range rules {
		ruleOwner := fileOwner
		for _, owner := range comments.Only[comments.Owner](rule.Comments, comments.RuleOwnerType) {
			ruleOwner = owner.Name
		}
		entries = append(entries, Entry{
			Path: Path{
				Name:          sourcePath,
				SymlinkTarget: reportedPath,
			},
			Rule:           rule,
			ModifiedLines:  rule.Lines.Expand(),
			Owner:          ruleOwner,
			DisabledChecks: disabledChecks,
		})
	}

	if len(rules) == 0 && len(badOwners) > 0 {
		for _, comment := range badOwners {
			owner := comment.Value.(comments.Owner)
			entries = append(entries, Entry{
				Path: Path{
					Name:          sourcePath,
					SymlinkTarget: reportedPath,
				},
				PathError:     comments.OwnerError(owner),
				ModifiedLines: contentLines.Expand(),
			})
		}
	}

	slog.Debug("File parsed", slog.String("path", sourcePath), slog.Int("rules", len(entries)))
	return entries, nil
}

func isValidOwner(s string, valid []*regexp.Regexp) bool {
	if len(valid) == 0 {
		return true
	}
	for _, v := range valid {
		if v.MatchString(s) {
			return true
		}
	}
	return false
}
