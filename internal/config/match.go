package config

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
)

const (
	AlertingRuleType  = "alerting"
	RecordingRuleType = "recording"
	InvalidRuleType   = "invalid"

	StateAny        = "any"
	StateAdded      = "added"
	StateModified   = "modified"
	StateRenamed    = "renamed"
	StateRemoved    = "removed"
	StateUnmodified = "unmodified"
)

var (
	CommandKey   ContextCommandKey = "command"
	CICommand    ContextCommandVal = "ci"
	LintCommand  ContextCommandVal = "lint"
	WatchCommand ContextCommandVal = "watch"

	CIStates  = []string{StateAdded, StateModified, StateRenamed, StateRemoved}
	AnyStates = []string{StateAny}
)

type (
	ContextCommandKey string
	ContextCommandVal string
)

type Match struct {
	Label      *MatchLabel        `hcl:"label,block" json:"label,omitempty"`
	Annotation *MatchAnnotation   `hcl:"annotation,block" json:"annotation,omitempty"`
	Command    *ContextCommandVal `hcl:"command,optional" json:"command,omitempty"`

	pathRe                     *regexp.Regexp
	nameRe                     *regexp.Regexp
	forDurationMatch           *durationMatch
	keepFiringForDurationMatch *durationMatch

	Path          string   `hcl:"path,optional" json:"path,omitempty"`
	Name          string   `hcl:"name,optional" json:"name,omitempty"`
	Kind          string   `hcl:"kind,optional" json:"kind,omitempty"`
	For           string   `hcl:"for,optional" json:"for,omitempty"`
	KeepFiringFor string   `hcl:"keep_firing_for,optional" json:"keep_firing_for,omitempty"`
	State         []string `hcl:"state,optional" json:"state,omitempty"`
}

func (m *Match) Validate(allowEmpty bool) (err error) {
	if m.Path != "" {
		m.pathRe, err = regexp.Compile("^" + m.Path + "$")
		if err != nil {
			return err
		}
	}

	if m.Name != "" {
		m.nameRe, err = regexp.Compile("^" + m.Name + "$")
		if err != nil {
			return err
		}
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
		if err := m.Label.Validate(); err != nil {
			return err
		}
	}

	if m.Annotation != nil {
		if err := m.Annotation.Validate(); err != nil {
			return err
		}
	}

	if m.For != "" {
		dm, err := parseDurationMatch(m.For)
		if err != nil {
			return err
		}
		m.forDurationMatch = &dm
	}

	if m.KeepFiringFor != "" {
		dm, err := parseDurationMatch(m.KeepFiringFor)
		if err != nil {
			return err
		}
		m.keepFiringForDurationMatch = &dm
	}

	for _, s := range m.State {
		switch s {
		case StateAny, StateAdded, StateModified, StateRenamed, StateRemoved, StateUnmodified:
			// valid values
		default:
			return fmt.Errorf("unknown rule state: %s", s)
		}
	}

	if !allowEmpty &&
		m.Path == "" &&
		m.Name == "" &&
		m.Kind == "" &&
		m.Label == nil &&
		m.Annotation == nil &&
		m.Command == nil &&
		m.For == "" &&
		m.KeepFiringFor == "" &&
		m.State == nil {
		return errors.New("ignore block must have at least one condition")
	}

	return nil
}

func (m Match) IsMatch(ctx context.Context, path string, e *discovery.Entry) bool {
	cmd := commandFromContext(ctx)

	if m.Command != nil {
		if cmd != *m.Command {
			return false
		}
	}

	if len(m.State) != 0 && !stateMatches(m.State, e.State) {
		return false
	}

	if m.Kind != "" {
		if e.Rule.AlertingRule != nil && m.Kind != AlertingRuleType {
			return false
		}
		if e.Rule.RecordingRule != nil && m.Kind != RecordingRuleType {
			return false
		}
	}

	if m.pathRe != nil {
		if !m.pathRe.MatchString(path) {
			return false
		}
	}

	if m.nameRe != nil {
		if e.Rule.AlertingRule != nil && !m.nameRe.MatchString(e.Rule.AlertingRule.Alert.Value) {
			return false
		}
		if e.Rule.RecordingRule != nil && !m.nameRe.MatchString(e.Rule.RecordingRule.Record.Value) {
			return false
		}
	}

	if m.Label != nil {
		if !m.Label.isMatching(e) {
			return false
		}
	}

	if m.Annotation != nil {
		if !m.Annotation.isMatching(e.Rule) {
			return false
		}
	}

	if m.forDurationMatch != nil {
		if e.Rule.AlertingRule != nil && e.Rule.AlertingRule.For != nil && e.Rule.AlertingRule.For.ParseError == nil {
			if !m.forDurationMatch.isMatch(e.Rule.AlertingRule.For.Value) {
				return false
			}
		} else {
			return false
		}
	}

	if m.keepFiringForDurationMatch != nil {
		if e.Rule.AlertingRule != nil && e.Rule.AlertingRule.KeepFiringFor != nil && e.Rule.AlertingRule.KeepFiringFor.ParseError == nil {
			if !m.keepFiringForDurationMatch.isMatch(e.Rule.AlertingRule.KeepFiringFor.Value) {
				return false
			}
		} else {
			return false
		}
	}

	return true
}

type MatchLabel struct {
	keyRe *regexp.Regexp
	valRe *regexp.Regexp
	Key   string `hcl:",label" json:"key"`
	Value string `hcl:"value" json:"value"`
}

func (ml *MatchLabel) Validate() (err error) {
	ml.keyRe, err = regexp.Compile("^" + ml.Key + "$")
	if err != nil {
		return err
	}

	ml.valRe, err = regexp.Compile("^" + ml.Value + "$")
	if err != nil {
		return err
	}

	return nil
}

func (ml MatchLabel) isMatching(entry *discovery.Entry) bool {
	for _, label := range entry.Labels().Items {
		if ml.keyRe.MatchString(label.Key.Value) && ml.valRe.MatchString(label.Value.Value) {
			return true
		}
	}

	return false
}

type MatchAnnotation struct {
	keyRe *regexp.Regexp
	valRe *regexp.Regexp
	Key   string `hcl:",label" json:"key"`
	Value string `hcl:"value" json:"value"`
}

func (ma *MatchAnnotation) Validate() (err error) {
	ma.keyRe, err = regexp.Compile("^" + ma.Key + "$")
	if err != nil {
		return err
	}

	ma.valRe, err = regexp.Compile("^" + ma.Value + "$")
	if err != nil {
		return err
	}

	return nil
}

func (ma MatchAnnotation) isMatching(rule parser.Rule) bool {
	if rule.AlertingRule == nil || rule.AlertingRule.Annotations == nil {
		return false
	}
	for _, ann := range rule.AlertingRule.Annotations.Items {
		if ma.keyRe.MatchString(ann.Key.Value) && ma.valRe.MatchString(ann.Value.Value) {
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
	default:
		return false
	}
}

func stateMatches(states []string, state discovery.ChangeType) bool {
	for _, s := range states {
		switch s {
		case StateAny:
			return true
		case StateAdded:
			if state == discovery.Added {
				return true
			}
		case StateModified:
			if state == discovery.Modified {
				return true
			}
		case StateRenamed:
			if state == discovery.Moved {
				return true
			}
		case StateRemoved:
			if state == discovery.Removed {
				return true
			}
		case StateUnmodified:
			if state == discovery.Noop {
				return true
			}
		}
	}
	return false
}
