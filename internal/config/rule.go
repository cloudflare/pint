package config

import (
	"context"
	"log/slog"
	"regexp"
	"slices"
	"time"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/comments"
	"github.com/cloudflare/pint/internal/config/options"
	"github.com/cloudflare/pint/internal/parser"
)

type Rule struct {
	Match         []Match                    `hcl:"match,block" json:"match,omitempty"`
	Ignore        []Match                    `hcl:"ignore,block" json:"ignore,omitempty"`
	Enable        []string                   `hcl:"enable,optional" json:"enable,omitempty"`
	Disable       []string                   `hcl:"disable,optional" json:"disable,omitempty"`
	Aggregate     []AggregateSettings        `hcl:"aggregate,block" json:"aggregate,omitempty"`
	Annotation    []AnnotationSettings       `hcl:"annotation,block" json:"annotation,omitempty"`
	Label         []AnnotationSettings       `hcl:"label,block" json:"label,omitempty"`
	Cost          *CostSettings              `hcl:"cost,block" json:"cost,omitempty"`
	Alerts        *AlertsSettings            `hcl:"alerts,block" json:"alerts,omitempty"`
	For           *ForSettings               `hcl:"for,block" json:"for,omitempty"`
	KeepFiringFor *ForSettings               `hcl:"keep_firing_for,block" json:"keep_firing_for,omitempty"`
	RangeQuery    *RangeQuerySettings        `hcl:"range_query,block" json:"range_query,omitempty"`
	Report        *ReportSettings            `hcl:"report,block" json:"report,omitempty"`
	Reject        []RejectSettings           `hcl:"reject,block" json:"reject,omitempty"`
	RuleLink      []RuleLinkSettings         `hcl:"link,block" json:"link,omitempty"`
	RuleName      []RuleNameSettings         `hcl:"name,block" json:"name,omitempty"`
	Selector      []options.SelectorSettings `hcl:"selector,block" json:"selector,omitempty"`
	Call          []options.CallSettings     `hcl:"call,block" json:"call,omitempty"`
	Locked        bool                       `hcl:"locked,optional" json:"locked,omitempty"`
}

func (rule Rule) validate() (err error) {
	for _, match := range rule.Match {
		if err = match.validate(true); err != nil {
			return err
		}
	}

	for _, ignore := range rule.Ignore {
		if err = ignore.validate(false); err != nil {
			return err
		}
	}

	for _, name := range rule.Enable {
		if err = validateCheckName(name); err != nil {
			return err
		}
	}

	for _, name := range rule.Disable {
		if err = validateCheckName(name); err != nil {
			return err
		}
	}

	for _, aggr := range rule.Aggregate {
		if err = aggr.validate(); err != nil {
			return err
		}
	}

	for _, ann := range rule.Annotation {
		if err = ann.validate(); err != nil {
			return err
		}
	}

	for _, lab := range rule.Label {
		if err = lab.validate(); err != nil {
			return err
		}
	}

	if rule.Cost != nil {
		if err = rule.Cost.validate(); err != nil {
			return err
		}
	}

	if rule.Alerts != nil {
		if err = rule.Alerts.validate(); err != nil {
			return err
		}
	}

	for _, reject := range rule.Reject {
		if err = reject.validate(); err != nil {
			return err
		}
	}

	for _, link := range rule.RuleLink {
		if err = link.validate(); err != nil {
			return err
		}
	}

	if rule.For != nil {
		if err = rule.For.validate(); err != nil {
			return err
		}
	}

	if rule.KeepFiringFor != nil {
		if err = rule.KeepFiringFor.validate(); err != nil {
			return err
		}
	}

	for _, name := range rule.RuleName {
		if err = name.validate(); err != nil {
			return err
		}
	}

	if rule.RangeQuery != nil {
		if err = rule.RangeQuery.validate(); err != nil {
			return err
		}
	}

	if rule.Report != nil {
		if err = rule.Report.validate(); err != nil {
			return err
		}
	}

	for _, selector := range rule.Selector {
		if err = selector.Validate(); err != nil {
			return err
		}
	}

	for _, call := range rule.Call {
		if err = call.Validate(); err != nil {
			return err
		}
	}

	return nil
}

func isDisabledForRule(rule parser.Rule, name string, check checks.RuleChecker, promTags []string) bool {
	matches := []string{
		name,
		check.String(),
	}
	for _, tag := range promTags {
		matches = append(matches, name+"(+"+tag+")")
	}
	for _, disable := range comments.Only[comments.Disable](rule.Comments, comments.DisableType) {
		for _, match := range matches {
			if match == disable.Match {
				slog.LogAttrs(context.Background(), slog.LevelDebug,
					"Check disabled by comment",
					slog.String("check", check.String()),
					slog.String("match", match),
				)
				return true
			}
		}
	}
	for _, snooze := range comments.Only[comments.Snooze](rule.Comments, comments.SnoozeType) {
		if !snooze.Until.After(time.Now()) {
			continue
		}
		if slices.Contains(matches, snooze.Match) {
			slog.LogAttrs(context.Background(), slog.LevelDebug,
				"Check snoozed by comment",
				slog.String("check", check.String()),
				slog.String("match", snooze.Match),
				slog.Time("until", snooze.Until),
			)
			return true
		}
	}
	return false
}

func isEnabled(enabledChecks, disabledChecks []string, rule parser.Rule, name string, check checks.RuleChecker, promTags []string, locked bool) bool {
	if check.Meta().AlwaysEnabled {
		return true
	}

	if !locked && isDisabledForRule(rule, name, check, promTags) {
		return false
	}

	for _, c := range disabledChecks {
		if c == name || c == check.String() {
			return false
		}
		for _, tag := range promTags {
			if c == name+"(+"+tag+")" {
				return false
			}
		}
	}
	if len(enabledChecks) == 0 {
		return true
	}
	return slices.Contains(enabledChecks, name)
}

func strictRegex(s string) *regexp.Regexp {
	return regexp.MustCompile("^" + s + "$")
}

func MustCompileRegexes(l ...string) (r []*regexp.Regexp) {
	for _, pattern := range l {
		r = append(r, strictRegex(pattern))
	}
	return r
}
