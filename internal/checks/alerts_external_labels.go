package checks

import (
	"context"
	"errors"
	"fmt"

	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/promapi"
)

const (
	AlertsExternalLabelsCheckName = "alerts/external_labels"
)

func NewAlertsExternalLabelsCheck(prom *promapi.FailoverGroup) AlertsExternalLabelsCheck {
	return AlertsExternalLabelsCheck{
		prom: prom,
	}
}

type AlertsExternalLabelsCheck struct {
	prom *promapi.FailoverGroup
}

func (c AlertsExternalLabelsCheck) Meta() CheckMeta {
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

func (c AlertsExternalLabelsCheck) String() string {
	return fmt.Sprintf("%s(%s)", AlertsExternalLabelsCheckName, c.prom.Name())
}

func (c AlertsExternalLabelsCheck) Reporter() string {
	return AlertsExternalLabelsCheckName
}

func (c AlertsExternalLabelsCheck) Check(ctx context.Context, entry discovery.Entry, _ []discovery.Entry) (problems []Problem) {
	if entry.Rule.AlertingRule == nil {
		return problems
	}

	if entry.Rule.AlertingRule.Expr.SyntaxError != nil {
		return problems
	}

	cfg, err := c.prom.Config(ctx, 0)
	if err != nil {
		if errors.Is(err, promapi.ErrUnsupported) {
			c.prom.DisableCheck(promapi.APIPathConfig, c.Reporter())
			return problems
		}
		problems = append(problems, problemFromError(err, entry.Rule, c.Reporter(), c.prom.Name(), Bug))
		return problems
	}

	for _, label := range entry.Labels().Items {
		for _, name := range checkExternalLabels(label.Key.Value, label.Value.Value, cfg.Config.Global.ExternalLabels) {
			problems = append(problems, Problem{
				Anchor: AnchorAfter,
				Lines: diags.LineRange{
					First: label.Key.Pos.Lines().First,
					Last:  label.Value.Pos.Lines().Last,
				},
				Reporter: c.Reporter(),
				Summary:  "invalid label",
				Details:  fmt.Sprintf("[Click here](%s/config) to see `%s` Prometheus runtime configuration.", cfg.URI, c.prom.Name()),
				Severity: Bug,
				Diagnostics: []diags.Diagnostic{
					{
						Message:     fmt.Sprintf("Template is using `%s` external label but %s doesn't have this label configured in global:external_labels.", name, promText(c.prom.Name(), cfg.URI)),
						Pos:         label.Value.Pos,
						FirstColumn: 1,
						LastColumn:  len(label.Value.Value),
						Kind:        diags.Issue,
					},
				},
			})
		}
	}

	if entry.Rule.AlertingRule.Annotations != nil {
		for _, annotation := range entry.Rule.AlertingRule.Annotations.Items {
			for _, name := range checkExternalLabels(annotation.Key.Value, annotation.Value.Value, cfg.Config.Global.ExternalLabels) {
				problems = append(problems, Problem{
					Anchor: AnchorAfter,
					Lines: diags.LineRange{
						First: annotation.Key.Pos.Lines().First,
						Last:  annotation.Value.Pos.Lines().Last,
					},
					Reporter: c.Reporter(),
					Summary:  "invalid label",
					Details:  fmt.Sprintf("[Click here](%s/config) to see `%s` Prometheus runtime configuration.", cfg.URI, c.prom.Name()),
					Severity: Bug,
					Diagnostics: []diags.Diagnostic{
						{
							Message:     fmt.Sprintf("Template is using `%s` external label but %s doesn't have this label configured in global:external_labels.", name, promText(c.prom.Name(), cfg.URI)),
							Pos:         annotation.Value.Pos,
							FirstColumn: 1,
							LastColumn:  len(annotation.Value.Value),
							Kind:        diags.Issue,
						},
					},
				})
			}
		}
	}

	return problems
}

func checkExternalLabels(name, text string, externalLabels map[string]string) (labels []string) {
	vars, aliases, ok := findTemplateVariables(name, text)
	if !ok {
		return nil
	}

	done := map[string]struct{}{}
	externalLabelsAliases := aliases.varAliases(".ExternalLabels")
	for _, v := range vars {
		for _, a := range externalLabelsAliases {
			if len(v.value) > 1 && v.value[0] == a {
				name := v.value[1]
				if _, ok = done[name]; ok {
					continue
				}
				if _, ok := externalLabels[name]; !ok {
					labels = append(labels, name)
				}
				done[name] = struct{}{}
			}
		}
	}

	return labels
}
