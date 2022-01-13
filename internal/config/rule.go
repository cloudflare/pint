package config

import (
	"fmt"
	"regexp"
	"time"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
	"github.com/rs/zerolog/log"
)

var (
	alertingRuleType  = "alerting"
	recordingRuleType = "recording"
)

type MatchLabel struct {
	Key             string `hcl:",label" json:"key"`
	Value           string `hcl:"value" json:"value"`
	annotationCheck bool
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

	if ml.annotationCheck {
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

type Match struct {
	Path       string      `hcl:"path,optional" json:"path,omitempty"`
	Name       string      `hcl:"name,optional" json:"name,omitempty"`
	Kind       string      `hcl:"kind,optional" json:"kind,omitempty"`
	Label      *MatchLabel `hcl:"label,block" json:"label,omitempty"`
	Annotation *MatchLabel `hcl:"annotation,block" json:"annotation,omitempty"`
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
			return nil
		}
	}

	if !allowEmpty && m.Path == "" && m.Name == "" && m.Kind == "" && m.Label == nil && m.Annotation == nil {
		return fmt.Errorf("ignore block must have at least one condition")
	}

	return nil
}

func (m Match) IsMatch(path string, r parser.Rule) bool {
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

func (rule Rule) resolveChecks(path string, r parser.Rule, enabledChecks, disabledChecks []string, prometheusServers []*promapi.Prometheus) []checks.RuleChecker {
	enabled := []checks.RuleChecker{}

	if rule.Ignore != nil && rule.Ignore.IsMatch(path, r) {
		return enabled
	}

	if rule.Match != nil && !rule.Match.IsMatch(path, r) {
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
				if isEnabled(enabledChecks, disabledChecks, checks.AggregationCheckName, r) {
					enabled = append(enabled, checks.NewAggregationCheck(nameRegex, label, true, severity))
				}
			}
			for _, label := range aggr.Strip {
				if isEnabled(enabledChecks, disabledChecks, checks.AggregationCheckName, r) {
					enabled = append(enabled, checks.NewAggregationCheck(nameRegex, label, false, severity))
				}
			}
		}
	}

	if rule.Cost != nil && isEnabled(enabledChecks, disabledChecks, checks.CostCheckName, r) {
		severity := rule.Cost.getSeverity(checks.Bug)
		for _, prom := range prometheusServers {
			enabled = append(enabled, checks.NewCostCheck(prom, rule.Cost.BytesPerSample, rule.Cost.MaxSeries, severity))
		}
	}

	if len(rule.Annotation) > 0 && isEnabled(enabledChecks, disabledChecks, checks.AnnotationCheckName, r) {
		for _, ann := range rule.Annotation {
			var valueRegex *regexp.Regexp
			if ann.Value != "" {
				valueRegex = strictRegex(ann.Value)
			}
			severity := ann.getSeverity(checks.Warning)
			enabled = append(enabled, checks.NewAnnotationCheck(ann.Key, valueRegex, ann.Required, severity))
		}
	}
	if len(rule.Label) > 0 && isEnabled(enabledChecks, disabledChecks, checks.LabelCheckName, r) {
		for _, lab := range rule.Label {
			var valueRegex *regexp.Regexp
			if lab.Value != "" {
				valueRegex = strictRegex(lab.Value)
			}
			severity := lab.getSeverity(checks.Warning)
			enabled = append(enabled, checks.NewLabelCheck(lab.Key, valueRegex, lab.Required, severity))
		}
	}

	if rule.Alerts != nil && isEnabled(enabledChecks, disabledChecks, checks.AlertsCheckName, r) {
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
			enabled = append(enabled, checks.NewAlertsCheck(prom, qRange, qStep, qResolve))
		}
	}

	if len(rule.Reject) > 0 && isEnabled(enabledChecks, disabledChecks, checks.RejectCheckName, r) {
		for _, reject := range rule.Reject {
			severity := reject.getSeverity(checks.Bug)
			if reject.LabelKeys {
				re := strictRegex(reject.Regex)
				enabled = append(enabled, checks.NewRejectCheck(true, false, re, nil, severity))
			}
			if reject.LabelValues {
				re := strictRegex(reject.Regex)
				enabled = append(enabled, checks.NewRejectCheck(true, false, nil, re, severity))
			}
			if reject.AnnotationKeys {
				re := strictRegex(reject.Regex)
				enabled = append(enabled, checks.NewRejectCheck(false, true, re, nil, severity))
			}
			if reject.AnnotationValues {
				re := strictRegex(reject.Regex)
				enabled = append(enabled, checks.NewRejectCheck(false, true, nil, re, severity))
			}
		}
	}

	return enabled
}

func isEnabled(enabledChecks, disabledChecks []string, name string, rule parser.Rule) bool {
	if rule.HasComment(fmt.Sprintf("disable %s", removeRedundantSpaces(name))) {
		log.Debug().
			Str("check", name).
			Msg("Check disabled by comment")
		return false
	}

	for _, c := range disabledChecks {
		if c == name {
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
