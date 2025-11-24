package checks

import (
	"context"
	"errors"
	"fmt"

	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
)

const (
	LabelsConflictCheckName = "labels/conflict"
)

func NewLabelsConflictCheck(prom *promapi.FailoverGroup) LabelsConflictCheck {
	return LabelsConflictCheck{
		prom:     prom,
		instance: fmt.Sprintf("%s(%s)", LabelsConflictCheckName, prom.Name()),
	}
}

type LabelsConflictCheck struct {
	prom     *promapi.FailoverGroup
	instance string
}

func (c LabelsConflictCheck) Meta() CheckMeta {
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

func (c LabelsConflictCheck) String() string {
	return c.instance
}

func (c LabelsConflictCheck) Reporter() string {
	return LabelsConflictCheckName
}

func (c LabelsConflictCheck) Check(ctx context.Context, entry *discovery.Entry, _ []*discovery.Entry) (problems []Problem) {
	var labels *parser.YamlMap
	if entry.Rule.AlertingRule != nil && entry.Rule.AlertingRule.Expr.SyntaxError() == nil && entry.Rule.AlertingRule.Labels != nil {
		labels = entry.Rule.AlertingRule.Labels
	}
	if entry.Rule.RecordingRule != nil && entry.Rule.RecordingRule.Expr.SyntaxError() == nil && entry.Rule.RecordingRule.Labels != nil {
		labels = entry.Rule.RecordingRule.Labels
	}
	if labels == nil {
		return problems
	}

	cfg, err := c.prom.Config(ctx, 0)
	if err != nil {
		if errors.Is(err, promapi.ErrUnsupported) {
			c.prom.DisableCheck(promapi.APIPathConfig, c.Reporter())
			return problems
		}
		problems = append(problems, problemFromError(err, entry.Rule, c.Reporter(), c.prom.Name(), Warning))
		return problems
	}

	for _, label := range labels.Items {
		for k, v := range cfg.Config.Global.ExternalLabels {
			if label.Key.Value == k {
				problems = append(problems, Problem{
					Anchor: AnchorAfter,
					Lines: diags.LineRange{
						First: label.Key.Pos.Lines().First,
						Last:  label.Value.Pos.Lines().Last,
					},
					Reporter: c.Reporter(),
					Summary:  "conflicting labels",
					Details:  fmt.Sprintf("[Click here](%s/config) to see `%s` Prometheus runtime configuration.", cfg.URI, c.prom.Name()),
					Severity: Warning,
					Diagnostics: []diags.Diagnostic{
						{
							Message:     c.formatText(k, label.Value.Value, v, entry.Rule.Type(), cfg),
							Pos:         label.Key.Pos,
							FirstColumn: 1,
							LastColumn:  len(label.Key.Value),
							Kind:        diags.Issue,
						},
					},
				})
			}
		}
	}

	return problems
}

func (c LabelsConflictCheck) formatText(k, lv, ev string, kind parser.RuleType, cfg *promapi.ConfigResult) string {
	if (lv == ev) && kind == parser.AlertingRuleType {
		return fmt.Sprintf("This label is redundant. %s external_labels already has %s=%q label set and it will be automatically added to all alerts, there's no need to set it manually.", promText(c.prom.Name(), cfg.URI), k, ev)
	}
	return fmt.Sprintf("%s external_labels already has %s=%q label set, please choose a different name for this label to avoid any conflicts.", promText(c.prom.Name(), cfg.URI), k, ev)
}
