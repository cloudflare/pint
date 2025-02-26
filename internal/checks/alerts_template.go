package checks

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	textTemplate "text/template"
	"text/template/parse"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/timestamp"
	"github.com/prometheus/prometheus/promql"
	promParser "github.com/prometheus/prometheus/promql/parser"
	promTemplate "github.com/prometheus/prometheus/template"

	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/parser/utils"
)

const (
	TemplateCheckName          = "alerts/template"
	TemplateCheckSyntaxDetails = `Supported template syntax is documented [here](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/#templating).`
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
		"stripPort":          dummyFuncMap,
		"stripDomain":        dummyFuncMap,
		"humanize":           dummyFuncMap,
		"humanize1024":       dummyFuncMap,
		"humanizeDuration":   dummyFuncMap,
		"humanizePercentage": dummyFuncMap,
		"humanizeTimestamp":  dummyFuncMap,
		"toTime":             dummyFuncMap,
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

type TemplateCheck struct{}

func (c TemplateCheck) Meta() CheckMeta {
	return CheckMeta{
		States: []discovery.ChangeType{
			discovery.Noop,
			discovery.Added,
			discovery.Modified,
			discovery.Moved,
		},
		Online:        false,
		AlwaysEnabled: false,
	}
}

func (c TemplateCheck) String() string {
	return TemplateCheckName
}

func (c TemplateCheck) Reporter() string {
	return TemplateCheckName
}

func (c TemplateCheck) Check(ctx context.Context, _ discovery.Path, rule parser.Rule, _ []discovery.Entry) (problems []Problem) {
	if rule.AlertingRule == nil {
		return nil
	}

	if rule.AlertingRule.Expr.SyntaxError != nil {
		return nil
	}

	src := utils.LabelsSource(rule.AlertingRule.Expr.Value.Value, rule.AlertingRule.Expr.Query.Expr)
	data := promTemplate.AlertTemplateData(map[string]string{}, map[string]string{}, "", promql.Sample{})

	if rule.AlertingRule.Labels != nil {
		for _, label := range rule.AlertingRule.Labels.Items {
			if err := checkTemplateSyntax(ctx, label.Key.Value, label.Value.Value, data); err != nil {
				problems = append(problems, Problem{
					Lines: parser.LineRange{
						First: label.Key.Lines.First,
						Last:  label.Value.Lines.Last,
					},
					Reporter: c.Reporter(),
					Summary:  fmt.Sprintf("Template failed to parse with this error: `%s`.", err),
					Details:  TemplateCheckSyntaxDetails,
					Severity: Fatal,
				})
			}
			for _, msg := range checkForValueInLabels(label.Key.Value, label.Value.Value) {
				problems = append(problems, Problem{
					Lines: parser.LineRange{
						First: label.Key.Lines.First,
						Last:  label.Value.Lines.Last,
					},
					Reporter: c.Reporter(),
					Summary:  msg,
					Severity: Bug,
				})
			}

			for _, problem := range checkQueryLabels(label.Key.Value, label.Value.Value, src) {
				problems = append(problems, Problem{
					Lines: parser.LineRange{
						First: label.Key.Lines.First,
						Last:  label.Value.Lines.Last,
					},
					Reporter: c.Reporter(),
					Summary:  problem.summary,
					Details:  problem.details,
					Severity: problem.severity,
				})
			}
		}
	}

	if rule.AlertingRule.Annotations != nil {
		for _, annotation := range rule.AlertingRule.Annotations.Items {
			if err := checkTemplateSyntax(ctx, annotation.Key.Value, annotation.Value.Value, data); err != nil {
				problems = append(problems, Problem{
					Lines: parser.LineRange{
						First: annotation.Key.Lines.First,
						Last:  annotation.Value.Lines.Last,
					},
					Reporter: c.Reporter(),
					Summary:  fmt.Sprintf("Template failed to parse with this error: `%s`.", err),
					Details:  TemplateCheckSyntaxDetails,
					Severity: Fatal,
				})
			}

			for _, problem := range checkQueryLabels(annotation.Key.Value, annotation.Value.Value, src) {
				problems = append(problems, Problem{
					Lines: parser.LineRange{
						First: annotation.Key.Lines.First,
						Last:  annotation.Value.Lines.Last,
					},
					Reporter: c.Reporter(),
					Summary:  problem.summary,
					Details:  problem.details,
					Severity: problem.severity,
				})
			}

			if hasValue(annotation.Key.Value, annotation.Value.Value) && !hasHumanize(annotation.Key.Value, annotation.Value.Value) {
				for _, problem := range c.checkHumanizeIsNeeded(rule.AlertingRule.Expr, annotation.Value) {
					problems = append(problems, Problem{
						Lines: parser.LineRange{
							First: min(rule.AlertingRule.Expr.Value.Lines.First, annotation.Value.Lines.First),
							Last:  max(rule.AlertingRule.Expr.Value.Lines.Last, annotation.Value.Lines.Last),
						},
						Reporter:    c.Reporter(),
						Summary:     problem.summary,
						Severity:    problem.severity,
						Diagnostics: problem.diags,
					})
				}
			}
		}
	}

	return problems
}

func (c TemplateCheck) checkHumanizeIsNeeded(expr parser.PromQLExpr, ann *parser.YamlNode) (problems []exprProblem) {
	for _, call := range utils.HasOuterRate(expr.Query) {
		problems = append(problems, exprProblem{
			summary: "use humanize filters for the results",
			details: "[Click here](https://prometheus.io/docs/prometheus/latest/configuration/template_reference/) for a full list of all available template functions.",
			diags: []output.Diagnostic{
				{
					Message:     fmt.Sprintf("`%s()` will produce results that are hard to read for humans.", call.Func.Name),
					Line:        expr.Value.Lines.First,
					FirstColumn: expr.Value.Column + int(call.PosRange.Start),
					LastColumn:  expr.Value.Column + int(call.PosRange.End) - 1,
				},
				{
					Message:     "Use one of humanize template functions to make the result more readable.",
					Line:        ann.Lines.First,
					FirstColumn: ann.Column,
					LastColumn:  nodeLastColumn(ann),
				},
			},
			severity: Information,
		})
	}
	return problems
}

func queryFunc(_ context.Context, expr string, _ time.Time) (promql.Vector, error) {
	if _, err := promParser.ParseExpr(expr); err != nil {
		return nil, err
	}
	// return a single sample so template using `... | first` don't fail
	return promql.Vector{{}}, nil
}

func normalizeTemplateError(name string, err error) error {
	e := strings.TrimPrefix(err.Error(), fmt.Sprintf("template: %s:", name))
	if v := strings.SplitN(e, ":", 2); len(v) > 1 {
		e = strings.TrimPrefix(v[1], " ")
	}
	return errors.New(e)
}

func maybeExpandError(err error) error {
	if e := errors.Unwrap(err); e != nil {
		return e
	}
	return err
}

func checkTemplateSyntax(ctx context.Context, name, text string, data interface{}) error {
	tmpl := promTemplate.NewTemplateExpander(
		ctx,
		strings.Join(append(templateDefs, text), ""),
		name,
		data,
		model.Time(timestamp.FromTime(time.Now())),
		queryFunc,
		nil,
		nil,
	)

	if err := tmpl.ParseTest(); err != nil {
		return normalizeTemplateError(name, maybeExpandError(err))
	}

	_, err := tmpl.Expand()
	if err != nil {
		return normalizeTemplateError(name, maybeExpandError(err))
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
	aliases := aliasesForTemplate(t)
	for _, node := range t.Root.Nodes {
		if v, ok := containsAliasedNode(aliases, node, ".Value"); ok {
			msg := fmt.Sprintf("Using `%s` in labels will generate a new alert on every value change, move it to annotations.", v)
			msgs = append(msgs, msg)
		}
	}
	return msgs
}

func containsAliasedNode(am aliasMap, node parse.Node, alias string) (string, bool) {
	valAliases := am.varAliases(alias)
	for _, vars := range getVariables(node) {
		for _, v := range vars {
			for _, a := range valAliases {
				if v == a {
					return v, true
				}
			}
		}
	}
	return "", false
}

func hasValue(name, text string) bool {
	t, err := textTemplate.
		New(name).
		Funcs(templateFuncMap).
		Option("missingkey=zero").
		Parse(strings.Join(append(templateDefs, text), ""))
	if err != nil {
		// no need to double report errors
		return false
	}
	aliases := aliasesForTemplate(t)
	for _, node := range t.Root.Nodes {
		if _, ok := containsAliasedNode(aliases, node, ".Value"); ok {
			return true
		}
	}
	return false
}

func hasHumanize(name, text string) bool {
	t, err := textTemplate.
		New(name).
		Funcs(templateFuncMap).
		Option("missingkey=zero").
		Parse(strings.Join(append(templateDefs, text), ""))
	if err != nil {
		// no need to double report errors
		return false
	}
	aliases := aliasesForTemplate(t)

	for _, node := range t.Root.Nodes {
		if _, ok := containsAliasedNode(aliases, node, ".Value"); !ok {
			continue
		}
		if n, ok := node.(*parse.ActionNode); ok {
			for _, cmd := range n.Pipe.Cmds {
				for _, arg := range cmd.Args {
					if m, ok := arg.(*parse.IdentifierNode); ok {
						for _, f := range []string{"humanize", "humanize1024", "humanizePercentage", "humanizeDuration", "printf"} {
							for _, a := range aliases.varAliases(f) {
								if m.Ident == a {
									return true
								}
							}
						}
					}
				}
			}
		}
	}

	return false
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

func aliasesForTemplate(t *textTemplate.Template) aliasMap {
	aliases := aliasMap{aliases: map[string]map[string]struct{}{}}
	for _, n := range t.Root.Nodes {
		getAliases(n, &aliases)
	}
	return aliases
}

func getAliases(node parse.Node, aliases *aliasMap) {
	if n, ok := node.(*parse.ActionNode); ok {
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

func findTemplateVariables(name, text string) (vars [][]string, aliases aliasMap, ok bool) {
	t, err := textTemplate.
		New(name).
		Funcs(templateFuncMap).
		Option("missingkey=zero").
		Parse(strings.Join(append(templateDefs, text), ""))
	if err != nil {
		// no need to double report errors
		return vars, aliases, false
	}

	aliases.aliases = map[string]map[string]struct{}{}
	for _, node := range t.Root.Nodes {
		getAliases(node, &aliases)
		vars = append(vars, getVariables(node)...)
	}

	return vars, aliases, true
}

func checkQueryLabels(labelName, labelValue string, src []utils.Source) (problems []exprProblem) {
	vars, aliases, ok := findTemplateVariables(labelName, labelValue)
	if !ok {
		return nil
	}

	done := map[string]struct{}{}
	labelsAliases := aliases.varAliases(".Labels")
	for _, v := range vars {
		for _, a := range labelsAliases {
			if len(v) > 1 && v[0] == a {
				if _, ok := done[v[1]]; ok {
					continue
				}
				for _, s := range src {
					if s.IsDead {
						continue
					}
					if slices.Contains(s.IncludedLabels, v[1]) {
						continue
					}
					if slices.Contains(s.GuaranteedLabels, v[1]) {
						continue
					}
					if s.FixedLabels {
						problems = append(problems, textForProblem(v[1], "", s, Bug))
						goto NEXT
					}
					if slices.Contains(s.ExcludedLabels, v[1]) {
						problems = append(problems, textForProblem(v[1], v[1], s, Bug))
						goto NEXT
					}
				}
			NEXT:
				done[v[1]] = struct{}{}
			}
		}
	}

	return problems
}

func textForProblem(label, reasonLabel string, src utils.Source, severity Severity) exprProblem {
	return exprProblem{
		summary:  fmt.Sprintf("Template is using `%s` label but the query results won't have this label.", label),
		details:  src.ExcludeReason[reasonLabel].Reason,
		severity: severity,
	}
}
