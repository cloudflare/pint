package config

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/cloudflare/pint/internal/parser"
)

const (
	AlertingRuleType  = "alerting"
	RecordingRuleType = "recording"
	InvalidRuleType   = "invalid"
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

type Match struct {
	Label         *MatchLabel        `hcl:"label,block" json:"label,omitempty"`
	Annotation    *MatchAnnotation   `hcl:"annotation,block" json:"annotation,omitempty"`
	Command       *ContextCommandVal `hcl:"command,optional" json:"command,omitempty"`
	Path          string             `hcl:"path,optional" json:"path,omitempty"`
	Name          string             `hcl:"name,optional" json:"name,omitempty"`
	Kind          string             `hcl:"kind,optional" json:"kind,omitempty"`
	For           string             `hcl:"for,optional" json:"for,omitempty"`
	KeepFiringFor string             `hcl:"keep_firing_for,optional" json:"keep_firing_for,omitempty"`
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
	case AlertingRuleType, RecordingRuleType:
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

	if m.For != "" {
		if _, err := parseDurationMatch(m.For); err != nil {
			return err
		}
	}

	if !allowEmpty && m.Path == "" && m.Name == "" && m.Kind == "" && m.Label == nil && m.Annotation == nil && m.Command == nil && m.For == "" {
		return fmt.Errorf("ignore block must have at least one condition")
	}

	return nil
}

func (m Match) IsMatch(ctx context.Context, path string, r parser.Rule) bool {
	if m.Kind != "" {
		if r.AlertingRule != nil && m.Kind != AlertingRuleType {
			return false
		}
		if r.RecordingRule != nil && m.Kind != RecordingRuleType {
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
		if r.AlertingRule != nil && !re.MatchString(r.AlertingRule.Alert.Value) {
			return false
		}
		if r.RecordingRule != nil && !re.MatchString(r.RecordingRule.Record.Value) {
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

	if m.For != "" {
		if r.AlertingRule != nil && r.AlertingRule.For != nil {
			dm, _ := parseDurationMatch(m.For)
			if dur, err := parseDuration(r.AlertingRule.For.Value); err == nil {
				if !dm.isMatch(dur) {
					return false
				}
			}
		} else {
			return false
		}
	}

	if m.KeepFiringFor != "" {
		if r.AlertingRule != nil && r.AlertingRule.KeepFiringFor != nil {
			dm, _ := parseDurationMatch(m.KeepFiringFor)
			if dur, err := parseDuration(r.AlertingRule.KeepFiringFor.Value); err == nil {
				if !dm.isMatch(dur) {
					return false
				}
			}
		} else {
			return false
		}
	}

	return true
}

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

type matchOperation string

const (
	opSeparator                = " "
	opLess      matchOperation = "<"
	opLessEqual matchOperation = "<="
	opEqual     matchOperation = "="
	opNotEqual  matchOperation = "!="
	opMoreEqual matchOperation = ">="
	opMore      matchOperation = ">"
)

func parseMatchOperation(expr string) (matchOperation, error) {
	switch expr {
	case string(opLess):
		return opLess, nil
	case string(opLessEqual):
		return opLessEqual, nil
	case string(opEqual):
		return opEqual, nil
	case string(opNotEqual):
		return opNotEqual, nil
	case string(opMoreEqual):
		return opMoreEqual, nil
	case string(opMore):
		return opMore, nil
	default:
		return opEqual, fmt.Errorf("unknown duration match operation: %s", expr)
	}
}

func parseDurationMatch(expr string) (dm durationMatch, err error) {
	parts := strings.SplitN(expr, opSeparator, 2)
	if len(parts) == 2 {
		if dm.op, err = parseMatchOperation(parts[0]); err != nil {
			return dm, err
		}
		dm.dur, err = parseDuration(parts[1])
	} else {
		dm.op = opEqual
		dm.dur, err = parseDuration(expr)
	}

	return dm, err
}

type durationMatch struct {
	op  matchOperation
	dur time.Duration
}

func (dm durationMatch) isMatch(dur time.Duration) bool {
	switch dm.op {
	case opLess:
		return dur < dm.dur
	case opLessEqual:
		return dur <= dm.dur
	case opEqual:
		return dur == dm.dur
	case opNotEqual:
		return dur != dm.dur
	case opMoreEqual:
		return dur >= dm.dur
	case opMore:
		return dur > dm.dur
	}
	return false
}
