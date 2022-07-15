package discovery

import (
	"errors"
	"os"
	"regexp"
	"strings"

	"github.com/prometheus/prometheus/model/rulefmt"

	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/parser"

	"github.com/rs/zerolog/log"
)

const (
	FileOwnerComment = "file/owner"
	RuleOwnerComment = "rule/owner"
)

var ignoredErrors = []string{
	"one of 'record' or 'alert' must be set",
	"could not parse expression: ",
	"cannot unmarshal !!seq into rulefmt.ruleGroups",
}

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
	Path          string
	PathError     error
	ModifiedLines []int
	Rule          parser.Rule
	Owner         string
}

func readFile(path string, isStrict bool) (entries []Entry, err error) {
	p := parser.NewParser()

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	content, err := parser.ReadContent(f)
	f.Close()
	if err != nil {
		return nil, err
	}

	contentLines := []int{}
	for i := 1; i <= strings.Count(string(content), "\n"); i++ {
		contentLines = append(contentLines, i)
	}

	fileOwner, _ := parser.GetComment(string(content), FileOwnerComment)

	if isStrict {
		if _, errs := rulefmt.Parse(content); len(errs) > 0 {
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
					Path:          path,
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

	rules, err := p.Parse(content)
	if err != nil {
		log.Error().
			Err(err).
			Str("path", path).
			Str("lines", output.FormatLineRangeString(contentLines)).
			Msg("Failed to parse file content")
		entries = append(entries, Entry{
			Path:          path,
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
			Path:  path,
			Rule:  rule,
			Owner: owner.Value,
		})
	}

	log.Info().Str("path", path).Int("rules", len(entries)).Msg("File parsed")
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
