package discovery

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/prometheus/prometheus/model/rulefmt"
	"golang.org/x/exp/slices"

	"github.com/cloudflare/pint/internal/comments"
	"github.com/cloudflare/pint/internal/parser"
)

const (
	FileOwnerComment         = "file/owner"
	FileDisabledCheckComment = "file/disable"
	FileSnoozeCheckComment   = "file/snooze"
	RuleOwnerComment         = "rule/owner"
)

var ignoredErrors = []string{
	"one of 'record' or 'alert' must be set",
	"field 'expr' must be set in rule",
	"could not parse expression: ",
	"cannot unmarshal !!seq into rulefmt.ruleGroups",
	": template: __",
	"invalid label name: ",
	"invalid annotation name: ",
	"invalid recording rule name: ",
}

type FileIgnoreError struct {
	Err  error
	Line int
}

func (fe FileIgnoreError) Error() string {
	return fe.Err.Error()
}

func isStrictIgnored(err error) bool {
	s := err.Error()
	for _, ign := range ignoredErrors {
		if strings.Contains(s, ign) {
			return true
		}
	}
	return false
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
	case Excluded:
		return "excluded"
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
	Excluded
)

type Entry struct {
	PathError      error
	ReportedPath   string // symlink target
	SourcePath     string // file path (can be symlink)
	Owner          string
	ModifiedLines  []int
	DisabledChecks []string
	Rule           parser.Rule
	State          ChangeType
}

func readRules(reportedPath, sourcePath string, r io.Reader, isStrict bool) (entries []Entry, err error) {
	p := parser.NewParser()

	content, fileComments, err := parser.ReadContent(r)
	if err != nil {
		return nil, err
	}

	contentLines := parser.LineRange{
		First: min(content.TotalLines, 1),
		Last:  content.TotalLines,
	}

	var fileOwner string
	var disabledChecks []string
	for _, comment := range fileComments {
		// nolint:exhaustive
		switch comment.Type {
		case comments.FileOwnerType:
			owner := comment.Value.(comments.Owner)
			fileOwner = owner.Name
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
				ReportedPath:  reportedPath,
				SourcePath:    sourcePath,
				PathError:     comment.Value.(comments.Invalid).Err,
				Owner:         fileOwner,
				ModifiedLines: contentLines.Expand(),
			})
		}
	}

	if content.Ignored {
		entries = append(entries, Entry{
			ReportedPath: reportedPath,
			SourcePath:   sourcePath,
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

	if isStrict {
		if _, errs := rulefmt.Parse(content.Body); len(errs) > 0 {
			seen := map[string]struct{}{}
			for _, err := range errs {
				if isStrictIgnored(err) {
					continue
				}
				if _, ok := seen[err.Error()]; ok {
					continue
				}
				seen[err.Error()] = struct{}{}
				entries = append(entries, Entry{
					ReportedPath:  reportedPath,
					SourcePath:    sourcePath,
					PathError:     err,
					Owner:         fileOwner,
					ModifiedLines: contentLines.Expand(),
				})
			}
			if len(entries) > 0 {
				return entries, nil
			}
		}
	}

	rules, err := p.Parse(content.Body)
	if err != nil {
		slog.Error(
			"Failed to parse file content",
			slog.Any("err", err),
			slog.String("path", sourcePath),
			slog.String("lines", contentLines.String()),
		)
		entries = append(entries, Entry{
			ReportedPath:  reportedPath,
			SourcePath:    sourcePath,
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
			ReportedPath:   reportedPath,
			SourcePath:     sourcePath,
			Rule:           rule,
			ModifiedLines:  rule.Lines.Expand(),
			Owner:          ruleOwner,
			DisabledChecks: disabledChecks,
		})
	}

	slog.Debug("File parsed", slog.String("path", sourcePath), slog.Int("rules", len(entries)))
	return entries, nil
}
