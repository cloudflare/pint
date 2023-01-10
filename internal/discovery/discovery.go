package discovery

import (
	"errors"
	"os"
	"regexp"
	"strings"

	"github.com/prometheus/prometheus/model/rulefmt"
	"golang.org/x/exp/slices"

	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/parser"

	"github.com/rs/zerolog/log"
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

	werr := &rulefmt.WrappedError{}
	if errors.As(err, &werr) {
		if uerr := werr.Unwrap(); uerr != nil {
			s = uerr.Error()
		}
	}
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

type Entry struct {
	ReportedPath   string
	SourcePath     string
	PathError      error
	ModifiedLines  []int
	Rule           parser.Rule
	Owner          string
	DisabledChecks []string
}

func readFile(path string, isStrict bool) (entries []Entry, err error) {
	p := parser.NewParser()

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	content, err := parser.ReadContent(f)
	_ = f.Close()
	if err != nil {
		return nil, err
	}

	contentLines := []int{}
	for i := 1; i <= strings.Count(string(content.Body), "\n"); i++ {
		contentLines = append(contentLines, i)
	}

	body := string(content.Body)
	fileOwner, _ := parser.GetLastComment(body, FileOwnerComment)

	var disabledChecks []string
	for _, comment := range parser.GetComments(body, FileDisabledCheckComment) {
		if !slices.Contains(disabledChecks, comment.Value) {
			disabledChecks = append(disabledChecks, comment.Value)
		}
	}
	for _, comment := range parser.GetComments(body, FileSnoozeCheckComment) {
		s := parser.ParseSnooze(comment.Value)
		if s == nil {
			continue
		}
		if !slices.Contains(disabledChecks, s.Text) {
			disabledChecks = append(disabledChecks, s.Text)
		}
		log.Debug().
			Str("check", s.Text).
			Str("comment", comment.String()).
			Time("until", s.Until).
			Str("snooze", s.Text).
			Msg("Check snoozed by comment")
	}

	if content.Ignored {
		entries = append(entries, Entry{
			ReportedPath:  path,
			SourcePath:    path,
			PathError:     ErrFileIsIgnored,
			Owner:         fileOwner.Value,
			ModifiedLines: contentLines,
		})
		return entries, nil
	}

	if isStrict {
		if _, errs := rulefmt.Parse(content.Body); len(errs) > 0 {
			for _, err := range errs {
				if isStrictIgnored(err) {
					continue
				}
				log.Error().
					Err(err).
					Str("path", path).
					Str("lines", output.FormatLineRangeString(contentLines)).
					Msg("Failed to unmarshal file content")
				entries = append(entries, Entry{
					ReportedPath:  path,
					SourcePath:    path,
					PathError:     err,
					Owner:         fileOwner.Value,
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
		log.Error().
			Err(err).
			Str("path", path).
			Str("lines", output.FormatLineRangeString(contentLines)).
			Msg("Failed to parse file content")
		entries = append(entries, Entry{
			ReportedPath:  path,
			SourcePath:    path,
			PathError:     err,
			Owner:         fileOwner.Value,
			ModifiedLines: contentLines,
		})
		return entries, nil
	}

	for _, rule := range rules {
		owner, ok := rule.GetComment(RuleOwnerComment)
		if !ok {
			owner = fileOwner
		}
		entries = append(entries, Entry{
			ReportedPath:   path,
			SourcePath:     path,
			Rule:           rule,
			Owner:          owner.Value,
			DisabledChecks: disabledChecks,
		})
	}

	log.Debug().Str("path", path).Int("rules", len(entries)).Msg("File parsed")
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
