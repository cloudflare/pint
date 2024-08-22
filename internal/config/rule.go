package config

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"time"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/comments"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
)

type Rule struct {
	Match         []Match              `hcl:"match,block" json:"match,omitempty"`
	Ignore        []Match              `hcl:"ignore,block" json:"ignore,omitempty"`
	Aggregate     []AggregateSettings  `hcl:"aggregate,block" json:"aggregate,omitempty"`
	Annotation    []AnnotationSettings `hcl:"annotation,block" json:"annotation,omitempty"`
	Label         []AnnotationSettings `hcl:"label,block" json:"label,omitempty"`
	Cost          *CostSettings        `hcl:"cost,block" json:"cost,omitempty"`
	Alerts        *AlertsSettings      `hcl:"alerts,block" json:"alerts,omitempty"`
	For           *ForSettings         `hcl:"for,block" json:"for,omitempty"`
	KeepFiringFor *ForSettings         `hcl:"keep_firing_for,block" json:"keep_firing_for,omitempty"`
	RangeQuery    *RangeQuerySettings  `hcl:"range_query,block" json:"range_query,omitempty"`
	Reject        []RejectSettings     `hcl:"reject,block" json:"reject,omitempty"`
	RuleLink      []RuleLinkSettings   `hcl:"link,block" json:"link,omitempty"`
	RuleName      []RuleNameSettings   `hcl:"name,block" json:"name,omitempty"`
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

	return nil
}

func (rule Rule) resolveChecks(ctx context.Context, e discovery.Entry, prometheusServers []*promapi.FailoverGroup) []checkMeta {
	cmd := ctx.Value(CommandKey).(ContextCommandVal)

	enabled := []checkMeta{}

	for _, ignore := range rule.Ignore {
		if ignore.IsMatch(ctx, e.Path.Name, e) {
			return enabled
		}
	}

	if len(rule.Match) > 0 {
		var found bool
		for _, match := range rule.Match {
			switch {
			case len(match.State) != 0:
				// use value from config
			case cmd == CICommand:
				match.State = CIStates
			default:
				match.State = AnyStates
			}
			if match.IsMatch(ctx, e.Path.Name, e) {
				found = true
				break
			}
		}
		if !found {
			return enabled
		}
	} else {
		// If there are no match blocks then we would allow rule in any state to pass through.
		// But we need to exclude some states depending on commands we run.
		var match Match
		switch cmd {
		case CICommand:
			match.State = CIStates
		default:
			match.State = AnyStates
		}
		if !match.IsMatch(ctx, e.Path.Name, e) {
			return enabled
		}
	}

	if len(rule.Aggregate) > 0 {
		var nameRegex *checks.TemplatedRegexp
		for _, aggr := range rule.Aggregate {
			if aggr.Name != "" {
				nameRegex = checks.MustTemplatedRegexp(aggr.Name)
			}
			severity := aggr.getSeverity(checks.Warning)
			for _, label := range aggr.Keep {
				enabled = append(enabled, checkMeta{
					name:  checks.AggregationCheckName,
					check: checks.NewAggregationCheck(nameRegex, label, true, aggr.Comment, severity),
				})
			}
			for _, label := range aggr.Strip {
				enabled = append(enabled, checkMeta{
					name:  checks.AggregationCheckName,
					check: checks.NewAggregationCheck(nameRegex, label, false, aggr.Comment, severity),
				})
			}
		}
	}

	if rule.Cost != nil {
		severity := rule.Cost.getSeverity(checks.Bug)
		evalDur, _ := parseDuration(rule.Cost.MaxEvaluationDuration)
		for _, prom := range prometheusServers {
			enabled = append(enabled, checkMeta{
				name:  checks.CostCheckName,
				check: checks.NewCostCheck(prom, rule.Cost.MaxSeries, rule.Cost.MaxTotalSamples, rule.Cost.MaxPeakSamples, evalDur, rule.Cost.Comment, severity),
				tags:  prom.Tags(),
			})
		}
	}

	if len(rule.Annotation) > 0 {
		for _, ann := range rule.Annotation {
			var tokenRegex, valueRegex *checks.TemplatedRegexp
			if ann.Token != "" {
				tokenRegex = checks.MustRawTemplatedRegexp(ann.Token)
			}
			if ann.Value != "" {
				valueRegex = checks.MustTemplatedRegexp(ann.Value)
			}
			severity := ann.getSeverity(checks.Warning)
			enabled = append(enabled, checkMeta{
				name:  checks.AnnotationCheckName,
				check: checks.NewAnnotationCheck(checks.MustTemplatedRegexp(ann.Key), tokenRegex, valueRegex, ann.Values, ann.Required, ann.Comment, severity),
			})
		}
	}

	if len(rule.Label) > 0 {
		for _, lab := range rule.Label {
			var tokenRegex, valueRegex *checks.TemplatedRegexp
			if lab.Token != "" {
				tokenRegex = checks.MustRawTemplatedRegexp(lab.Token)
			}
			if lab.Value != "" {
				valueRegex = checks.MustTemplatedRegexp(lab.Value)
			}
			severity := lab.getSeverity(checks.Warning)
			enabled = append(enabled, checkMeta{
				name:  checks.LabelCheckName,
				check: checks.NewLabelCheck(checks.MustTemplatedRegexp(lab.Key), tokenRegex, valueRegex, lab.Values, lab.Required, lab.Comment, severity),
			})
		}
	}

	if rule.Alerts != nil {
		qRange := time.Hour * 24
		if rule.Alerts.Range != "" {
			qRange, _ = parseDuration(rule.Alerts.Range)
		}
		qStep := time.Minute
		if rule.Alerts.Step != "" {
			qStep, _ = parseDuration(rule.Alerts.Step)
		}
		qResolve := time.Minute * 5
		if rule.Alerts.Resolve != "" {
			qResolve, _ = parseDuration(rule.Alerts.Resolve)
		}
		severity := rule.Alerts.getSeverity(checks.Information)
		for _, prom := range prometheusServers {
			enabled = append(enabled, checkMeta{
				name:  checks.AlertsCheckName,
				check: checks.NewAlertsCheck(prom, qRange, qStep, qResolve, rule.Alerts.MinCount, rule.Alerts.Comment, severity),
				tags:  prom.Tags(),
			})
		}
	}

	if len(rule.Reject) > 0 {
		for _, reject := range rule.Reject {
			severity := reject.getSeverity(checks.Bug)
			re := checks.MustTemplatedRegexp(reject.Regex)
			if reject.LabelKeys {
				enabled = append(enabled, checkMeta{
					name:  checks.RejectCheckName,
					check: checks.NewRejectCheck(true, false, re, nil, severity),
				})
			}
			if reject.LabelValues {
				enabled = append(enabled, checkMeta{
					name:  checks.RejectCheckName,
					check: checks.NewRejectCheck(true, false, nil, re, severity),
				})
			}
			if reject.AnnotationKeys {
				enabled = append(enabled, checkMeta{
					name:  checks.RejectCheckName,
					check: checks.NewRejectCheck(false, true, re, nil, severity),
				})
			}
			if reject.AnnotationValues {
				enabled = append(enabled, checkMeta{
					name:  checks.RejectCheckName,
					check: checks.NewRejectCheck(false, true, nil, re, severity),
				})
			}
		}
	}

	for _, link := range rule.RuleLink {
		severity := link.getSeverity(checks.Bug)
		re := checks.MustTemplatedRegexp(link.Regex)
		var timeout time.Duration
		if link.Timeout != "" {
			timeout, _ = parseDuration(link.Timeout)
		} else {
			timeout = time.Minute
		}
		enabled = append(enabled, checkMeta{
			name:  checks.RuleLinkCheckName,
			check: checks.NewRuleLinkCheck(re, link.URI, timeout, link.Headers, link.Comment, severity),
		})
	}

	if rule.For != nil {
		severity, minFor, maxFor := rule.For.resolve()
		enabled = append(enabled, checkMeta{
			name:  checks.RuleForCheckName,
			check: checks.NewRuleForCheck(checks.RuleForFor, minFor, maxFor, rule.For.Comment, severity),
		})
	}

	if rule.KeepFiringFor != nil {
		severity, minFor, maxFor := rule.KeepFiringFor.resolve()
		enabled = append(enabled, checkMeta{
			name:  checks.RuleForCheckName,
			check: checks.NewRuleForCheck(checks.RuleForKeepFiringFor, minFor, maxFor, rule.KeepFiringFor.Comment, severity),
		})
	}

	for _, name := range rule.RuleName {
		re := checks.MustTemplatedRegexp(name.Regex)
		severity := name.getSeverity(checks.Information)
		enabled = append(enabled, checkMeta{
			name:  checks.RuleNameCheckName,
			check: checks.NewRuleNameCheck(re, name.Comment, severity),
		})
	}

	if rule.RangeQuery != nil {
		severity := rule.RangeQuery.getSeverity(checks.Warning)
		limit, _ := parseDuration(rule.RangeQuery.Max)
		enabled = append(enabled, checkMeta{
			name:  checks.CostCheckName,
			check: checks.NewRangeQueryCheck(nil, limit, rule.RangeQuery.Comment, severity),
		})
	}

	return enabled
}

func isEnabled(enabledChecks, disabledChecks []string, rule parser.Rule, name string, check checks.RuleChecker, promTags []string) bool {
	matches := []string{
		name,
		check.String(),
	}
	for _, tag := range promTags {
		matches = append(matches, fmt.Sprintf("%s(+%s)", name, tag))
	}

	for _, disable := range comments.Only[comments.Disable](rule.Comments, comments.DisableType) {
		for _, match := range matches {
			if match == disable.Match {
				slog.Debug(
					"Check disabled by comment",
					slog.String("check", check.String()),
					slog.String("match", match),
				)
				return false
			}
		}
	}
	for _, snooze := range comments.Only[comments.Snooze](rule.Comments, comments.SnoozeType) {
		if !snooze.Until.After(time.Now()) {
			continue
		}
		for _, match := range matches {
			if match == snooze.Match {
				slog.Debug(
					"Check snoozed by comment",
					slog.String("check", check.String()),
					slog.String("match", snooze.Match),
					slog.Time("until", snooze.Until),
				)
				return false
			}
		}
	}

	for _, c := range disabledChecks {
		if c == name || c == check.String() {
			return false
		}
		for _, tag := range promTags {
			if c == fmt.Sprintf("%s(+%s)", name, tag) {
				return false
			}
		}
	}
	if len(enabledChecks) == 0 {
		return true
	}
	for _, c := range enabledChecks {
		if c == name {
			return true
		}
	}
	return false
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
