package discovery

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
	"golang.org/x/exp/slices"

	"github.com/cloudflare/pint/internal/git"
	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/parser"
)

func NewGitBranchFinder(
	gitCmd git.CommandRunner,
	include []*regexp.Regexp,
	exclude []*regexp.Regexp,
	baseBranch string,
	maxCommits int,
	relaxed []*regexp.Regexp,
) GitBranchFinder {
	return GitBranchFinder{
		gitCmd:     gitCmd,
		include:    include,
		exclude:    exclude,
		baseBranch: baseBranch,
		maxCommits: maxCommits,
		relaxed:    relaxed,
	}
}

type GitBranchFinder struct {
	gitCmd     git.CommandRunner
	include    []*regexp.Regexp
	exclude    []*regexp.Regexp
	baseBranch string
	maxCommits int
	relaxed    []*regexp.Regexp
}

func (f GitBranchFinder) Find() (entries []Entry, err error) {
	cr, err := git.CommitRange(f.gitCmd, f.baseBranch)
	if err != nil {
		return nil, fmt.Errorf("failed to get the list of commits to scan: %w", err)
	}
	log.Debug().Str("from", cr.From).Str("to", cr.To).Msg("Got commit range from git")

	if len(cr.Commits) > f.maxCommits {
		return nil, fmt.Errorf("number of commits to check (%d) is higher than maxCommits (%d), exiting", len(cr.Commits), f.maxCommits)
	}

	changes, err := git.Changes(f.gitCmd, cr)
	if err != nil {
		return nil, err
	}

	shouldSkip, err := f.shouldSkipAllChecks(changes)
	if err != nil {
		return nil, err
	}
	if shouldSkip {
		return nil, nil
	}

	for _, change := range changes {
		if !f.isPathAllowed(change.Path.After.Name) {
			log.Debug().Str("path", change.Path.After.Name).Msg("Skipping file due to include/exclude rules")
			continue
		}

		var entriesBefore, entriesAfter []Entry
		entriesBefore, _ = readRules(
			change.Path.Before.EffectivePath(),
			change.Path.Before.Name,
			bytes.NewReader(change.Body.Before),
			!matchesAny(f.relaxed, change.Path.Before.Name),
		)
		entriesAfter, err = readRules(
			change.Path.After.EffectivePath(),
			change.Path.After.Name,
			bytes.NewReader(change.Body.After),
			!matchesAny(f.relaxed, change.Path.After.Name),
		)
		if err != nil {
			return nil, fmt.Errorf("invalid file syntax: %w", err)
		}

		for _, me := range matchEntries(entriesBefore, entriesAfter) {
			switch {
			case me.before == nil && me.after != nil:
				me.after.State = Added
				me.after.ModifiedLines = commonLines(change.Body.ModifiedLines, me.after.ModifiedLines)
				log.Debug().
					Str("name", me.after.Rule.Name()).
					Stringer("state", me.after.State).
					Str("path", me.after.SourcePath).
					Str("ruleLines", output.FormatLineRangeString(me.after.Rule.Lines())).
					Str("modifiedLines", output.FormatLineRangeString(me.after.ModifiedLines)).
					Msg("Rule added on HEAD branch")
				entries = append(entries, *me.after)
			case me.before != nil && me.after != nil:
				if me.isIdentical {
					log.Debug().
						Str("name", me.after.Rule.Name()).
						Str("lines", output.FormatLineRangeString(me.after.Rule.Lines())).
						Msg("Rule content was not modified on HEAD, identical rule present before")
					continue
				}
				me.after.State = Modified
				me.after.ModifiedLines = commonLines(change.Body.ModifiedLines, me.after.ModifiedLines)
				log.Debug().
					Str("name", me.after.Rule.Name()).
					Stringer("state", me.after.State).
					Str("path", me.after.SourcePath).
					Str("ruleLines", output.FormatLineRangeString(me.after.Rule.Lines())).
					Str("modifiedLines", output.FormatLineRangeString(me.after.ModifiedLines)).
					Msg("Rule modified on HEAD branch")
				entries = append(entries, *me.after)
			case me.before != nil && me.after == nil:
				me.before.State = Removed
				log.Debug().
					Str("name", me.before.Rule.Name()).
					Stringer("state", me.before.State).
					Str("path", me.before.SourcePath).
					Str("ruleLines", output.FormatLineRangeString(me.before.Rule.Lines())).
					Str("modifiedLines", output.FormatLineRangeString(me.before.ModifiedLines)).
					Msg("Rule removed on HEAD branch")
				entries = append(entries, *me.before)
			default:
				log.Debug().
					Stringer("state", me.before.State).
					Str("path", me.before.SourcePath).
					Str("modifiedLines", output.FormatLineRangeString(me.before.ModifiedLines)).
					Msg("Unknown rule")
				entries = append(entries, *me.after)
			}
		}
	}

	symlinks, err := addSymlinkedEntries(entries)
	if err != nil {
		return nil, err
	}

	for _, entry := range symlinks {
		if f.isPathAllowed(entry.SourcePath) {
			entries = append(entries, entry)
		}
	}

	return entries, nil
}

func (f GitBranchFinder) isPathAllowed(path string) bool {
	if len(f.include) == 0 && len(f.exclude) == 0 {
		return true
	}

	for _, pattern := range f.exclude {
		if pattern.MatchString(path) {
			return false
		}
	}

	for _, pattern := range f.include {
		if pattern.MatchString(path) {
			return true
		}
	}

	return false
}

func (f GitBranchFinder) shouldSkipAllChecks(changes []*git.FileChange) (bool, error) {
	commits := map[string]struct{}{}
	for _, change := range changes {
		for _, commit := range change.Commits {
			commits[commit] = struct{}{}
		}
	}

	for commit := range commits {
		msg, err := git.CommitMessage(f.gitCmd, commit)
		if err != nil {
			return false, fmt.Errorf("failed to get commit message for %s: %w", commit, err)
		}
		for _, comment := range []string{"[skip ci]", "[no ci]"} {
			if strings.Contains(msg, comment) {
				log.Info().Str("commit", commit).Msgf("Found a commit with '%s', skipping all checks", comment)
				return true, nil
			}
		}
	}

	return false, nil
}

func commonLines(a, b []int) (common []int) {
	for _, ai := range a {
		if slices.Contains(b, ai) {
			common = append(common, ai)
		}
	}
	for _, bi := range b {
		if slices.Contains(a, bi) && !slices.Contains(common, bi) {
			common = append(common, bi)
		}
	}
	return common
}

type matchedEntry struct {
	before      *Entry
	after       *Entry
	isIdentical bool
}

func matchEntries(before, after []Entry) (ml []matchedEntry) {
	for _, a := range after {
		a := a

		m := matchedEntry{after: &a}
		beforeSwap := make([]Entry, 0, len(before))
		var matches []Entry
		var matched bool

		for _, b := range before {
			b := b

			if !matched && a.Rule.Name() != "" && a.Rule.ToYAML() == b.Rule.ToYAML() {
				m.before = &b
				m.isIdentical = true
				matched = true
			} else {
				beforeSwap = append(beforeSwap, b)
			}
		}
		before = beforeSwap

		if !matched {
			before, matches = findRulesByName(before, a.Rule.Name(), a.Rule.Type())
			switch len(matches) {
			case 0:
			case 1:
				m.before = &matches[0]
			default:
				before = append(before, matches...)
			}
		}

		ml = append(ml, m)
	}

	for _, b := range before {
		b := b
		ml = append(ml, matchedEntry{before: &b})
	}

	return ml
}

func findRulesByName(entries []Entry, name string, typ parser.RuleType) (nomatch, match []Entry) {
	for _, entry := range entries {
		if entry.PathError == nil && entry.Rule.Type() == typ && entry.Rule.Name() == name {
			match = append(match, entry)
		} else {
			nomatch = append(nomatch, entry)
		}
	}
	return nomatch, match
}
