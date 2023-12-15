package checks

import (
	"context"
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
		IsOnline: true,
	}
}

func (c LabelsConflictCheck) String() string {
	return fmt.Sprintf("%s(%s)", LabelsConflictCheckName, c.prom.Name())
}

func (c LabelsConflictCheck) Reporter() string {
	return LabelsConflictCheckName
}

func (c LabelsConflictCheck) Check(ctx context.Context, _ string, rule parser.Rule, _ []discovery.Entry) (problems []Problem) {
	if rule.RecordingRule == nil || rule.RecordingRule.Expr.SyntaxError != nil {
		return nil
	}

	if rule.RecordingRule.Labels == nil {
		return nil
	}

	cfg, err := c.prom.Config(ctx)
	if err != nil {
		text, severity := textAndSeverityFromError(err, c.Reporter(), c.prom.Name(), Warning)
		problems = append(problems, Problem{
			Lines:    rule.RecordingRule.Labels.Lines.Expand(),
			Reporter: c.Reporter(),
			Text:     text,
			Severity: severity,
		})
		return problems
	}

	for _, label := range rule.RecordingRule.Labels.Items {
		for k, v := range cfg.Config.Global.ExternalLabels {
			if label.Key.Value == k {
				problems = append(problems, Problem{
					Lines:    label.Lines(),
					Reporter: c.Reporter(),
					Text:     fmt.Sprintf("%s external_labels already has %s=%q label set, please choose a different name for this label to avoid any conflicts.", promText(c.prom.Name(), cfg.URI), k, v),
					Details:  fmt.Sprintf("[Click here](%s/config) to see `%s` Prometheus runtime configuration.", cfg.PublicURI, c.prom.Name()),
					Severity: Warning,
				})
			}
		}
	}

	return problems
}
