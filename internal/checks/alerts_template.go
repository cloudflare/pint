package checks

import (
	"context"
	"errors"
	"fmt"
	"strings"
	textTemplate "text/template"
	"text/template/parse"
	"time"

	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/parser/utils"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/pkg/timestamp"
	promTemplate "github.com/prometheus/prometheus/template"
)

const (
	TemplateCheckName = "alerts/template"
)

var (
	templateDefs = []string{
		"{{$labels := .Labels}}",
		"{{$externalLabels := .ExternalLabels}}",
		"{{$externalURL := .ExternalURL}}",
		"{{$value := .Value}}",
	}

	templateFuncMap = textTemplate.FuncMap{
		"query":              dummyFuncMap,
		"first":              dummyFuncMap,
		"label":              dummyFuncMap,
		"value":              dummyFuncMap,
		"strvalue":           dummyFuncMap,
		"args":               dummyFuncMap,
		"reReplaceAll":       dummyFuncMap,
		"safeHtml":           dummyFuncMap,
		"match":              dummyFuncMap,
		"title":              dummyFuncMap,
		"toUpper":            dummyFuncMap,
		"toLower":            dummyFuncMap,
		"graphLink":          dummyFuncMap,
		"tableLink":          dummyFuncMap,
		"sortByLabel":        dummyFuncMap,
		"humanize":           dummyFuncMap,
		"humanize1024":       dummyFuncMap,
		"humanizeDuration":   dummyFuncMap,
		"humanizePercentage": dummyFuncMap,
		"humanizeTimestamp":  dummyFuncMap,
		"pathPrefix":         dummyFuncMap,
		"externalURL":        dummyFuncMap,
	}
)

func dummyFuncMap(q string) string {
	return q
}

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

	aggr := utils.HasOuterAggregation(rule.AlertingRule.Expr.Query)

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
			// check key
			for _, msg := range checkForValueInLabels(label.Key.Value, label.Key.Value) {
				problems = append(problems, Problem{
					Fragment: fmt.Sprintf("%s: %s", label.Key.Value, label.Value.Value),
					Lines:    label.Lines(),
					Reporter: TemplateCheckName,
					Text:     msg,
					Severity: c.severity,
				})
			}
			// check value
			for _, msg := range checkForValueInLabels(label.Key.Value, label.Value.Value) {
				problems = append(problems, Problem{
					Fragment: fmt.Sprintf("%s: %s", label.Key.Value, label.Value.Value),
					Lines:    label.Lines(),
					Reporter: TemplateCheckName,
					Text:     msg,
					Severity: c.severity,
				})
			}

			if aggr != nil {
				for _, msg := range checkMetricLabels(label.Key.Value, label.Value.Value, aggr.Grouping, aggr.Without) {
					problems = append(problems, Problem{
						Fragment: fmt.Sprintf("%s: %s", label.Key.Value, label.Value.Value),
						Lines:    label.Lines(),
						Reporter: TemplateCheckName,
						Text:     msg,
						Severity: c.severity,
					})
				}
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

			if aggr != nil {
				for _, msg := range checkMetricLabels(annotation.Key.Value, annotation.Value.Value, aggr.Grouping, aggr.Without) {
					problems = append(problems, Problem{
						Fragment: fmt.Sprintf("%s: %s", annotation.Key.Value, annotation.Value.Value),
						Lines:    annotation.Lines(),
						Reporter: TemplateCheckName,
						Text:     msg,
						Severity: c.severity,
					})
				}
			}
		}
	}

	return problems
}

func checkTemplateSyntax(name, text string, data interface{}) error {
	tmpl := promTemplate.NewTemplateExpander(
		context.TODO(),
		strings.Join(append(templateDefs, text), ""),
		name,
		data,
		model.Time(timestamp.FromTime(time.Now())),
		nil,
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

func checkForValueInLabels(name, text string) (msgs []string) {
	t, err := textTemplate.
		New(name).
		Funcs(templateFuncMap).
		Option("missingkey=zero").
		Parse(strings.Join(append(templateDefs, text), ""))
	if err != nil {
		// no need to double report errors
		return nil
	}

	var aliases = aliasMap{aliases: map[string]map[string]struct{}{}}
	var vars = [][]string{}
	for _, node := range t.Root.Nodes {
		getAliases(node, &aliases)
		vars = append(vars, getVariables(node)...)
	}
	var valAliases = aliases.varAliases(".Value")
	for _, v := range vars {
		for _, a := range valAliases {
			if v[0] == a {
				msg := fmt.Sprintf("using %s in labels will generate a new alert on every value change, move it to annotations", v[0])
				msgs = append(msgs, msg)
			}
		}
	}
	return msgs
}

type aliasMap struct {
	aliases map[string]map[string]struct{}
}

func (am aliasMap) varAliases(k string) (vals []string) {
	vals = append(vals, k)
	if as, ok := am.aliases[k]; ok {
		for val := range as {
			vals = append(vals, am.varAliases(val)...)
		}
	}
	return vals
}

func getAliases(node parse.Node, aliases *aliasMap) {
	switch n := node.(type) {
	case *parse.ActionNode:
		if len(n.Pipe.Decl) == 1 && !n.Pipe.IsAssign && len(n.Pipe.Cmds) == 1 {
			for _, cmd := range n.Pipe.Cmds {
				for _, arg := range cmd.Args {
					for _, k := range getVariables(arg) {
						for _, d := range n.Pipe.Decl {
							for _, v := range getVariables(d) {
								if _, ok := aliases.aliases[k[0]]; !ok {
									aliases.aliases[k[0]] = map[string]struct{}{}
								}
								aliases.aliases[k[0]][v[0]] = struct{}{}
							}
						}
					}
				}
			}
		}
	}
}

func getVariables(node parse.Node) (vars [][]string) {
	switch n := node.(type) {
	case *parse.ActionNode:
		if len(n.Pipe.Decl) == 0 && len(n.Pipe.Cmds) > 0 {
			vars = append(vars, getVariables(n.Pipe.Cmds[0])...)
		}
	case *parse.CommandNode:
		for _, arg := range n.Args {
			vars = append(vars, getVariables(arg)...)
		}
	case *parse.FieldNode:
		n.Ident[0] = "." + n.Ident[0]
		vars = append(vars, n.Ident)
	case *parse.VariableNode:
		vars = append(vars, n.Ident)
	}

	return vars
}

func checkMetricLabels(name, text string, metricLabels []string, excludeLabels bool) (msgs []string) {
	t, err := textTemplate.
		New(name).
		Funcs(templateFuncMap).
		Option("missingkey=zero").
		Parse(strings.Join(append(templateDefs, text), ""))
	if err != nil {
		// no need to double report errors
		return nil
	}

	var aliases = aliasMap{aliases: map[string]map[string]struct{}{}}
	var vars = [][]string{}
	for _, node := range t.Root.Nodes {
		getAliases(node, &aliases)
		vars = append(vars, getVariables(node)...)
	}

	var labelsAliases = aliases.varAliases(".Labels")
	for _, v := range vars {
		for _, a := range labelsAliases {
			if len(v) > 1 && v[0] == a {
				var found bool
				for _, l := range metricLabels {
					if len(v) > 1 && v[1] == l {
						found = true
					}
				}
				if found == excludeLabels {
					msg := fmt.Sprintf("template is using %q label but the query doesn't preseve it", v[1])
					msgs = append(msgs, msg)

				}
			}
		}
	}

	return
}
