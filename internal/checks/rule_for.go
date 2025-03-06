package checks

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/common/model"

	"github.com/cloudflare/pint/internal/diags"
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

func NewRuleForCheck(key RuleForKey, minFor, maxFor time.Duration, comment string, severity Severity) RuleForCheck {
	return RuleForCheck{
		key:      key,
		minFor:   minFor,
		maxFor:   maxFor,
		comment:  comment,
		severity: severity,
	}
}

type RuleForCheck struct {
	key      RuleForKey
	comment  string
	severity Severity
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
		Online:        false,
		AlwaysEnabled: false,
	}
}

func (c RuleForCheck) String() string {
	return fmt.Sprintf("%s(%s:%s)", RuleForCheckName, output.HumanizeDuration(c.minFor), output.HumanizeDuration(c.maxFor))
}

func (c RuleForCheck) Reporter() string {
	return RuleForCheckName
}

func (c RuleForCheck) Check(_ context.Context, _ discovery.Path, rule parser.Rule, _ []discovery.Entry) (problems []Problem) {
	if rule.AlertingRule == nil {
		return nil
	}

	var forDur model.Duration
	var lines diags.LineRange
	var diag diags.Diagnostic

	switch {
	case c.key == RuleForFor && rule.AlertingRule.For != nil:
		forDur, _ = model.ParseDuration(rule.AlertingRule.For.Value)
		lines = rule.AlertingRule.For.Lines
		diag = diags.Diagnostic{
			Message:     "",
			Pos:         rule.AlertingRule.For.Pos,
			FirstColumn: 1,
			LastColumn:  len(rule.AlertingRule.For.Value),
		}
	case c.key == RuleForKeepFiringFor && rule.AlertingRule.KeepFiringFor != nil:
		forDur, _ = model.ParseDuration(rule.AlertingRule.KeepFiringFor.Value)
		lines = rule.AlertingRule.KeepFiringFor.Lines
		diag = diags.Diagnostic{
			Message:     "",
			Pos:         rule.AlertingRule.KeepFiringFor.Pos,
			FirstColumn: 1,
			LastColumn:  len(rule.AlertingRule.KeepFiringFor.Value),
		}
	default:
		lines = rule.AlertingRule.Alert.Lines
		diag = diags.Diagnostic{
			Message:     "",
			Pos:         rule.AlertingRule.Alert.Pos,
			FirstColumn: 1,
			LastColumn:  len(rule.AlertingRule.Alert.Value),
		}
	}

	if time.Duration(forDur) < c.minFor {
		problems = append(problems, Problem{
			Anchor:   AnchorAfter,
			Lines:    lines,
			Reporter: c.Reporter(),
			Summary:  "duration required",
			Details:  maybeComment(c.comment),
			Severity: c.severity,
			Diagnostics: []diags.Diagnostic{
				{
					Message:     fmt.Sprintf("This alert rule must have a `%s` field with a minimum duration of %s.", c.key, output.HumanizeDuration(c.minFor)),
					Pos:         diag.Pos,
					FirstColumn: diag.FirstColumn,
					LastColumn:  diag.LastColumn,
				},
			},
		})
	}

	if c.maxFor > 0 && time.Duration(forDur) > c.maxFor {
		problems = append(problems, Problem{
			Anchor:   AnchorAfter,
			Lines:    lines,
			Reporter: c.Reporter(),
			Summary:  "duration too long",
			Details:  maybeComment(c.comment),
			Severity: c.severity,
			Diagnostics: []diags.Diagnostic{
				{
					Message:     fmt.Sprintf("This alert rule must have a `%s` field with a maximum duration of %s.", c.key, output.HumanizeDuration(c.maxFor)),
					Pos:         diag.Pos,
					FirstColumn: diag.FirstColumn,
					LastColumn:  diag.LastColumn,
				},
			},
		})
	}

	return problems
}
