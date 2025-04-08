package config

import (
	"context"
	"slices"
	"time"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/promapi"
)

type parsedRule struct {
	match  []Match
	ignore []Match
	name   string
	check  checks.RuleChecker
	tags   []string
	locked bool
}

func newParsedRule(
	rule Rule,
	defaultStates []string,
	name string,
	check checks.RuleChecker,
	tags []string,
) parsedRule {
	return parsedRule{
		match:  defaultRuleMatch(rule.Match, defaultStates),
		ignore: rule.Ignore,
		name:   name,
		check:  check,
		tags:   tags,
		locked: rule.Locked,
	}
}

func baseParsedRule(
	match []Match,
	name string,
	check checks.RuleChecker,
	tags []string,
) parsedRule {
	return parsedRule{
		match:  match,
		ignore: nil,
		name:   name,
		check:  check,
		tags:   tags,
		locked: false,
	}
}

func isMatch(ctx context.Context, e discovery.Entry, ignore, match []Match) bool {
	for _, ignore := range ignore {
		if ignore.IsMatch(ctx, e.Path.Name, e) {
			return false
		}
	}

	if len(match) > 0 {
		var found bool
		for _, match := range match {
			if match.IsMatch(ctx, e.Path.Name, e) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

func (rule parsedRule) isEnabled(
	ctx context.Context,
	enabled, disabled []string,
	checks []checks.RuleChecker,
	e discovery.Entry,
	cfgRules []Rule,
	locked bool,
) bool {
	// Entry state is not what the check is for.
	if !slices.Contains(rule.check.Meta().States, e.State) {
		return false
	}

	// Check if check is disabled for specific Prometheus rule.
	if !isEnabled(enabled, e.DisabledChecks, e.Rule, rule.name, rule.check, rule.tags, locked) {
		return false
	}

	var enabledByConfigRule bool
	for _, cfgRule := range cfgRules {
		if !isMatch(ctx, e, cfgRule.Ignore, cfgRule.Match) {
			continue
		}
		if slices.Contains(cfgRule.Disable, rule.name) {
			return false
		}
		if slices.Contains(cfgRule.Enable, rule.name) {
			enabledByConfigRule = true
		}
	}
	if enabledByConfigRule {
		return true
	}

	// Check if rule was disabled globally.
	if !isEnabled(enabled, disabled, e.Rule, rule.name, rule.check, rule.tags, locked) {
		return false
	}
	// Check if rule was already enabled.
	var v bool
	for _, er := range checks {
		if er.String() == rule.check.String() {
			v = true
			break
		}
	}
	return !v
}

func defaultMatchStates(cmd ContextCommandVal) []string {
	switch cmd {
	case CICommand:
		return CIStates
	default:
		return AnyStates
	}
}

func baseRules(proms []*promapi.FailoverGroup, match []Match) (rules []parsedRule) {
	rules = append(rules,
		baseParsedRule(match, checks.SyntaxCheckName, checks.NewSyntaxCheck(), nil),
		baseParsedRule(match, checks.AlertForCheckName, checks.NewAlertsForCheck(), nil),
		baseParsedRule(match, checks.ComparisonCheckName, checks.NewComparisonCheck(), nil),
		baseParsedRule(match, checks.TemplateCheckName, checks.NewTemplateCheck(), nil),
		baseParsedRule(match, checks.FragileCheckName, checks.NewFragileCheck(), nil),
		baseParsedRule(match, checks.RegexpCheckName, checks.NewRegexpCheck(), nil),
		baseParsedRule(match, checks.RuleDependencyCheckName, checks.NewRuleDependencyCheck(), nil),
		baseParsedRule(match, checks.ImpossibleCheckName, checks.NewImpossibleCheck(), nil),
	)

	for _, p := range proms {
		rules = append(
			rules,
			baseParsedRule(match, checks.RateCheckName, checks.NewRateCheck(p), p.Tags()),
			baseParsedRule(match, checks.SeriesCheckName, checks.NewSeriesCheck(p), p.Tags()),
			baseParsedRule(
				match,
				checks.VectorMatchingCheckName,
				checks.NewVectorMatchingCheck(p),
				p.Tags(),
			),
			baseParsedRule(
				match,
				checks.RangeQueryCheckName,
				checks.NewRangeQueryCheck(p, 0, "", checks.Warning),
				p.Tags(),
			),
			baseParsedRule(
				match,
				checks.RuleDuplicateCheckName,
				checks.NewRuleDuplicateCheck(p),
				p.Tags(),
			),
			baseParsedRule(
				match,
				checks.LabelsConflictCheckName,
				checks.NewLabelsConflictCheck(p),
				p.Tags(),
			),
			baseParsedRule(
				match,
				checks.AlertsExternalLabelsCheckName,
				checks.NewAlertsExternalLabelsCheck(p),
				p.Tags(),
			),
			baseParsedRule(match, checks.CounterCheckName, checks.NewCounterCheck(p), p.Tags()),
			baseParsedRule(
				match,
				checks.AlertsAbsentCheckName,
				checks.NewAlertsAbsentCheck(p),
				p.Tags(),
			),
		)
	}

	return rules
}

func defaultRuleMatch(match []Match, defaultStates []string) []Match {
	if len(match) == 0 {
		return []Match{{State: defaultStates}} // nolint: exhaustruct
	}
	dst := make([]Match, 0, len(match))
	for _, m := range match {
		if len(m.State) == 0 {
			m.State = defaultStates
		}
		dst = append(dst, m)
	}
	return dst
}

func parseRule(
	rule Rule,
	prometheusServers []*promapi.FailoverGroup,
	defaultStates []string,
) (rules []parsedRule) {
	if len(rule.Aggregate) > 0 {
		var nameRegex *checks.TemplatedRegexp
		for _, aggr := range rule.Aggregate {
			if aggr.Name != "" {
				nameRegex = checks.MustTemplatedRegexp(aggr.Name)
			}
			severity := aggr.getSeverity(checks.Warning)
			for _, label := range aggr.Keep {
				rules = append(rules, newParsedRule(
					rule,
					defaultStates,
					checks.AggregationCheckName,
					checks.NewAggregationCheck(nameRegex, label, true, aggr.Comment, severity),
					nil,
				))
			}
			for _, label := range aggr.Strip {
				rules = append(rules, newParsedRule(
					rule,
					defaultStates,
					checks.AggregationCheckName,
					checks.NewAggregationCheck(nameRegex, label, false, aggr.Comment, severity),
					nil,
				))
			}
		}
	}

	if rule.Cost != nil {
		severity := rule.Cost.getSeverity(checks.Bug)
		evalDur, _ := parseDuration(rule.Cost.MaxEvaluationDuration)
		for _, prom := range prometheusServers {
			rules = append(rules, newParsedRule(
				rule,
				defaultStates,
				checks.CostCheckName,
				checks.NewCostCheck(
					prom,
					rule.Cost.MaxSeries,
					rule.Cost.MaxTotalSamples,
					rule.Cost.MaxPeakSamples,
					evalDur,
					rule.Cost.Comment,
					severity,
				),
				prom.Tags(),
			))
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
			rules = append(rules, newParsedRule(
				rule,
				defaultStates,
				checks.AnnotationCheckName,
				checks.NewAnnotationCheck(
					checks.MustTemplatedRegexp(ann.Key),
					tokenRegex,
					valueRegex,
					ann.Values,
					ann.Required,
					ann.Comment,
					severity,
				),
				nil,
			))
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
			rules = append(rules, newParsedRule(
				rule,
				defaultStates,
				checks.LabelCheckName,
				checks.NewLabelCheck(
					checks.MustTemplatedRegexp(lab.Key),
					tokenRegex,
					valueRegex,
					lab.Values,
					lab.Required,
					lab.Comment,
					severity,
				),
				nil,
			))
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
			rules = append(rules, newParsedRule(
				rule,
				defaultStates,
				checks.AlertsCheckName,
				checks.NewAlertsCheck(
					prom,
					qRange,
					qStep,
					qResolve,
					rule.Alerts.MinCount,
					rule.Alerts.Comment,
					severity,
				),
				prom.Tags(),
			))
		}
	}

	if len(rule.Reject) > 0 {
		for _, reject := range rule.Reject {
			severity := reject.getSeverity(checks.Bug)
			re := checks.MustTemplatedRegexp(reject.Regex)
			if reject.LabelKeys {
				rules = append(rules, newParsedRule(
					rule,
					defaultStates,
					checks.RejectCheckName,
					checks.NewRejectCheck(true, false, re, nil, severity),
					nil,
				))
			}
			if reject.LabelValues {
				rules = append(rules, newParsedRule(
					rule,
					defaultStates,
					checks.RejectCheckName,
					checks.NewRejectCheck(true, false, nil, re, severity),
					nil,
				))
			}
			if reject.AnnotationKeys {
				rules = append(rules, newParsedRule(
					rule,
					defaultStates,
					checks.RejectCheckName,
					checks.NewRejectCheck(false, true, re, nil, severity),
					nil,
				))
			}
			if reject.AnnotationValues {
				rules = append(rules, newParsedRule(
					rule,
					defaultStates,
					checks.RejectCheckName,
					checks.NewRejectCheck(false, true, nil, re, severity),
					nil,
				))
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
		rules = append(rules, newParsedRule(
			rule,
			defaultStates,
			checks.RuleLinkCheckName,
			checks.NewRuleLinkCheck(re, link.URI, timeout, link.Headers, link.Comment, severity),
			nil,
		))
	}

	if rule.For != nil {
		severity, minFor, maxFor := rule.For.resolve()
		rules = append(rules, newParsedRule(
			rule,
			defaultStates,
			checks.RuleForCheckName,
			checks.NewRuleForCheck(checks.RuleForFor, minFor, maxFor, rule.For.Comment, severity),
			nil,
		))
	}

	if rule.KeepFiringFor != nil {
		severity, minFor, maxFor := rule.KeepFiringFor.resolve()
		rules = append(rules, newParsedRule(
			rule,
			defaultStates,
			checks.RuleForCheckName,
			checks.NewRuleForCheck(
				checks.RuleForKeepFiringFor,
				minFor,
				maxFor,
				rule.KeepFiringFor.Comment,
				severity,
			),
			nil,
		))
	}

	for _, name := range rule.RuleName {
		re := checks.MustTemplatedRegexp(name.Regex)
		severity := name.getSeverity(checks.Information)
		rules = append(rules, newParsedRule(
			rule,
			defaultStates,
			checks.RuleNameCheckName,
			checks.NewRuleNameCheck(re, name.Comment, severity),
			nil,
		))
	}

	if rule.RangeQuery != nil {
		severity := rule.RangeQuery.getSeverity(checks.Warning)
		limit, _ := parseDuration(rule.RangeQuery.Max)
		rules = append(rules, newParsedRule(
			rule,
			defaultStates,
			checks.CostCheckName,
			checks.NewRangeQueryCheck(nil, limit, rule.RangeQuery.Comment, severity),
			nil,
		))
	}

	if rule.Report != nil {
		rules = append(rules, newParsedRule(
			rule,
			defaultStates,
			checks.CostCheckName,
			checks.NewReportCheck(rule.Report.Comment, rule.Report.getSeverity()),
			nil,
		))
	}

	return rules
}
