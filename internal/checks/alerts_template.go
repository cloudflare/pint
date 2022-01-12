package checks

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	textTemplate "text/template"
	"text/template/parse"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/timestamp"
	promTemplate "github.com/prometheus/prometheus/template"

	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/parser/utils"
)

const (
	TemplateCheckName = "alerts/template"

	msgAggregation = "template is using %q label but the query removes it"
	msgAbsent      = "template is using %q label but absent() is not passing it"
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
		"parseDuration":      dummyFuncMap,
	}
)

func dummyFuncMap(q string) string {
	return q
}

func NewTemplateCheck() TemplateCheck {
	return TemplateCheck{}
}

type TemplateCheck struct {
}

func (c TemplateCheck) String() string {
	return TemplateCheckName
}

func (c TemplateCheck) Reporter() string {
	return TemplateCheckName
}

func (c TemplateCheck) Check(ctx context.Context, rule parser.Rule) (problems []Problem) {
	if rule.AlertingRule == nil {
		return nil
	}

	if rule.AlertingRule.Expr.SyntaxError != nil {
		return nil
	}

	aggrs := utils.HasOuterAggregation(rule.AlertingRule.Expr.Query)
	absentCalls := utils.HasOuterAbsent(rule.AlertingRule.Expr.Query)

	data := promTemplate.AlertTemplateData(map[string]string{}, map[string]string{}, "", 0)

	if rule.AlertingRule.Labels != nil {
		for _, label := range rule.AlertingRule.Labels.Items {
			if err := checkTemplateSyntax(label.Key.Value, label.Value.Value, data); err != nil {
				problems = append(problems, Problem{
					Fragment: fmt.Sprintf("%s: %s", label.Key.Value, label.Value.Value),
					Lines:    label.Lines(),
					Reporter: c.Reporter(),
					Text:     fmt.Sprintf("template parse error: %s", err),
					Severity: Fatal,
				})
			}
			// check key
			for _, msg := range checkForValueInLabels(label.Key.Value, label.Key.Value) {
				problems = append(problems, Problem{
					Fragment: fmt.Sprintf("%s: %s", label.Key.Value, label.Value.Value),
					Lines:    label.Lines(),
					Reporter: c.Reporter(),
					Text:     msg,
					Severity: Bug,
				})
			}
			// check value
			for _, msg := range checkForValueInLabels(label.Key.Value, label.Value.Value) {
				problems = append(problems, Problem{
					Fragment: fmt.Sprintf("%s: %s", label.Key.Value, label.Value.Value),
					Lines:    label.Lines(),
					Reporter: c.Reporter(),
					Text:     msg,
					Severity: Bug,
				})
			}

			for _, aggr := range aggrs {
				for _, msg := range checkMetricLabels(msgAggregation, label.Key.Value, label.Value.Value, aggr.Grouping, aggr.Without) {
					problems = append(problems, Problem{
						Fragment: fmt.Sprintf("%s: %s", label.Key.Value, label.Value.Value),
						Lines:    mergeLines(label.Lines(), rule.AlertingRule.Expr.Lines()),
						Reporter: c.Reporter(),
						Text:     msg,
						Severity: Bug,
					})
				}
			}

			for _, call := range absentCalls {
				if len(utils.HasOuterAggregation(call)) > 0 {
					continue
				}
				for _, msg := range checkMetricLabels(msgAbsent, label.Key.Value, label.Value.Value, absentLabels(call), false) {
					problems = append(problems, Problem{
						Fragment: fmt.Sprintf("%s: %s", label.Key.Value, label.Value.Value),
						Lines:    mergeLines(label.Lines(), rule.AlertingRule.Expr.Lines()),
						Reporter: c.Reporter(),
						Text:     msg,
						Severity: Bug,
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
					Reporter: c.Reporter(),
					Text:     fmt.Sprintf("template parse error: %s", err),
					Severity: Fatal,
				})
			}

			for _, aggr := range aggrs {
				for _, msg := range checkMetricLabels(msgAggregation, annotation.Key.Value, annotation.Value.Value, aggr.Grouping, aggr.Without) {
					problems = append(problems, Problem{
						Fragment: fmt.Sprintf("%s: %s", annotation.Key.Value, annotation.Value.Value),
						Lines:    mergeLines(annotation.Lines(), rule.AlertingRule.Expr.Lines()),
						Reporter: c.Reporter(),
						Text:     msg,
						Severity: Bug,
					})
				}
			}

			for _, call := range absentCalls {
				if len(utils.HasOuterAggregation(call)) > 0 {
					continue
				}
				for _, msg := range checkMetricLabels(msgAbsent, annotation.Key.Value, annotation.Value.Value, absentLabels(call), false) {
					problems = append(problems, Problem{
						Fragment: fmt.Sprintf("%s: %s", annotation.Key.Value, annotation.Value.Value),
						Lines:    mergeLines(annotation.Lines(), rule.AlertingRule.Expr.Lines()),
						Reporter: c.Reporter(),
						Text:     msg,
						Severity: Bug,
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

func checkMetricLabels(msg, name, text string, metricLabels []string, excludeLabels bool) (msgs []string) {
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

	done := map[string]struct{}{}
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
					if _, ok := done[v[1]]; !ok {
						msg := fmt.Sprintf(msg, v[1])
						msgs = append(msgs, msg)
						done[v[1]] = struct{}{}
					}
				}
			}
		}
	}

	return
}

func absentLabels(node *parser.PromQLNode) []string {
	labelMap := map[string]struct{}{}

	for _, child := range node.Children {
		for _, v := range utils.HasVectorSelector(child) {
			for _, lm := range v.LabelMatchers {
				if lm.Type == labels.MatchEqual {
					labelMap[lm.Name] = struct{}{}
				}
			}
		}
	}

	names := make([]string, 0, len(labelMap))
	for name := range labelMap {
		names = append(names, name)
	}

	return names
}

func mergeLines(a, b []int) (l []int) {
	l = append(a, b...)
	sort.Ints(l)
	return
}
