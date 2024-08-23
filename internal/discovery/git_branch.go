package discovery

import (
	"bytes"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"golang.org/x/exp/slices"

	"github.com/cloudflare/pint/internal/git"
	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/parser"
)

func NewGitBranchFinder(
	gitCmd git.CommandRunner,
	filter git.PathFilter,
	baseBranch string,
	maxCommits int,
) GitBranchFinder {
	return GitBranchFinder{
		gitCmd:     gitCmd,
		filter:     filter,
		baseBranch: baseBranch,
		maxCommits: maxCommits,
	}
}

type GitBranchFinder struct {
	gitCmd     git.CommandRunner
	baseBranch string
	filter     git.PathFilter
	maxCommits int
}

func (f GitBranchFinder) Find(allEntries []Entry) (entries []Entry, err error) {
	changes, err := git.Changes(f.gitCmd, f.baseBranch, f.filter)
	if err != nil {
		return nil, err
	}

	if totalCommits := countCommits(changes); totalCommits > f.maxCommits {
		return nil, fmt.Errorf("number of commits to check (%d) is higher than maxCommits (%d), exiting", totalCommits, f.maxCommits)
	}

	shouldSkip, err := f.shouldSkipAllChecks(changes)
	if err != nil {
		return nil, err
	}
	if shouldSkip {
		return nil, nil
	}

	for _, change := range changes {
		var entriesBefore, entriesAfter []Entry
		entriesBefore, err = readRules(
			change.Path.Before.EffectivePath(),
			change.Path.Before.Name,
			bytes.NewReader(change.Body.Before),
			!f.filter.IsRelaxed(change.Path.Before.Name),
		)
		if err != nil {
			slog.Debug("Cannot read before rules", slog.String("path", change.Path.Before.Name), slog.Any("err", err))
		}
		entriesAfter, err = readRules(
			change.Path.After.EffectivePath(),
			change.Path.After.Name,
			bytes.NewReader(change.Body.After),
			!f.filter.IsRelaxed(change.Path.After.Name),
		)
		if err != nil {
			return nil, fmt.Errorf("invalid file syntax: %w", err)
		}

		failedEntries := entriesWithPathErrors(entriesAfter)

		slog.Debug(
			"Parsing git file change",
			slog.Any("commits", change.Commits),
			slog.String("before.path", change.Path.Before.Name),
			slog.String("before.target", change.Path.Before.SymlinkTarget),
			slog.Any("before.type", change.Path.Before.Type),
			slog.Int("before.entries", len(entriesBefore)),
			slog.String("after.path", change.Path.After.Name),
			slog.String("after.target", change.Path.After.SymlinkTarget),
			slog.Any("after.type", change.Path.After.Type),
			slog.Int("after.entries", len(entriesAfter)),
			slog.Any("modifiedLines", change.Body.ModifiedLines),
		)
		for _, me := range matchEntries(entriesBefore, entriesAfter) {
			switch {
			case !me.hasBefore && me.hasAfter:
				me.after.State = Added
				me.after.ModifiedLines = commonLines(change.Body.ModifiedLines, me.after.ModifiedLines)
				slog.Debug(
					"Rule added on HEAD branch",
					slog.String("name", me.after.Rule.Name()),
					slog.String("state", me.after.State.String()),
					slog.String("path", me.after.Path.Name),
					slog.String("ruleLines", me.after.Rule.Lines.String()),
					slog.String("modifiedLines", output.FormatLineRangeString(me.after.ModifiedLines)),
				)
				entries = append(entries, me.after)
			case me.hasBefore && me.hasAfter:
				switch {
				case me.isIdentical && !me.wasMoved:
					me.after.State = Excluded
					me.after.ModifiedLines = []int{}
					slog.Debug(
						"Rule content was not modified on HEAD, identical rule present before",
						slog.String("name", me.after.Rule.Name()),
						slog.String("lines", me.after.Rule.Lines.String()),
					)
				case me.wasMoved:
					me.after.State = Moved
					me.after.ModifiedLines = git.CountLines(change.Body.After)
					slog.Debug(
						"Rule content was not modified on HEAD but the file was moved or renamed",
						slog.String("name", me.after.Rule.Name()),
						slog.String("lines", me.after.Rule.Lines.String()),
					)
				default:
					me.after.State = Modified
					me.after.ModifiedLines = commonLines(change.Body.ModifiedLines, me.after.ModifiedLines)
					slog.Debug(
						"Rule modified on HEAD branch",
						slog.String("name", me.after.Rule.Name()),
						slog.String("state", me.after.State.String()),
						slog.String("path", me.after.Path.Name),
						slog.String("ruleLines", me.after.Rule.Lines.String()),
						slog.String("modifiedLines", output.FormatLineRangeString(me.after.ModifiedLines)),
					)
				}
				entries = append(entries, me.after)
			case me.hasBefore && !me.hasAfter && len(failedEntries) == 0:
				me.before.State = Removed
				ml := commonLines(change.Body.ModifiedLines, me.before.ModifiedLines)
				if len(ml) > 0 {
					me.before.ModifiedLines = ml
				}
				slog.Debug(
					"Rule removed on HEAD branch",
					slog.String("name", me.before.Rule.Name()),
					slog.String("state", me.before.State.String()),
					slog.String("path", me.before.Path.Name),
					slog.String("ruleLines", me.before.Rule.Lines.String()),
					slog.String("modifiedLines", output.FormatLineRangeString(me.before.ModifiedLines)),
				)
				entries = append(entries, me.before)
			case me.hasBefore && !me.hasAfter && len(failedEntries) > 0:
				slog.Debug(
					"Rule not present on HEAD branch but there are parse errors",
					slog.String("name", me.before.Rule.Name()),
					slog.String("state", me.before.State.String()),
					slog.String("path", me.before.Path.Name),
					slog.String("ruleLines", me.before.Rule.Lines.String()),
					slog.String("modifiedLines", output.FormatLineRangeString(me.before.ModifiedLines)),
				)
			default:
				slog.Warn(
					"Unknown rule state",
					slog.String("state", me.before.State.String()),
					slog.String("path", me.before.Path.Name),
					slog.String("modifiedLines", output.FormatLineRangeString(me.before.ModifiedLines)),
				)
				entries = append(entries, me.after)
			}
		}
	}

	symlinks, err := addSymlinkedEntries(entries)
	if err != nil {
		return nil, err
	}

	for _, entry := range symlinks {
		if f.filter.IsPathAllowed(entry.Path.Name) {
			entries = append(entries, entry)
		}
	}

	var found bool
	for _, entry := range entries {
		found = false
		if entry.State == Removed {
			goto NEXT
		}
		for i, globEntry := range allEntries {
			if entry.Path.Name == globEntry.Path.Name && entry.Rule.IsSame(globEntry.Rule) {
				allEntries[i].State = entry.State
				allEntries[i].ModifiedLines = entry.ModifiedLines
				found = true
				break
			}
		}
	NEXT:
		if !found {
			allEntries = append(allEntries, entry)
		}
	}

	slog.Debug("Git branch finder completed", slog.Int("count", len(allEntries)))
	return allEntries, nil
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
				slog.Info(
					fmt.Sprintf("Found a commit with '%s', skipping all checks", comment),
					slog.String("commit", commit))
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
	before      Entry
	after       Entry
	hasBefore   bool
	hasAfter    bool
	isIdentical bool
	wasMoved    bool
}

func matchEntries(before, after []Entry) (ml []matchedEntry) {
	for _, a := range after {
		slog.Debug(
			"Matching HEAD rule",
			slog.String("path", a.Path.Name),
			slog.String("source", a.Path.SymlinkTarget),
			slog.String("name", a.Rule.Name()),
		)

		m := matchedEntry{after: a, hasAfter: true}
		beforeSwap := make([]Entry, 0, len(before))
		var matches []Entry
		var matched bool

		for _, b := range before {
			if !matched && a.Rule.Name() != "" && a.Rule.IsIdentical(b.Rule) {
				m.before = b
				m.hasBefore = true
				m.isIdentical = isEntryIdentical(b, a)
				m.wasMoved = a.Path.Name != b.Path.Name
				matched = true
				slog.Debug(
					"Found identical rule on before & after",
					slog.Bool("identical", m.isIdentical),
					slog.Bool("moved", m.wasMoved),
				)
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
				m.before = matches[0]
				m.hasBefore = true
				m.wasMoved = a.Path.Name != matches[0].Path.Name
				slog.Debug("Found rule with same name on before & after")
			default:
				slog.Debug(
					"Found multiple rules with same name on before & after",
					slog.Int("matches", len(matches)),
				)
				before = append(before, matches...)
			}
		}

		ml = append(ml, m)
	}

	for _, b := range before {
		ml = append(ml, matchedEntry{before: b, hasBefore: true})
	}

	return ml
}

func isEntryIdentical(b, a Entry) bool {
	if !slices.Equal(sort.StringSlice(b.DisabledChecks), sort.StringSlice(a.DisabledChecks)) {
		slog.Debug("List of disabled checks was modified",
			slog.Any("before", sort.StringSlice(b.DisabledChecks)),
			slog.Any("after", sort.StringSlice(a.DisabledChecks)))
		return false
	}
	return true
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

func countCommits(changes []*git.FileChange) int {
	commits := map[string]struct{}{}
	for _, change := range changes {
		for _, commit := range change.Commits {
			commits[commit] = struct{}{}
		}
	}
	return len(commits)
}

func entriesWithPathErrors(entries []Entry) (match []Entry) {
	for _, entry := range entries {
		if entry.PathError != nil {
			match = append(match, entry)
		}
	}
	return match
}
