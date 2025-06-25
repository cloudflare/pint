package checks

import (
	"bytes"
	"regexp"
	"text/template"
	"unique"

	"github.com/cloudflare/pint/internal/parser"
)

var aliases = "{{ $alert := .Alert }}{{ $record := .Record }}{{ $for := .For }}{{ $labels := .Labels }}{{ $annotations := .Annotations }}"

func NewTemplatedRegexp(s string) (unique.Handle[TemplatedRegexp], error) {
	tr := TemplatedRegexp{anchored: "^" + s + "$", original: s}
	_, err := tr.Expand(parser.Rule{})
	return unique.Make(tr), err
}

func NewRawTemplatedRegexp(s string) (unique.Handle[TemplatedRegexp], error) {
	tr := TemplatedRegexp{anchored: s, original: s}
	_, err := tr.Expand(parser.Rule{})
	return unique.Make(tr), err
}

func MustTemplatedRegexp(re string) *TemplatedRegexp {
	tr, _ := NewTemplatedRegexp(re)
	v := tr.Value()
	return &v
}

func MustRawTemplatedRegexp(re string) *TemplatedRegexp {
	tr, _ := NewRawTemplatedRegexp(re)
	v := tr.Value()
	return &v
}

type TemplatedRegexp struct {
	anchored string
	original string
}

func (tr TemplatedRegexp) Expand(rule parser.Rule) (*regexp.Regexp, error) {
	tctx := newTemplateContext(rule)
	tmpl, err := newTemplateFromContext(tr.anchored)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, tctx)
	if err != nil {
		return nil, err
	}

	return regexp.Compile(buf.String())
}

func (tr TemplatedRegexp) MustExpand(rule parser.Rule) *regexp.Regexp {
	re, _ := tr.Expand(rule)
	return re
}

func newTemplateFromContext(content string) (*template.Template, error) {
	tmpl, err := template.New("regexp").Parse(aliases + content)
	if err != nil {
		return nil, err
	}
	tmpl.Option("missingkey=zero")
	return tmpl, nil
}

func newTemplateContext(rule parser.Rule) (c TemplateContext) {
	c.Labels = map[string]string{}
	c.Annotations = map[string]string{}

	if rule.AlertingRule != nil {
		c.Alert = rule.AlertingRule.Alert.Value
		c.Expr = rule.AlertingRule.Expr.Value.Value
		if rule.AlertingRule.For != nil {
			c.For = rule.AlertingRule.For.Value
		}
		if rule.AlertingRule.Labels != nil {
			for _, label := range rule.AlertingRule.Labels.Items {
				c.Labels[label.Key.Value] = label.Value.Value
			}
		}
		if rule.AlertingRule.Annotations != nil {
			for _, ann := range rule.AlertingRule.Annotations.Items {
				c.Labels[ann.Key.Value] = ann.Value.Value
			}
		}
	}
	if rule.RecordingRule != nil {
		c.Record = rule.RecordingRule.Record.Value
		c.Expr = rule.RecordingRule.Expr.Value.Value
		if rule.RecordingRule.Labels != nil {
			for _, label := range rule.RecordingRule.Labels.Items {
				c.Labels[label.Key.Value] = label.Value.Value
			}
		}
	}
	return c
}

type TemplateContext struct {
	Labels      map[string]string
	Annotations map[string]string
	Alert       string
	Record      string
	Expr        string
	For         string
}
