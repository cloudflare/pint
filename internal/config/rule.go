package config

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
)

const (
	alertingRuleType  = "alerting"
	recordingRuleType = "recording"
)

type (
	ContextCommandKey string
	ContextCommandVal string
)

var (
	CommandKey   ContextCommandKey = "command"
	CICommand    ContextCommandVal = "ci"
	LintCommand  ContextCommandVal = "lint"
	WatchCommand ContextCommandVal = "watch"
)

type MatchLabel struct {
	Key   string `hcl:",label" json:"key"`
	Value string `hcl:"value" json:"value"`
}

func (ml MatchLabel) validate() error {
	if _, err := regexp.Compile(ml.Key); err != nil {
		return err
	}
	if _, err := regexp.Compile(ml.Value); err != nil {
		return err
	}
	return nil
}

func (ml MatchLabel) isMatching(rule parser.Rule) bool {
	keyRe := strictRegex(ml.Key)
	valRe := strictRegex(ml.Value)

	if rule.AlertingRule != nil {
		if rule.AlertingRule.Labels != nil {
			for _, labl := range rule.AlertingRule.Labels.Items {
				if keyRe.MatchString(labl.Key.Value) && valRe.MatchString(labl.Value.Value) {
					return true
				}
			}
		}
	}
	if rule.RecordingRule != nil {
		if rule.RecordingRule.Labels != nil {
			for _, labl := range rule.RecordingRule.Labels.Items {
				if keyRe.MatchString(labl.Key.Value) && valRe.MatchString(labl.Value.Value) {
					return true
				}
			}
		}
	}

	return false
}

type MatchAnnotation struct {
	Key   string `hcl:",label" json:"key"`
	Value string `hcl:"value" json:"value"`
}

func (ma MatchAnnotation) validate() error {
	if _, err := regexp.Compile(ma.Key); err != nil {
		return err
	}
	if _, err := regexp.Compile(ma.Value); err != nil {
		return err
	}
	return nil
}

func (ma MatchAnnotation) isMatching(rule parser.Rule) bool {
	keyRe := strictRegex(ma.Key)
	valRe := strictRegex(ma.Value)

	if rule.AlertingRule == nil || rule.AlertingRule.Annotations == nil {
		return false
	}
	for _, ann := range rule.AlertingRule.Annotations.Items {
		if keyRe.MatchString(ann.Key.Value) && valRe.MatchString(ann.Value.Value) {
			return true
		}
	}
	return false
}

type Match struct {
	Path       string             `hcl:"path,optional" json:"path,omitempty"`
	Name       string             `hcl:"name,optional" json:"name,omitempty"`
	Kind       string             `hcl:"kind,optional" json:"kind,omitempty"`
	Label      *MatchLabel        `hcl:"label,block" json:"label,omitempty"`
	Annotation *MatchAnnotation   `hcl:"annotation,block" json:"annotation,omitempty"`
	Command    *ContextCommandVal `hcl:"command,optional" json:"command,omitempty"`
}

func (m Match) validate(allowEmpty bool) error {
	if _, err := regexp.Compile(m.Path); err != nil {
		return err
	}

	if _, err := regexp.Compile(m.Name); err != nil {
		return err
	}

	switch m.Kind {
	case "":
		// not set
	case alertingRuleType, recordingRuleType:
		// pass
	default:
		return fmt.Errorf("unknown rule type: %s", m.Kind)
	}

	if m.Label != nil {
		if err := m.Label.validate(); err != nil {
			return err
		}
	}

	if m.Annotation != nil {
		if err := m.Annotation.validate(); err != nil {
			return err
		}
	}

	if !allowEmpty && m.Path == "" && m.Name == "" && m.Kind == "" && m.Label == nil && m.Annotation == nil && m.Command == nil {
		return fmt.Errorf("ignore block must have at least one condition")
	}

	return nil
}

func (m Match) IsMatch(ctx context.Context, path string, r parser.Rule) bool {
	if m.Kind != "" {
		if r.AlertingRule != nil && m.Kind != alertingRuleType {
			return false
		}
		if r.RecordingRule != nil && m.Kind != recordingRuleType {
			return false
		}
	}

	if m.Path != "" {
		re := strictRegex(m.Path)
		if !re.MatchString(path) {
			return false
		}
	}

	if m.Name != "" {
		re := strictRegex(m.Name)
		if r.AlertingRule != nil && !re.MatchString(r.AlertingRule.Alert.Value.Value) {
			return false
		}
		if r.RecordingRule != nil && !re.MatchString(r.RecordingRule.Record.Value.Value) {
			return false
		}
	}

	if m.Label != nil {
		if !m.Label.isMatching(r) {
			return false
		}
	}

	if m.Annotation != nil {
		if !m.Annotation.isMatching(r) {
			return false
		}
	}

	if m.Command != nil {
		cmd := ctx.Value(CommandKey).(ContextCommandVal)
		if cmd != *m.Command {
			return false
		}
	}

	return true
}

type Rule struct {
	Match      *Match               `hcl:"match,block" json:"match,omitempty"`
	Ignore     *Match               `hcl:"ignore,block" json:"ignore,omitempty"`
	Aggregate  []AggregateSettings  `hcl:"aggregate,block" json:"aggregate,omitempty"`
	Annotation []AnnotationSettings `hcl:"annotation,block" json:"annotation,omitempty"`
	Label      []AnnotationSettings `hcl:"label,block" json:"label,omitempty"`
	Cost       *CostSettings        `hcl:"cost,block" json:"cost,omitempty"`
	Alerts     *AlertsSettings      `hcl:"alerts,block" json:"alerts,omitempty"`
	Reject     []RejectSettings     `hcl:"reject,block" json:"reject,omitempty"`
}

func (rule Rule) validate() (err error) {
	if rule.Match != nil {
		if err = rule.Match.validate(true); err != nil {
			return err
		}
	}
	if rule.Ignore != nil {
		if err = rule.Ignore.validate(false); err != nil {
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

	return nil
}

func (rule Rule) resolveChecks(ctx context.Context, path string, r parser.Rule, enabledChecks, disabledChecks []string, prometheusServers []*promapi.Prometheus) []checkMeta {
	enabled := []checkMeta{}

	if rule.Ignore != nil && rule.Ignore.IsMatch(ctx, path, r) {
		return enabled
	}

	if rule.Match != nil && !rule.Match.IsMatch(ctx, path, r) {
		return enabled
	}

	if len(rule.Aggregate) > 0 {
		var nameRegex *regexp.Regexp
		for _, aggr := range rule.Aggregate {
			if aggr.Name != "" {
				nameRegex = strictRegex(aggr.Name)
			}
			severity := aggr.getSeverity(checks.Warning)
			for _, label := range aggr.Keep {
				enabled = append(enabled, checkMeta{
					name:  checks.AggregationCheckName,
					check: checks.NewAggregationCheck(nameRegex, label, true, severity),
				})
			}
			for _, label := range aggr.Strip {
				enabled = append(enabled, checkMeta{
					name:  checks.AggregationCheckName,
					check: checks.NewAggregationCheck(nameRegex, label, false, severity),
				})
			}
		}
	}

	if rule.Cost != nil {
		severity := rule.Cost.getSeverity(checks.Bug)
		for _, prom := range prometheusServers {
			enabled = append(enabled, checkMeta{
				name:  checks.CostCheckName,
				check: checks.NewCostCheck(prom, rule.Cost.BytesPerSample, rule.Cost.MaxSeries, severity),
			})
		}
	}

	if len(rule.Annotation) > 0 {
		for _, ann := range rule.Annotation {
			var valueRegex *regexp.Regexp
			if ann.Value != "" {
				valueRegex = strictRegex(ann.Value)
			}
			severity := ann.getSeverity(checks.Warning)
			enabled = append(enabled, checkMeta{
				name:  checks.AnnotationCheckName,
				check: checks.NewAnnotationCheck(ann.Key, valueRegex, ann.Required, severity),
			})
		}
	}

	if len(rule.Label) > 0 {
		for _, lab := range rule.Label {
			var valueRegex *regexp.Regexp
			if lab.Value != "" {
				valueRegex = strictRegex(lab.Value)
			}
			severity := lab.getSeverity(checks.Warning)
			enabled = append(enabled, checkMeta{
				name:  checks.LabelCheckName,
				check: checks.NewLabelCheck(lab.Key, valueRegex, lab.Required, severity),
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
		for _, prom := range prometheusServers {
			enabled = append(enabled, checkMeta{
				name:  checks.AlertsCheckName,
				check: checks.NewAlertsCheck(prom, qRange, qStep, qResolve),
			})
		}
	}

	if len(rule.Reject) > 0 {
		for _, reject := range rule.Reject {
			severity := reject.getSeverity(checks.Bug)
			re := strictRegex(reject.Regex)
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

	return enabled
}

func isEnabled(enabledChecks, disabledChecks []string, rule parser.Rule, name string, check checks.RuleChecker) bool {
	instance := check.String()
	comments := []string{
		fmt.Sprintf("disable %s", name),
		fmt.Sprintf("disable %s", instance),
	}
	for _, comment := range comments {
		if rule.HasComment(comment) {
			log.Debug().
				Str("check", instance).
				Str("comment", comment).
				Msg("Check disabled by comment")
			return false
		}
	}

	for _, c := range disabledChecks {
		if c == name || c == instance {
			return false
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
