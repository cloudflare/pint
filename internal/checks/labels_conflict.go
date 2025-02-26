package checks

import (
	"context"
	"errors"
	"fmt"

	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
)

const (
	LabelsConflictCheckName = "labels/conflict"
)

func NewLabelsConflictCheck(prom *promapi.FailoverGroup) LabelsConflictCheck {
	return LabelsConflictCheck{prom: prom}
}

type LabelsConflictCheck struct {
	prom *promapi.FailoverGroup
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
	return fmt.Sprintf("%s(%s)", LabelsConflictCheckName, c.prom.Name())
}

func (c LabelsConflictCheck) Reporter() string {
	return LabelsConflictCheckName
}

func (c LabelsConflictCheck) Check(ctx context.Context, _ discovery.Path, rule parser.Rule, _ []discovery.Entry) (problems []Problem) {
	var labels *parser.YamlMap
	if rule.AlertingRule != nil && rule.AlertingRule.Expr.SyntaxError == nil && rule.AlertingRule.Labels != nil {
		labels = rule.AlertingRule.Labels
	}
	if rule.RecordingRule != nil && rule.RecordingRule.Expr.SyntaxError == nil && rule.RecordingRule.Labels != nil {
		labels = rule.RecordingRule.Labels
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
		text, severity := textAndSeverityFromError(err, c.Reporter(), c.prom.Name(), Warning)
		problems = append(problems, Problem{
			Lines:    labels.Lines,
			Reporter: c.Reporter(),
			Summary:  text,
			Severity: severity,
		})
		return problems
	}

	for _, label := range labels.Items {
		for k, v := range cfg.Config.Global.ExternalLabels {
			if label.Key.Value == k {
				problems = append(problems, Problem{
					Lines: parser.LineRange{
						First: label.Key.Lines.First,
						Last:  label.Value.Lines.Last,
					},
					Reporter: c.Reporter(),
					Summary:  c.formatText(k, label.Value.Value, v, rule.Type(), cfg),
					Details:  fmt.Sprintf("[Click here](%s/config) to see `%s` Prometheus runtime configuration.", cfg.URI, c.prom.Name()),
					Severity: Warning,
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
