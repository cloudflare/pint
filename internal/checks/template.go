package checks

import (
	"bytes"
	"regexp"
	"text/template"

	"github.com/cloudflare/pint/internal/parser"
)

var aliases = "{{ $alert := .Alert }}{{ $record := .Record }}{{ $for := .For }}{{ $labels := .Labels }}{{ $annotations := .Annotations }}"

func NewTemplatedRegexp(s string) (*TemplatedRegexp, error) {
	tr := TemplatedRegexp{anchored: "^" + s + "$", original: s, static: nil}
	re, expanded, err := tr.expand(parser.Rule{})
	if err != nil {
		return nil, err
	}
	if expanded == tr.anchored {
		tr.static = re
	}
	return &tr, nil
}

func NewRawTemplatedRegexp(s string) (*TemplatedRegexp, error) {
	tr := TemplatedRegexp{anchored: s, original: s, static: nil}
	re, expanded, err := tr.expand(parser.Rule{})
	if err != nil {
		return nil, err
	}
	if expanded == tr.anchored {
		tr.static = re
	}
	return &tr, nil
}

func MustTemplatedRegexp(re string) *TemplatedRegexp {
	tr, _ := NewTemplatedRegexp(re)
	return tr
}

func MustRawTemplatedRegexp(re string) *TemplatedRegexp {
	tr, _ := NewRawTemplatedRegexp(re)
	return tr
}

type TemplatedRegexp struct {
	static   *regexp.Regexp
	anchored string
	original string
}

func (tr TemplatedRegexp) Expand(rule parser.Rule) (*regexp.Regexp, error) {
	if tr.static != nil {
		return tr.static, nil
	}
	re, _, err := tr.expand(rule)
	return re, err
}

func (tr TemplatedRegexp) expand(rule parser.Rule) (*regexp.Regexp, string, error) {
	tctx := newTemplateContext(rule)
	tmpl, err := newTemplateFromContext(tr.anchored)
	if err != nil {
		return nil, "", err
	}

	var buf bytes.Buffer
	if err = tmpl.Execute(&buf, tctx); err != nil {
		return nil, "", err
	}

	s := buf.String()
	re, err := regexp.Compile(s)
	if err != nil {
		return nil, "", err
	}
	return re, s, nil
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
			c.For = rule.AlertingRule.For.Raw
		}
		if rule.AlertingRule.Labels != nil {
			for _, label := range rule.AlertingRule.Labels.Items {
				c.Labels[label.Key.Value] = label.Value.Value
			}
		}
		if rule.AlertingRule.Annotations != nil {
			for _, ann := range rule.AlertingRule.Annotations.Items {
				c.Annotations[ann.Key.Value] = ann.Value.Value
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
