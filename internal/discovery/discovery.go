package discovery

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/prometheus/prometheus/model/rulefmt"
	"golang.org/x/exp/slices"

	"github.com/cloudflare/pint/internal/comments"
	"github.com/cloudflare/pint/internal/output"
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
}

var ErrFileIsIgnored = errors.New("file was ignored")

func isStrictIgnored(err error) bool {
	s := err.Error()
	for _, ign := range ignoredErrors {
		if strings.Contains(s, ign) {
			return true
		}
	}
	return false
}

type RuleFinder interface {
	Find() ([]Entry, error)
}

type ChangeType int

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

type Entry struct {
	State          ChangeType
	ReportedPath   string // symlink target
	SourcePath     string // file path (can be symlink)
	PathError      error
	ModifiedLines  []int
	Rule           parser.Rule
	Owner          string
	DisabledChecks []string
}

func readRules(reportedPath, sourcePath string, r io.Reader, isStrict bool) (entries []Entry, err error) {
	p := parser.NewParser()

	content, fileComments, err := parser.ReadContent(r)
	if err != nil {
		return nil, err
	}

	contentLines := []int{}
	for i := 1; i <= strings.Count(string(content.Body), "\n"); i++ {
		contentLines = append(contentLines, i)
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
		}
	}

	if content.Ignored {
		entries = append(entries, Entry{
			ReportedPath:  reportedPath,
			SourcePath:    sourcePath,
			PathError:     ErrFileIsIgnored,
			Owner:         fileOwner,
			ModifiedLines: contentLines,
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
					ModifiedLines: contentLines,
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
			slog.String("lines", output.FormatLineRangeString(contentLines)),
		)
		entries = append(entries, Entry{
			ReportedPath:  reportedPath,
			SourcePath:    sourcePath,
			PathError:     err,
			Owner:         fileOwner,
			ModifiedLines: contentLines,
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
			ModifiedLines:  rule.Lines(),
			Owner:          ruleOwner,
			DisabledChecks: disabledChecks,
		})
	}

	slog.Debug("File parsed", slog.String("path", sourcePath), slog.Int("rules", len(entries)))
	return entries, nil
}

func matchesAny(re []*regexp.Regexp, s string) bool {
	for _, r := range re {
		if v := r.MatchString(s); v {
			return true
		}
	}
	return false
}
