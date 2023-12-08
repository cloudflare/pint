package checks

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/common/model"

	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/parser"
)

const (
	RuleForCheckName = "rule/for"
)

type RuleForKey string

const (
	RuleForFor           RuleForKey = "for"
	RuleForKeepFiringFor RuleForKey = "keep_firing_for"
)

func NewRuleForCheck(key RuleForKey, minFor, maxFor time.Duration, severity Severity) RuleForCheck {
	return RuleForCheck{
		key:      key,
		minFor:   minFor,
		maxFor:   maxFor,
		severity: severity,
	}
}

type RuleForCheck struct {
	severity Severity
	key      RuleForKey
	minFor   time.Duration
	maxFor   time.Duration
}

func (c RuleForCheck) Meta() CheckMeta {
	return CheckMeta{
		States: []discovery.ChangeType{
			discovery.Noop,
			discovery.Added,
			discovery.Modified,
			discovery.Moved,
		},
		IsOnline: true,
	}
}

func (c RuleForCheck) String() string {
	return fmt.Sprintf("%s(%s:%s)", RuleForCheckName, output.HumanizeDuration(c.minFor), output.HumanizeDuration(c.maxFor))
}

func (c RuleForCheck) Reporter() string {
	return RuleForCheckName
}

func (c RuleForCheck) Check(_ context.Context, _ string, rule parser.Rule, _ []discovery.Entry) (problems []Problem) {
	if rule.AlertingRule == nil {
		return nil
	}

	var forDur model.Duration
	var fragment string
	var lines []int

	switch c.key {
	case RuleForFor:
		if rule.AlertingRule.For != nil {
			forDur, _ = model.ParseDuration(rule.AlertingRule.For.Value.Value)
			fragment = rule.AlertingRule.For.Value.Value
			lines = rule.AlertingRule.For.Lines()
		}
	case RuleForKeepFiringFor:
		if rule.AlertingRule.KeepFiringFor != nil {
			forDur, _ = model.ParseDuration(rule.AlertingRule.KeepFiringFor.Value.Value)
			fragment = rule.AlertingRule.KeepFiringFor.Value.Value
			lines = rule.AlertingRule.KeepFiringFor.Lines()
		}
	}

	if fragment == "" {
		fragment = rule.AlertingRule.Alert.Value.Value
		lines = rule.AlertingRule.Alert.Lines()
	}

	if time.Duration(forDur) < c.minFor {
		problems = append(problems, Problem{
			Lines:    lines,
			Reporter: c.Reporter(),
			Text:     fmt.Sprintf("This alert rule must have a `%s` field with a minimum duration of %s.", c.key, output.HumanizeDuration(c.minFor)),
			Severity: c.severity,
		})
	}

	if c.maxFor > 0 && time.Duration(forDur) > c.maxFor {
		problems = append(problems, Problem{
			Lines:    lines,
			Reporter: c.Reporter(),
			Text:     fmt.Sprintf("This alert rule must have a `%s` field with a maximum duration of %s.", c.key, output.HumanizeDuration(c.maxFor)),
			Severity: c.severity,
		})
	}

	return problems
}
