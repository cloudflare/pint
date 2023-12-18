package checks

import (
	"context"
	"fmt"

	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
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
		IsOnline: true,
	}
}

func (c AlertsExternalLabelsCheck) String() string {
	return fmt.Sprintf("%s(%s)", AlertsExternalLabelsCheckName, c.prom.Name())
}

func (c AlertsExternalLabelsCheck) Reporter() string {
	return AlertsExternalLabelsCheckName
}

func (c AlertsExternalLabelsCheck) Check(ctx context.Context, _ string, rule parser.Rule, _ []discovery.Entry) (problems []Problem) {
	if rule.AlertingRule == nil {
		return problems
	}

	if rule.AlertingRule.Expr.SyntaxError != nil {
		return problems
	}

	cfg, err := c.prom.Config(ctx)
	if err != nil {
		text, severity := textAndSeverityFromError(err, c.Reporter(), c.prom.Name(), Bug)
		problems = append(problems, Problem{
			Lines:    rule.Lines,
			Reporter: c.Reporter(),
			Text:     text,
			Severity: severity,
		})
		return problems
	}

	if rule.AlertingRule.Labels != nil {
		for _, label := range rule.AlertingRule.Labels.Items {
			for _, name := range checkExternalLabels(label.Key.Value, label.Key.Value, cfg.Config.Global.ExternalLabels) {
				problems = append(problems, Problem{
					Lines: parser.LineRange{
						First: label.Key.Lines.First,
						Last:  label.Value.Lines.Last,
					},
					Reporter: c.Reporter(),
					Text:     fmt.Sprintf("Template is using `%s` external label but %s doesn't have this label configured in global:external_labels.", name, promText(c.prom.Name(), cfg.URI)),
					Details:  fmt.Sprintf("[Click here](%s/config) to see `%s` Prometheus runtime configuration.", cfg.PublicURI, c.prom.Name()),
					Severity: Bug,
				})
			}
			for _, name := range checkExternalLabels(label.Key.Value, label.Value.Value, cfg.Config.Global.ExternalLabels) {
				problems = append(problems, Problem{
					Lines: parser.LineRange{
						First: label.Key.Lines.First,
						Last:  label.Value.Lines.Last,
					},
					Reporter: c.Reporter(),
					Text:     fmt.Sprintf("Template is using `%s` external label but %s doesn't have this label configured in global:external_labels.", name, promText(c.prom.Name(), cfg.URI)), Severity: Bug,
					Details: fmt.Sprintf("[Click here](%s/config) to see `%s` Prometheus runtime configuration.", cfg.PublicURI, c.prom.Name()),
				})
			}
		}
	}

	if rule.AlertingRule.Annotations != nil {
		for _, annotation := range rule.AlertingRule.Annotations.Items {
			for _, name := range checkExternalLabels(annotation.Key.Value, annotation.Key.Value, cfg.Config.Global.ExternalLabels) {
				problems = append(problems, Problem{
					Lines: parser.LineRange{
						First: annotation.Key.Lines.First,
						Last:  annotation.Value.Lines.Last,
					},
					Reporter: c.Reporter(),
					Text:     fmt.Sprintf("Template is using `%s` external label but %s doesn't have this label configured in global:external_labels.", name, promText(c.prom.Name(), cfg.URI)),
					Details:  fmt.Sprintf("[Click here](%s/config) to see `%s` Prometheus runtime configuration.", cfg.PublicURI, c.prom.Name()),
					Severity: Bug,
				})
			}
			for _, name := range checkExternalLabels(annotation.Key.Value, annotation.Value.Value, cfg.Config.Global.ExternalLabels) {
				problems = append(problems, Problem{
					Lines: parser.LineRange{
						First: annotation.Key.Lines.First,
						Last:  annotation.Value.Lines.Last,
					},
					Reporter: c.Reporter(),
					Text:     fmt.Sprintf("Template is using `%s` external label but %s doesn't have this label configured in global:external_labels.", name, promText(c.prom.Name(), cfg.URI)),
					Details:  fmt.Sprintf("[Click here](%s/config) to see `%s` Prometheus runtime configuration.", cfg.PublicURI, c.prom.Name()),
					Severity: Bug,
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
			if len(v) > 1 && v[0] == a {
				name := v[1]
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
