package checks

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/cloudflare/pint/internal/comments"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/parser/utils"
	"github.com/cloudflare/pint/internal/promapi"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	promParser "github.com/prometheus/prometheus/promql/parser"
)

type PromqlSeriesSettings struct {
	IgnoreLabelsValue     map[string][]string `hcl:"ignoreLabelsValue,optional" json:"ignoreLabelsValue,omitempty"`
	LookbackRange         string              `hcl:"lookbackRange,optional" json:"lookbackRange,omitempty"`
	LookbackStep          string              `hcl:"lookbackStep,optional" json:"lookbackStep,omitempty"`
	IgnoreMetrics         []string            `hcl:"ignoreMetrics,optional" json:"ignoreMetrics,omitempty"`
	FallbackTimeout       string              `hcl:"fallbackTimeout,optional" json:"fallbackTimeout,omitempty"`
	ignoreMetricsRe       []*regexp.Regexp
	lookbackRangeDuration time.Duration
	lookbackStepDuration  time.Duration
	fallbackTimeout       time.Duration
}

func (c *PromqlSeriesSettings) Validate() error {
	for _, re := range c.IgnoreMetrics {
		re, err := regexp.Compile("^" + re + "$")
		if err != nil {
			return err
		}
		c.ignoreMetricsRe = append(c.ignoreMetricsRe, re)
	}

	c.lookbackRangeDuration = time.Hour * 24 * 7
	if c.LookbackRange != "" {
		dur, err := model.ParseDuration(c.LookbackRange)
		if err != nil {
			return err
		}
		c.lookbackRangeDuration = time.Duration(dur)
	}

	c.lookbackStepDuration = time.Minute * 5
	if c.LookbackStep != "" {
		dur, err := model.ParseDuration(c.LookbackStep)
		if err != nil {
			return err
		}
		c.lookbackStepDuration = time.Duration(dur)
	}

	c.fallbackTimeout = time.Minute * 5
	if c.FallbackTimeout != "" {
		dur, err := model.ParseDuration(c.FallbackTimeout)
		if err != nil {
			return err
		}
		c.fallbackTimeout = time.Duration(dur)
	}

	for selector := range c.IgnoreLabelsValue {
		if _, err := promParser.ParseMetricSelector(selector); err != nil {
			return fmt.Errorf("%q is not a valid PromQL metric selector: %w", selector, err)
		}
	}

	return nil
}

const (
	SeriesCheckName        = "promql/series"
	SeriesCheckRuleDetails = `This usually means that you're deploying a set of rules where one is using the metric produced by another rule.
To avoid false positives pint won't run series checks here but that doesn't guarantee that there are no problems here.
To fully validate your changes it's best to first deploy the rules that generate the time series needed by other rules.
[Click here](https://cloudflare.github.io/pint/checks/promql/series.html#your-query-is-using-recording-rules) for more details.
`
	SeriesCheckCommonProblemDetails = `[Click here](https://cloudflare.github.io/pint/checks/promql/series.html#common-problems) to see a list of common problems that might cause this.`
	SeriesCheckMinAgeDetails        = `You have a comment that tells pint how long can a metric be missing before it warns you about it but this comment is not formatted correctly.
[Click here](https://cloudflare.github.io/pint/checks/promql/series.html#min-age) to see supported syntax.`
	SeriesCheckUnusedDisableComment = "One of the `# pint disable promql/series` comments used in this rule doesn't have any effect and won't disable anything. Make sure that the comment targets series that are used in the rule query and are not already ignored.\n[Click here](https://cloudflare.github.io/pint/checks/promql/series.html#how-to-disable-it) to see docs about disable comment syntax."
	SeriesCheckUnusedRuleSetComment = "One of the `# pint rule/set promql/series` comments used in this rule doesn't have any effect. Make sure that the comment targets series and labels that are used in the rule query and are not already ignored.\n[Click here](https://cloudflare.github.io/pint/checks/promql/series.html#ignorelabel-value) for docs about comment syntax."
)

func NewSeriesCheck(prom *promapi.FailoverGroup) SeriesCheck {
	return SeriesCheck{prom: prom}
}

func (c SeriesCheck) Meta() CheckMeta {
	return CheckMeta{
		States: []discovery.ChangeType{
			discovery.Noop,
			discovery.Added,
			discovery.Modified,
			discovery.Moved,
		},
		Online:        true,
		AlwaysEnabled: false,
	}
}

type SeriesCheck struct {
	prom *promapi.FailoverGroup
}

func (c SeriesCheck) String() string {
	return fmt.Sprintf("%s(%s)", SeriesCheckName, c.prom.Name())
}

func (c SeriesCheck) Reporter() string {
	return SeriesCheckName
}

func (c SeriesCheck) Check(ctx context.Context, _ discovery.Path, rule parser.Rule, entries []discovery.Entry) (problems []Problem) {
	var settings *PromqlSeriesSettings
	if s := ctx.Value(SettingsKey(c.Reporter())); s != nil {
		settings = s.(*PromqlSeriesSettings)
	}
	if settings == nil {
		settings = &PromqlSeriesSettings{}
		_ = settings.Validate()
	}

	expr := rule.Expr()

	if expr.SyntaxError != nil {
		return problems
	}

	params := promapi.NewRelativeRange(settings.lookbackRangeDuration, settings.lookbackStepDuration)

	selectors := getNonFallbackSelectors(expr)

	done := map[string]bool{}
	for _, selector := range selectors {
		if _, ok := done[selector.String()]; ok {
			continue
		}

		done[selector.String()] = true

		if isDisabled(rule, selector) {
			done[selector.String()] = true
			continue
		}

		metricName := selector.Name
		if metricName == "" {
			for _, lm := range selector.LabelMatchers {
				if lm.Name == labels.MetricName && lm.Type == labels.MatchEqual {
					metricName = lm.Value
					break
				}
			}
		}

		// 0. Special case for alert metrics
		if metricName == "ALERTS" || metricName == "ALERTS_FOR_STATE" {
			var alertname string
			for _, lm := range selector.LabelMatchers {
				if lm.Name == "alertname" && lm.Type != labels.MatchRegexp && lm.Type != labels.MatchNotRegexp {
					alertname = lm.Value
				}
			}
			var arEntry *discovery.Entry
			if alertname != "" {
				for _, entry := range entries {
					if entry.Rule.AlertingRule != nil &&
						entry.Rule.Error.Err == nil &&
						entry.Rule.AlertingRule.Alert.Value == alertname {
						arEntry = &entry
						break
					}
				}
				if arEntry != nil {
					slog.Debug(
						"Metric is provided by alerting rule",
						slog.String("selector", selector.String()),
						slog.String("path", arEntry.Path.Name),
					)
				} else {
					problems = append(problems, Problem{
						Lines:    expr.Value.Lines,
						Reporter: c.Reporter(),
						Text:     fmt.Sprintf("`%s` metric is generated by alerts but didn't found any rule named `%s`.", selector.String(), alertname),
						Details:  SeriesCheckCommonProblemDetails,
						Severity: Bug,
					})
				}
			}
			// ALERTS{} query with no alertname, all good
			continue
		}

		labelNames := []string{}
		for _, lm := range selector.LabelMatchers {
			if lm.Name == labels.MetricName {
				continue
			}
			if lm.Type == labels.MatchNotEqual || lm.Type == labels.MatchNotRegexp {
				continue
			}
			if slices.Contains(labelNames, lm.Name) {
				continue
			}
			labelNames = append(labelNames, lm.Name)
		}

		// 1. If foo{bar, baz} is there -> GOOD
		slog.Debug("Checking if selector returns anything", slog.String("check", c.Reporter()), slog.String("selector", selector.String()))
		count, err := c.instantSeriesCount(ctx, fmt.Sprintf("count(%s)", selector.String()))
		if err != nil {
			problems = append(problems, c.queryProblem(err, expr))
			continue
		}
		if count > 0 {
			slog.Debug("Found series, skipping further checks", slog.String("check", c.Reporter()), slog.String("selector", selector.String()))
			continue
		}

		promUptime, err := c.prom.RangeQuery(ctx, fmt.Sprintf("count(%s)", c.prom.UptimeMetric()), params)
		if err != nil {
			slog.Warn("Cannot detect Prometheus uptime gaps", slog.Any("err", err), slog.String("name", c.prom.Name()))
		}
		if promUptime != nil && promUptime.Series.Ranges.Len() == 0 {
			slog.Warn(
				"No results for Prometheus uptime metric, you might have set uptime config option to a missing metric, please check your config",
				slog.String("name", c.prom.Name()),
				slog.String("metric", c.prom.UptimeMetric()),
			)
		}
		if promUptime == nil || promUptime.Series.Ranges.Len() == 0 {
			slog.Warn(
				"Using dummy Prometheus uptime metric results with no gaps",
				slog.String("name", c.prom.Name()),
				slog.String("metric", c.prom.UptimeMetric()),
			)
			promUptime = &promapi.RangeQueryResult{ // nolint: exhaustruct
				URI: c.prom.URI(),
				Series: promapi.SeriesTimeRanges{
					From:  params.Start(),
					Until: params.End(),
					Step:  params.Step(),
					Ranges: promapi.MetricTimeRanges{
						{
							Fingerprint: 0,
							Labels:      labels.Labels{},
							Start:       params.Start(),
							End:         params.End(),
						},
					},
					Gaps: nil,
				},
			}
		}

		bareSelector := stripLabels(selector)

		// 2. If foo was NEVER there -> BUG
		slog.Debug("Checking if base metric has historical series", slog.String("check", c.Reporter()), slog.String("selector", (&bareSelector).String()))
		trs, err := c.prom.RangeQuery(ctx, fmt.Sprintf("count(%s)", bareSelector.String()), params)
		if err != nil {
			problems = append(problems, c.queryProblem(err, expr))
			continue
		}
		trs.Series.FindGaps(promUptime.Series, trs.Series.From, trs.Series.Until)
		if len(trs.Series.Ranges) == 0 {
			// Check if we have recording rule that provides this metric before we give up
			var rrEntry *discovery.Entry
			for _, entry := range entries {
				if entry.Rule.RecordingRule != nil &&
					entry.Rule.Error.Err == nil &&
					entry.Rule.RecordingRule.Record.Value == bareSelector.String() {
					rrEntry = &entry
					break
				}
			}
			if rrEntry != nil {
				// Validate recording rule instead
				slog.Debug("Metric is provided by recording rule", slog.String("selector", (&bareSelector).String()), slog.String("path", rrEntry.Path.Name))
				problems = append(problems, Problem{
					Lines:    expr.Value.Lines,
					Reporter: c.Reporter(),
					Text: fmt.Sprintf("%s didn't have any series for `%s` metric in the last %s but found recording rule that generates it, skipping further checks.",
						promText(c.prom.Name(), trs.URI), bareSelector.String(), sinceDesc(trs.Series.From)),
					Details:  SeriesCheckRuleDetails,
					Severity: Information,
				})
				continue
			}

			text, severity := c.textAndSeverity(
				settings,
				bareSelector.String(),
				fmt.Sprintf("%s didn't have any series for `%s` metric in the last %s.",
					promText(c.prom.Name(), trs.URI),
					bareSelector.String(),
					sinceDesc(trs.Series.From),
				),
				Bug,
			)
			problems = append(problems, Problem{
				Lines:    expr.Value.Lines,
				Reporter: c.Reporter(),
				Text:     text,
				Details:  c.checkOtherServer(ctx, selector.String(), settings.fallbackTimeout),
				Severity: severity,
			})
			slog.Debug("No historical series for base metric", slog.String("check", c.Reporter()), slog.String("selector", (&bareSelector).String()))
			continue
		}

		// 3. If foo is ALWAYS/SOMETIMES there BUT {bar OR baz} is NEVER there -> BUG
		for _, name := range labelNames {
			l := stripLabels(selector)
			l.LabelMatchers = append(l.LabelMatchers, labels.MustNewMatcher(labels.MatchRegexp, name, ".+"))
			slog.Debug("Checking if base metric has historical series with required label", slog.String("check", c.Reporter()), slog.String("selector", (&l).String()), slog.String("label", name))
			trsLabelCount, err := c.prom.RangeQuery(ctx, fmt.Sprintf("absent(%s)", l.String()), params)
			if err != nil {
				problems = append(problems, c.queryProblem(err, expr))
				continue
			}
			trsLabelCount.Series.FindGaps(promUptime.Series, trsLabelCount.Series.From, trsLabelCount.Series.Until)

			var isAbsentInsideSeriesRange bool
			for _, lr := range trsLabelCount.Series.Ranges {
				for _, str := range trs.Series.Ranges {
					if _, ok := promapi.Overlaps(lr, str, trsLabelCount.Series.Step); ok {
						isAbsentInsideSeriesRange = true
					}
				}
			}
			if !isAbsentInsideSeriesRange {
				continue
			}

			if trsLabelCount.Series.Ranges.Len() == 1 && len(trsLabelCount.Series.Gaps) == 0 {
				problems = append(problems, Problem{
					Lines:    expr.Value.Lines,
					Reporter: c.Reporter(),
					Text: fmt.Sprintf(
						"%s has `%s` metric but there are no series with `%s` label in the last %s.",
						promText(c.prom.Name(), trsLabelCount.URI), bareSelector.String(), name, sinceDesc(trsLabelCount.Series.From)),
					Details:  SeriesCheckCommonProblemDetails,
					Severity: Bug,
				})
				slog.Debug("No historical series with label used for the query", slog.String("check", c.Reporter()), slog.String("selector", (&l).String()), slog.String("label", name))
			}
		}
		if len(problems) > 0 {
			continue
		}

		// 4. If foo was ALWAYS there but it's NO LONGER there (for more than min-age) -> BUG
		if len(trs.Series.Ranges) == 1 &&
			!oldest(trs.Series.Ranges).After(trs.Series.From.Add(settings.lookbackStepDuration)) &&
			newest(trs.Series.Ranges).Before(trs.Series.Until.Add(settings.lookbackStepDuration*-1)) {

			minAge, p := c.getMinAge(rule, selector)
			if len(p) > 0 {
				problems = append(problems, p...)
			}

			if !newest(trs.Series.Ranges).Before(trs.Series.Until.Add(minAge * -1)) {
				slog.Debug(
					"Series disappeared from prometheus but for less then configured min-age",
					slog.String("check", c.Reporter()),
					slog.String("selector", selector.String()),
					slog.String("min-age", output.HumanizeDuration(minAge)),
					slog.String("last-seen", sinceDesc(newest(trs.Series.Ranges))),
				)
				continue
			}

			text, severity := c.textAndSeverity(
				settings,
				bareSelector.String(),
				fmt.Sprintf(
					"%s doesn't currently have `%s`, it was last present %s ago.",
					promText(c.prom.Name(), trs.URI), bareSelector.String(), sinceDesc(newest(trs.Series.Ranges))),
				Bug,
			)
			problems = append(problems, Problem{
				Lines:    expr.Value.Lines,
				Reporter: c.Reporter(),
				Text:     text,
				Details:  SeriesCheckCommonProblemDetails,
				Severity: severity,
			})
			slog.Debug("Series disappeared from prometheus", slog.String("check", c.Reporter()), slog.String("selector", (&bareSelector).String()))
			continue
		}

		for _, lm := range selector.LabelMatchers {
			if lm.Name == labels.MetricName {
				continue
			}
			if lm.Type != labels.MatchEqual && lm.Type != labels.MatchRegexp {
				continue
			}
			if c.isLabelValueIgnored(settings, rule, selector, lm.Name) {
				continue
			}
			labelSelector := promParser.VectorSelector{
				Name:          metricName,
				LabelMatchers: []*labels.Matcher{lm},
			}
			addNameSelectorIfNeeded(&labelSelector, selector.LabelMatchers)
			slog.Debug("Checking if there are historical series matching filter", slog.String("check", c.Reporter()), slog.String("selector", (&labelSelector).String()), slog.String("matcher", lm.String()))

			trsLabel, err := c.prom.RangeQuery(ctx, fmt.Sprintf("count(%s)", labelSelector.String()), params)
			if err != nil {
				problems = append(problems, c.queryProblem(err, expr))
				continue
			}
			trsLabel.Series.FindGaps(promUptime.Series, trsLabel.Series.From, trsLabel.Series.Until)

			// 5. If foo is ALWAYS/SOMETIMES there BUT {bar OR baz} value is NEVER there -> BUG
			if len(trsLabel.Series.Ranges) == 0 {
				text, severity := c.textAndSeverity(
					settings,
					bareSelector.String(),
					fmt.Sprintf(
						"%s has `%s` metric with `%s` label but there are no series matching `{%s}` in the last %s.",
						promText(c.prom.Name(), trsLabel.URI), bareSelector.String(), lm.Name, lm.String(), sinceDesc(trs.Series.From)),
					Bug,
				)
				problems = append(problems, Problem{
					Lines:    expr.Value.Lines,
					Reporter: c.Reporter(),
					Text:     text,
					Details:  SeriesCheckCommonProblemDetails,
					Severity: severity,
				})
				slog.Debug("No historical series matching filter used in the query",
					slog.String("check", c.Reporter()), slog.String("selector", selector.String()), slog.String("matcher", lm.String()))
				continue
			}

			// 6. If foo is ALWAYS/SOMETIMES there AND {bar OR baz} used to be there ALWAYS BUT it's NO LONGER there -> BUG
			if len(trsLabel.Series.Ranges) == 1 &&
				!oldest(trsLabel.Series.Ranges).After(trsLabel.Series.Until.Add(settings.lookbackRangeDuration-1).Add(settings.lookbackStepDuration)) &&
				newest(trsLabel.Series.Ranges).Before(trsLabel.Series.Until.Add(settings.lookbackStepDuration*-1)) {

				var labelGapOutsideBaseGaps bool
				for _, lg := range trsLabel.Series.Gaps {
					a := promapi.MetricTimeRange{Start: lg.Start, End: lg.End}
					var ok bool
					for _, bg := range trs.Series.Gaps {
						b := promapi.MetricTimeRange{Start: bg.Start, End: bg.End}
						_, ov := promapi.Overlaps(a, b, trs.Series.Step)
						if ov {
							ok = true
						}
					}
					if !ok {
						labelGapOutsideBaseGaps = true
					}
				}

				if !labelGapOutsideBaseGaps {
					continue
				}

				minAge, p := c.getMinAge(rule, selector)
				if len(p) > 0 {
					problems = append(problems, p...)
				}

				if !newest(trsLabel.Series.Ranges).Before(trsLabel.Series.Until.Add(minAge * -1)) {
					slog.Debug(
						"Series disappeared from prometheus but for less then configured min-age",
						slog.String("check", c.Reporter()),
						slog.String("selector", selector.String()),
						slog.String("min-age", output.HumanizeDuration(minAge)),
						slog.String("last-seen", sinceDesc(newest(trsLabel.Series.Ranges))),
					)
					continue
				}

				text, severity := c.textAndSeverity(
					settings,
					bareSelector.String(),
					fmt.Sprintf(
						"%s has `%s` metric but doesn't currently have series matching `{%s}`, such series was last present %s ago.",
						promText(c.prom.Name(), trs.URI), bareSelector.String(), lm.String(), sinceDesc(newest(trsLabel.Series.Ranges))),
					Bug,
				)
				problems = append(problems, Problem{
					Lines:    expr.Value.Lines,
					Reporter: c.Reporter(),
					Text:     text,
					Details:  SeriesCheckCommonProblemDetails,
					Severity: severity,
				})
				slog.Debug(
					"Series matching filter disappeared from prometheus",
					slog.String("check", c.Reporter()),
					slog.String("selector", selector.String()),
					slog.String("matcher", lm.String()),
				)
				continue
			}

			// 7. if foo is ALWAYS/SOMETIMES there BUT {bar OR baz} value is SOMETIMES there -> WARN
			if len(trsLabel.Series.Ranges) > 1 && len(trsLabel.Series.Gaps) > 0 {
				problems = append(problems, Problem{
					Lines:    expr.Value.Lines,
					Reporter: c.Reporter(),
					Text: fmt.Sprintf(
						"Metric `%s` with label `{%s}` is only sometimes present on %s with average life span of %s.",
						bareSelector.String(), lm.String(), promText(c.prom.Name(), trs.URI),
						output.HumanizeDuration(avgLife(trsLabel.Series.Ranges))),
					Details:  SeriesCheckCommonProblemDetails,
					Severity: Warning,
				})
				slog.Debug(
					"Series matching filter are only sometimes present",
					slog.String("check", c.Reporter()),
					slog.String("selector", selector.String()),
					slog.String("matcher", lm.String()),
				)
			}
		}
		if len(problems) > 0 {
			continue
		}

		// 8. If foo is SOMETIMES there -> WARN
		if len(trs.Series.Ranges) > 0 && len(trs.Series.Gaps) > 0 {
			problems = append(problems, Problem{
				Lines:    expr.Value.Lines,
				Reporter: c.Reporter(),
				Text: fmt.Sprintf(
					"Metric `%s` is only sometimes present on %s with average life span of %s in the last %s.",
					bareSelector.String(), promText(c.prom.Name(), trs.URI), output.HumanizeDuration(avgLife(trs.Series.Ranges)), sinceDesc(trs.Series.From)),
				Details:  SeriesCheckCommonProblemDetails,
				Severity: Warning,
			})
			slog.Debug(
				"Metric only sometimes present",
				slog.String("check", c.Reporter()),
				slog.String("selector", (&bareSelector).String()),
			)
		}
	}

	for _, disable := range orphanedDisableComments(ctx, rule, selectors) {
		problems = append(problems, Problem{
			Lines:    expr.Value.Lines,
			Reporter: c.Reporter(),
			Text:     fmt.Sprintf("pint %s comment `%s` doesn't match any selector in this query", comments.DisableComment, disable.Match),
			Details:  SeriesCheckUnusedDisableComment,
			Severity: Warning,
		})
	}
	for _, ruleSet := range orphanedRuleSetComments(rule, selectors) {
		problems = append(problems, Problem{
			Lines:    expr.Value.Lines,
			Reporter: c.Reporter(),
			Text:     fmt.Sprintf("pint %s comment `%s` doesn't match any label matcher in this query", comments.RuleSetComment, ruleSet.Value),
			Details:  SeriesCheckUnusedRuleSetComment,
			Severity: Warning,
		})
	}

	return problems
}

func (c SeriesCheck) checkOtherServer(ctx context.Context, query string, timeout time.Duration) string {
	var servers []*promapi.FailoverGroup
	if val := ctx.Value(promapi.AllPrometheusServers); val != nil {
		for _, s := range val.([]*promapi.FailoverGroup) {
			if s.Name() == c.prom.Name() {
				continue
			}
			servers = append(servers, s)
		}
	}

	if len(servers) == 0 {
		return SeriesCheckCommonProblemDetails
	}

	var suffix string
	var buf strings.Builder
	buf.WriteRune('`')
	buf.WriteString(query)
	buf.WriteString("` was found on other prometheus servers:\n\n")

	start := time.Now()
	var tested, matches, skipped int
	for _, prom := range servers {
		if time.Since(start) >= timeout {
			slog.Debug("Time limit reached for checking if metric exists on any other Prometheus server",
				slog.String("check", c.Reporter()),
				slog.String("selector", query),
			)
			suffix = fmt.Sprintf("\npint tried to check %d server(s) but stopped after checking %d server(s) due to reaching time limit (%s).\n",
				len(servers), tested, output.HumanizeDuration(timeout))
			break
		}

		slog.Debug("Checking if metric exists on any other Prometheus server",
			slog.String("check", c.Reporter()),
			slog.String("name", prom.Name()),
			slog.String("selector", query),
		)

		tested++
		qr, err := prom.Query(ctx, fmt.Sprintf("count(%s)", query))
		if err != nil {
			continue
		}

		var series int
		for _, s := range qr.Series {
			series += int(s.Value)
		}

		uri := prom.URI()

		if series > 0 {
			matches++
			if matches > 10 {
				skipped++
				continue
			}
			buf.WriteString("- [")
			buf.WriteString(prom.Name())
			buf.WriteString("](")
			buf.WriteString(uri)
			buf.WriteString("/graph?g0.expr=")
			buf.WriteString(query)
			buf.WriteString(")\n")
		}
	}
	if skipped > 0 {
		buf.WriteString("- and ")
		buf.WriteString(strconv.Itoa(skipped))
		buf.WriteString(" other server(s).\n")
	}
	buf.WriteString(suffix)

	buf.WriteString("\nYou might be trying to deploy this rule to the wrong Prometheus server instance.\n")

	if matches > 0 {
		return buf.String()
	}

	return SeriesCheckCommonProblemDetails
}

func (c SeriesCheck) queryProblem(err error, expr parser.PromQLExpr) Problem {
	text, severity := textAndSeverityFromError(err, c.Reporter(), c.prom.Name(), Bug)
	return Problem{
		Lines:    expr.Value.Lines,
		Reporter: c.Reporter(),
		Text:     text,
		Severity: severity,
	}
}

func (c SeriesCheck) instantSeriesCount(ctx context.Context, query string) (int, error) {
	qr, err := c.prom.Query(ctx, query)
	if err != nil {
		return 0, err
	}

	var series int
	for _, s := range qr.Series {
		series += int(s.Value)
	}

	return series, nil
}

func (c SeriesCheck) getMinAge(rule parser.Rule, selector *promParser.VectorSelector) (minAge time.Duration, problems []Problem) {
	minAge = time.Hour * 2
	for _, ruleSet := range comments.Only[comments.RuleSet](rule.Comments, comments.RuleSetType) {
		matcher, key, value := parseRuleSet(ruleSet.Value)
		if key != "min-age" {
			continue
		}
		if matcher != "" {
			isMatch, _ := matchSelectorToMetric(selector, matcher)
			if !isMatch {
				continue
			}
		}
		dur, err := model.ParseDuration(value)
		if err != nil {
			problems = append(problems, Problem{
				Lines:    rule.Lines,
				Reporter: c.Reporter(),
				Text:     fmt.Sprintf("Failed to parse pint comment as duration: %s", err),
				Details:  SeriesCheckMinAgeDetails,
				Severity: Warning,
			})
		} else {
			minAge = time.Duration(dur)
		}
	}

	return minAge, problems
}

func (c SeriesCheck) isLabelValueIgnored(settings *PromqlSeriesSettings, rule parser.Rule, selector *promParser.VectorSelector, labelName string) bool {
	for matcher, names := range settings.IgnoreLabelsValue {
		isMatch, _ := matchSelectorToMetric(selector, matcher)
		if !isMatch {
			continue
		}
		if slices.Contains(names, labelName) {
			slog.Debug("Label check disabled globally via config", slog.String("label", labelName))
			return true
		}
	}
	for _, ruleSet := range comments.Only[comments.RuleSet](rule.Comments, comments.RuleSetType) {
		matcher, key, value := parseRuleSet(ruleSet.Value)
		if key != "ignore/label-value" {
			continue
		}
		if matcher != "" {
			isMatch, _ := matchSelectorToMetric(selector, matcher)
			if !isMatch {
				continue
			}
		}
		if labelName == value {
			slog.Debug("Label check disabled by comment", slog.String("selector", selector.String()), slog.String("label", labelName))
			return true
		}
	}
	return false
}

func (c SeriesCheck) textAndSeverity(settings *PromqlSeriesSettings, name, text string, s Severity) (string, Severity) {
	for _, re := range settings.ignoreMetricsRe {
		if name != "" && re.MatchString(name) {
			slog.Debug(
				"Metric matches check ignore rules",
				slog.String("check", c.Reporter()),
				slog.String("metric", name),
				slog.String("regexp", re.String()))
			return fmt.Sprintf("%s Metric name `%s` matches `%s` check ignore regexp `%s`.", text, name, c.Reporter(), re), Warning
		}
	}
	return text, s
}

func selectorWithoutOffset(vs *promParser.VectorSelector) *promParser.VectorSelector {
	if vs.OriginalOffset == 0 {
		return vs
	}

	s := &promParser.VectorSelector{}
	*s = *vs
	s.Offset = 0
	s.OriginalOffset = 0
	return s
}

func sourceHasFallback(src []utils.Source) bool {
	for _, ls := range src {
		if ls.AlwaysReturns && len(ls.ReturnedNumbers) > 0 {
			return true
		}
	}
	return false
}

func getNonFallbackSelectors(n parser.PromQLExpr) (selectors []*promParser.VectorSelector) {
	sources := utils.LabelsSource(n.Value.Value, n.Query.Expr)
	hasVectorFallback := sourceHasFallback(sources)
	for _, ls := range sources {
		if !hasVectorFallback {
			if ls.Selector != nil {
				selectors = append(selectors, selectorWithoutOffset(ls.Selector))
			}
		}
		if !sourceHasFallback(ls.Joins) {
			for _, js := range ls.Joins {
				if js.Selector != nil {
					selectors = append(selectors, selectorWithoutOffset(js.Selector))
				}
			}
		}
		for _, us := range ls.Unless {
			if !us.IsConditional {
				continue
			}
			if us.Selector != nil {
				selectors = append(selectors, selectorWithoutOffset(us.Selector))
			}
		}
	}
	return selectors
}

func stripLabels(selector *promParser.VectorSelector) promParser.VectorSelector {
	s := promParser.VectorSelector{
		Name:          selector.Name,
		LabelMatchers: []*labels.Matcher{},
	}
	for _, lm := range selector.LabelMatchers {
		if lm.Name == labels.MetricName {
			s.LabelMatchers = append(s.LabelMatchers, lm)
			if lm.Type == labels.MatchEqual {
				s.Name = lm.Value
			}
		}
	}
	return s
}

func isDisabled(rule parser.Rule, selector *promParser.VectorSelector) bool {
	for _, disable := range comments.Only[comments.Disable](rule.Comments, comments.DisableType) {
		if strings.HasPrefix(disable.Match, SeriesCheckName+"(") && strings.HasSuffix(disable.Match, ")") {
			cs := strings.TrimSuffix(strings.TrimPrefix(disable.Match, SeriesCheckName+"("), ")")
			isMatch, ok := matchSelectorToMetric(selector, cs)
			if !ok {
				continue
			}
			if !isMatch {
				goto NEXT
			}
			return true
		}
	NEXT:
	}
	return false
}

func matchSelectorToMetric(selector *promParser.VectorSelector, metric string) (bool, bool) {
	// Try full string or name match first.
	if metric == selector.String() || metric == selector.Name {
		return true, true
	}
	// Then try matchers.
	m, err := promParser.ParseMetricSelector(metric)
	if err != nil {
		// Ignore errors
		return false, false
	}
	for _, l := range m {
		var isMatch bool
		for _, s := range selector.LabelMatchers {
			if s.Type == l.Type && s.Name == l.Name && s.Value == l.Value {
				return true, true
			}
		}
		if !isMatch {
			return false, true
		}
	}
	return false, true
}

func parseRuleSet(s string) (matcher, key, value string) {
	if strings.HasPrefix(s, SeriesCheckName+"(") {
		matcher = strings.TrimPrefix(s[:strings.LastIndex(s, ")")], SeriesCheckName+"(")
		s = s[strings.LastIndex(s, ")")+1:]
	} else {
		s = strings.TrimPrefix(s, SeriesCheckName)
	}
	parts := strings.Fields(s)
	if len(parts) > 0 {
		key = parts[0]
	}
	if len(parts) > 1 {
		value = strings.Join(parts[1:], " ")
	}
	return matcher, key, value
}

func orphanedDisableComments(ctx context.Context, rule parser.Rule, selectors []*promParser.VectorSelector) (orhpaned []comments.Disable) {
	var promNames, promTags []string
	if val := ctx.Value(promapi.AllPrometheusServers); val != nil {
		for _, server := range val.([]*promapi.FailoverGroup) {
			promNames = append(promNames, server.Name())
			promTags = append(promTags, server.Tags()...)
		}
	}

	for _, disable := range comments.Only[comments.Disable](rule.Comments, comments.DisableType) {
		match := strings.TrimSuffix(strings.TrimPrefix(disable.Match, SeriesCheckName+"("), ")")
		// Skip matching tags.
		if strings.HasPrefix(match, "+") && slices.Contains(promTags, strings.TrimPrefix(match, "+")) {
			continue
		}
		// Skip matching Prometheus servers.
		if slices.Contains(promNames, match) {
			continue
		}
		var wasUsed bool
		if !strings.HasPrefix(disable.Match, SeriesCheckName+"(") || !strings.HasSuffix(disable.Match, ")") {
			continue
		}
		for _, selector := range selectors {
			isMatch, ok := matchSelectorToMetric(selector, match)
			if !ok {
				continue
			}
			if isMatch {
				wasUsed = true
				goto NEXT
			}
		}
	NEXT:
		if !wasUsed {
			orhpaned = append(orhpaned, disable)
		}
	}
	return orhpaned
}

func orphanedRuleSetComments(rule parser.Rule, selectors []*promParser.VectorSelector) (orhpaned []comments.RuleSet) {
	for _, ruleSet := range comments.Only[comments.RuleSet](rule.Comments, comments.RuleSetType) {
		var wasUsed bool
		matcher, key, value := parseRuleSet(ruleSet.Value)
		for _, selector := range selectors {
			if matcher != "" {
				isMatch, _ := matchSelectorToMetric(selector, matcher)
				if !isMatch {
					continue
				}
			}
			switch key {
			case "min-age":
				wasUsed = true
			case "ignore/label-value":
				for _, lm := range selector.LabelMatchers {
					if lm.Name == value {
						wasUsed = true
						goto NEXT
					}
				}
			}
		}
	NEXT:
		if !wasUsed {
			orhpaned = append(orhpaned, ruleSet)
		}
	}
	return orhpaned
}

func sinceDesc(t time.Time) (s string) {
	dur := time.Since(t)
	if dur > time.Hour*24 {
		return output.HumanizeDuration(dur.Round(time.Hour))
	}
	return output.HumanizeDuration(dur.Round(time.Minute))
}

func avgLife(ranges []promapi.MetricTimeRange) (d time.Duration) {
	for _, r := range ranges {
		// ranges are aligned to $(step - 1 second) so here we add that second back
		// to have more round results
		d += r.End.Sub(r.Start) + time.Second
	}
	if len(ranges) == 0 {
		return time.Duration(0)
	}
	return time.Second * time.Duration(int(d.Seconds())/len(ranges))
}

func oldest(ranges []promapi.MetricTimeRange) (ts time.Time) {
	for _, r := range ranges {
		if ts.IsZero() || r.Start.Before(ts) {
			ts = r.Start
		}
	}
	return ts
}

func newest(ranges []promapi.MetricTimeRange) (ts time.Time) {
	for _, r := range ranges {
		if ts.IsZero() || r.End.After(ts) {
			ts = r.End
		}
	}
	return ts
}

func addNameSelectorIfNeeded(selector *promParser.VectorSelector, matchers []*labels.Matcher) {
	if selector.Name != "" {
		return
	}
	for _, lm := range selector.LabelMatchers {
		if lm.Name == model.MetricNameLabel {
			return
		}
	}

	for _, lm := range matchers {
		if lm.Name == model.MetricNameLabel {
			selector.LabelMatchers = append(selector.LabelMatchers, lm)
		}
	}
}
