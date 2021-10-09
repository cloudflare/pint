package checks

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cloudflare/pint/internal/parser"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/pkg/timestamp"
	promTemplate "github.com/prometheus/prometheus/template"
)

const (
	TemplateCheckName = "alerts/template"
)

func NewTemplateCheck(severity Severity) TemplateCheck {
	return TemplateCheck{severity: severity}
}

type TemplateCheck struct {
	severity Severity
}

func (c TemplateCheck) String() string {
	return TemplateCheckName
}

func (c TemplateCheck) Check(rule parser.Rule) (problems []Problem) {
	if rule.AlertingRule == nil {
		return nil
	}

	data := promTemplate.AlertTemplateData(map[string]string{}, map[string]string{}, "", 0)

	if rule.AlertingRule.Labels != nil {
		for _, label := range rule.AlertingRule.Labels.Items {
			if err := checkTemplateSyntax(label.Key.Value, label.Value.Value, data); err != nil {
				problems = append(problems, Problem{
					Fragment: fmt.Sprintf("%s: %s", label.Key.Value, label.Value.Value),
					Lines:    label.Lines(),
					Reporter: TemplateCheckName,
					Text:     fmt.Sprintf("template parse error: %s", err),
					Severity: c.severity,
				})
			}
		}
	}

	if rule.AlertingRule.Annotations != nil {
		for _, annotation := range rule.AlertingRule.Annotations.Items {
			if err := checkTemplateSyntax(annotation.Key.Value, annotation.Value.Value, data); err != nil {
				problems = append(problems, Problem{
					Fragment: fmt.Sprintf("%s: %s", annotation.Key.Value, annotation.Value.Value),
					Lines:    annotation.Lines(),
					Reporter: TemplateCheckName,
					Text:     fmt.Sprintf("template parse error: %s", err),
					Severity: c.severity,
				})
			}
		}
	}

	return problems
}

func checkTemplateSyntax(name, text string, data interface{}) error {
	defs := []string{
		"{{$labels := .Labels}}",
		"{{$externalLabels := .ExternalLabels}}",
		"{{$externalURL := .ExternalURL}}",
		"{{$value := .Value}}",
	}
	tmpl := promTemplate.NewTemplateExpander(
		context.TODO(),
		strings.Join(append(defs, text), ""),
		name,
		data,
		model.Time(timestamp.FromTime(time.Now())),
		nil,
		nil,
	)
	if err := tmpl.ParseTest(); err != nil {
		e := strings.TrimPrefix(err.Error(), fmt.Sprintf("template: %s:", name))
		if v := strings.SplitN(e, ":", 2); len(v) > 1 {
			e = strings.TrimPrefix(v[1], " ")
		}
		return errors.New(e)
	}
	return nil
}
