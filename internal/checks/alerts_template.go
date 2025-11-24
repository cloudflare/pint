package checks

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	textTemplate "text/template"
	"text/template/parse"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/timestamp"
	"github.com/prometheus/prometheus/promql"
	promParser "github.com/prometheus/prometheus/promql/parser"
	promTemplate "github.com/prometheus/prometheus/template"

	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/parser/source"
)

const (
	TemplateCheckName             = "alerts/template"
	TemplateCheckSyntaxDetails    = `Supported template syntax is documented [here](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/#templating).`
	TemplateCheckReferenceDetails = "[Click here](https://prometheus.io/docs/prometheus/latest/configuration/template_reference/) for a full list of all available template functions."
)

var (
	templateDefs = []string{
		"{{$labels := .Labels}}",
		"{{$externalLabels := .ExternalLabels}}",
		"{{$externalURL := .ExternalURL}}",
		"{{$value := .Value}}",
	}
	templateDefsString = strings.Join(templateDefs, "")
	templateDefsLen    = len(templateDefsString)

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
		"toDuration":         dummyFuncMap,
		"now":                dummyFuncMap,
		"pathPrefix":         dummyFuncMap,
		"externalURL":        dummyFuncMap,
		"parseDuration":      dummyFuncMap,
	}

	templatePool = sync.Pool{
		New: func() any {
			t := textTemplate.
				New(TemplateCheckName).
				Funcs(templateFuncMap).
				Option("missingkey=zero")
			return t
		},
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

func (c TemplateCheck) Check(ctx context.Context, entry *discovery.Entry, _ []*discovery.Entry) (problems []Problem) {
	if entry.Rule.AlertingRule == nil {
		return nil
	}

	if entry.Rule.AlertingRule.Expr.SyntaxError() != nil {
		return nil
	}

	src := entry.Rule.AlertingRule.Expr.Source()
	data := promTemplate.AlertTemplateData(map[string]string{}, map[string]string{}, "", promql.Sample{})

	for _, label := range entry.Labels().Items {
		if err := checkTemplateSyntax(ctx, label.Key.Value, label.Value.Value, data); err != nil {
			problems = append(problems, Problem{
				Anchor: AnchorAfter,
				Lines: diags.LineRange{
					First: label.Key.Pos.Lines().First,
					Last:  label.Value.Pos.Lines().Last,
				},
				Reporter: c.Reporter(),
				Summary:  "template syntax error",
				Details:  TemplateCheckSyntaxDetails,
				Severity: Fatal,
				Diagnostics: []diags.Diagnostic{
					{
						Message:     fmt.Sprintf("Template failed to parse with this error: `%s`.", err),
						Pos:         label.Value.Pos,
						FirstColumn: 1,
						LastColumn:  len(label.Value.Value),
						Kind:        diags.Issue,
					},
				},
			})
		}
		for _, msg := range checkForValueInLabels(label.Value.Value) {
			problems = append(problems, Problem{
				Anchor: AnchorAfter,
				Lines: diags.LineRange{
					First: label.Key.Pos.Lines().First,
					Last:  label.Value.Pos.Lines().Last,
				},
				Reporter: c.Reporter(),
				Summary:  "value used in labels",
				Details:  "",
				Severity: Bug,
				Diagnostics: []diags.Diagnostic{
					{
						Message:     msg,
						Pos:         label.Value.Pos,
						FirstColumn: 1,
						LastColumn:  len(label.Value.Value),
						Kind:        diags.Issue,
					},
				},
			})
		}

		problems = append(problems, c.checkQueryLabels(entry.Group, entry.Rule, label, src)...)
	}

	if entry.Rule.AlertingRule.Annotations != nil {
		for _, annotation := range entry.Rule.AlertingRule.Annotations.Items {
			if err := checkTemplateSyntax(ctx, annotation.Key.Value, annotation.Value.Value, data); err != nil {
				problems = append(problems, Problem{
					Anchor: AnchorAfter,
					Lines: diags.LineRange{
						First: annotation.Key.Pos.Lines().First,
						Last:  annotation.Value.Pos.Lines().Last,
					},
					Reporter: c.Reporter(),
					Summary:  "template syntax error",
					Details:  TemplateCheckSyntaxDetails,
					Severity: Fatal,
					Diagnostics: []diags.Diagnostic{
						{
							Message:     fmt.Sprintf("Template failed to parse with this error: `%s`.", err),
							Pos:         annotation.Value.Pos,
							FirstColumn: 1,
							LastColumn:  len(annotation.Value.Value),
							Kind:        diags.Issue,
						},
					},
				})
			}
			problems = append(problems, c.checkQueryLabels(entry.Group, entry.Rule, annotation, src)...)
			problems = append(problems, c.checkHumanizeIsNeeded(entry.Rule.AlertingRule.Expr, annotation)...)
		}
	}

	return problems
}

func (c TemplateCheck) checkHumanizeIsNeeded(expr parser.PromQLExpr, ann *parser.YamlKeyValue) (problems []Problem) {
	if !hasValue(ann.Value.Value) {
		return problems
	}
	if hasHumanize(ann.Value.Value) {
		return problems
	}
	vars, aliases, ok := findTemplateVariables(ann.Key.Value, ann.Value.Value)
	if !ok {
		return problems
	}
	for _, src := range expr.Source() {
		call := isRateResult(src)
		if call != nil {
			dgs := []diags.Diagnostic{
				{
					Message:     fmt.Sprintf("`%s()` will produce results that are hard to read for humans.", call.Func.Name),
					Pos:         expr.Value.Pos,
					FirstColumn: int(call.PosRange.Start) + 1,
					LastColumn:  int(call.PosRange.End),
					Kind:        diags.Context,
				},
			}
			labelsAliases := aliases.varAliases(".Value")
			for _, v := range vars {
				for _, a := range labelsAliases {
					if v.value[0] == a {
						dgs = append(dgs, diags.Diagnostic{
							Message:     "Use one of humanize template functions to make the result more readable.",
							Pos:         ann.Value.Pos,
							FirstColumn: v.column,
							LastColumn:  v.column + len(v.value[0]),
							Kind:        diags.Issue,
						})
					}
				}
			}

			problems = append(problems, Problem{
				Anchor: AnchorAfter,
				Lines: diags.LineRange{
					First: min(expr.Value.Pos.Lines().First, ann.Value.Pos.Lines().First),
					Last:  max(expr.Value.Pos.Lines().Last, ann.Value.Pos.Lines().Last),
				},
				Reporter:    c.Reporter(),
				Summary:     "use humanize filters for the results",
				Details:     TemplateCheckReferenceDetails,
				Diagnostics: dgs,
				Severity:    Information,
			})
		}
	}
	return problems
}

func isRateResult(src source.Source) *promParser.Call {
	if src.Type == source.AggregateSource {
		switch src.Operation() {
		case "count", "count_values", "group":
			return nil
		}
	}

	call, ok := source.MostOuterOperation[*promParser.Call](src)
	if !ok {
		return nil
	}

	switch call.Func.Name {
	case "rate", "irate", "deriv":
		return call
	default:
		return nil
	}
}

func queryFunc(_ context.Context, expr string, _ time.Time) (promql.Vector, error) {
	if _, err := promParser.ParseExpr(expr); err != nil {
		return nil, err
	}
	// return a single sample so template using `... | first` don't fail
	return promql.Vector{{}}, nil
}

func normalizeTemplateError(name string, err error) error {
	e := strings.TrimPrefix(err.Error(), "template: "+name+":")
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

func checkTemplateSyntax(ctx context.Context, name, text string, data any) error {
	tmpl := promTemplate.NewTemplateExpander(
		ctx,
		templateDefsString+text,
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

func checkForValueInLabels(text string) (msgs []string) {
	t := templatePool.Get().(*textTemplate.Template)
	defer templatePool.Put(t)

	tt, err := t.Parse(templateDefsString + text)
	if err != nil {
		// no need to double report errors
		return nil
	}
	aliases := aliasesForTemplate(tt)
	for _, node := range tt.Root.Nodes {
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
		for _, v := range vars.value {
			if slices.Contains(valAliases, v) {
				return v, true
			}
		}
	}
	return "", false
}

func hasValue(text string) bool {
	t := templatePool.Get().(*textTemplate.Template)
	defer templatePool.Put(t)

	tt, err := t.Parse(templateDefsString + text)
	if err != nil {
		// no need to double report errors
		return false
	}
	aliases := aliasesForTemplate(tt)
	for _, node := range t.Root.Nodes {
		if _, ok := containsAliasedNode(aliases, node, ".Value"); ok {
			return true
		}
	}
	return false
}

func hasHumanize(text string) bool {
	t := templatePool.Get().(*textTemplate.Template)
	defer templatePool.Put(t)

	tt, err := t.Parse(templateDefsString + text)
	if err != nil {
		// no need to double report errors
		return false
	}
	aliases := aliasesForTemplate(tt)

	for _, node := range tt.Root.Nodes {
		if _, ok := containsAliasedNode(aliases, node, ".Value"); !ok {
			continue
		}
		if n, ok := node.(*parse.ActionNode); ok {
			for _, cmd := range n.Pipe.Cmds {
				for _, arg := range cmd.Args {
					if m, ok := arg.(*parse.IdentifierNode); ok {
						for _, f := range []string{"humanize", "humanize1024", "humanizePercentage", "humanizeDuration", "printf"} {
							if slices.Contains(aliases.varAliases(f), m.Ident) {
								return true
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
								if _, ok := aliases.aliases[k.value[0]]; !ok {
									aliases.aliases[k.value[0]] = map[string]struct{}{}
								}
								aliases.aliases[k.value[0]][v.value[0]] = struct{}{}
							}
						}
					}
				}
			}
		}
	}
}

type tmplVar struct {
	value  []string
	column int
}

func getVariables(node parse.Node) (vars []tmplVar) {
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
		vars = append(vars, tmplVar{
			value:  n.Ident,
			column: int(n.Pos) + 1 - templateDefsLen,
		})
	case *parse.VariableNode:
		vars = append(vars, tmplVar{
			value:  n.Ident,
			column: int(n.Pos) + 1 - templateDefsLen,
		})
	}
	return vars
}

func findTemplateVariables(_, text string) (vars []tmplVar, aliases aliasMap, ok bool) {
	t := templatePool.Get().(*textTemplate.Template)
	defer templatePool.Put(t)

	tt, err := t.Parse(templateDefsString + text)
	if err != nil {
		// no need to double report errors
		return vars, aliases, false
	}

	aliases.aliases = map[string]map[string]struct{}{}
	for _, node := range tt.Root.Nodes {
		getAliases(node, &aliases)
		vars = append(vars, getVariables(node)...)
	}

	return vars, aliases, true
}

func (c TemplateCheck) checkQueryLabels(group *parser.Group, rule parser.Rule, label *parser.YamlKeyValue, src []source.Source) (problems []Problem) {
	vars, aliases, ok := findTemplateVariables(label.Key.Value, label.Value.Value)
	if !ok {
		return nil
	}

	done := map[string]struct{}{}
	labelsAliases := aliases.varAliases(".Labels")
	for _, v := range vars {
		for _, a := range labelsAliases {
			if len(v.value) > 1 && v.value[0] == a {
				if _, ok := done[v.value[1]]; ok {
					continue
				}
				if group != nil && group.Labels != nil && group.Labels.GetValue(v.value[1]) != nil {
					goto NEXT
				}
				for _, s := range src {
					if s.DeadInfo != nil {
						continue
					}
					if !s.CanHaveLabel(v.value[1]) {
						reason, fragment := s.LabelExcludeReason(v.value[1])
						problems = append(problems, Problem{
							Anchor:   AnchorAfter,
							Lines:    rule.Lines,
							Reporter: c.Reporter(),
							Summary:  "template uses non-existent label",
							Details:  "",
							Severity: Bug,
							Diagnostics: []diags.Diagnostic{
								{
									Message:     fmt.Sprintf("Template is using `%s` label but the query results won't have this label.", v.value[1]),
									Pos:         label.Value.Pos,
									FirstColumn: v.column,
									LastColumn:  v.column + len(v.value[1]),
									Kind:        diags.Issue,
								},
								{
									Message:     reason,
									Pos:         rule.AlertingRule.Expr.Value.Pos,
									FirstColumn: int(fragment.Start) + 1,
									LastColumn:  int(fragment.End),
									Kind:        diags.Context,
								},
							},
						})
						goto NEXT
					}
				}
			NEXT:
				done[v.value[1]] = struct{}{}
			}
		}
	}

	return problems
}
