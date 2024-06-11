package checks

import (
	"context"
	"errors"
	"fmt"
	"strings"
	textTemplate "text/template"
	"text/template/parse"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/timestamp"
	"github.com/prometheus/prometheus/promql"
	promParser "github.com/prometheus/prometheus/promql/parser"
	promTemplate "github.com/prometheus/prometheus/template"
	"golang.org/x/exp/slices"

	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/parser/utils"
)

const (
	TemplateCheckName               = "alerts/template"
	TemplateCheckSyntaxDetails      = `Supported template syntax is documented [here](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/#templating).`
	TemplateCheckAggregationDetails = `The query used here is using one of [aggregation functions](https://prometheus.io/docs/prometheus/latest/querying/operators/#aggregation-operators) provided by PromQL.
By default aggregations will remove *all* labels from the results, unless you explicitly specify which labels to remove or keep.
This means that with current query it's impossible for the results to have labels you're trying to use.`
	TemplateCheckAbsentDetails = `The [absent()](https://prometheus.io/docs/prometheus/latest/querying/functions/#absent) function is used to check if provided query doesn't match any time series.
You will only get any results back if the metric selector you pass doesn't match anything.
Since there are no matching time series there are also no labels. If some time series is missing you cannot read its labels.
This means that the only labels you can get back from absent call are the ones you pass to it.
If you're hoping to get instance specific labels this way and alert when some target is down then that won't work, use the ` + "`up`" + ` metric instead.`
	TemplateCheckOnDetails = `Using [vector matching](https://prometheus.io/docs/prometheus/latest/querying/operators/#vector-matching) operations will impact which labels are available on the results of your query.
When using ` + "`on()`" + `make sure that all labels you're trying to use in this templare match what the query can return.`
	TemplateCheckLabelsDetails = `This query doesn't seem to be using any time series and so cannot have any labels.`

	msgAggregation = "Template is using `%s` label but the query removes it."
	msgAbsent      = "Template is using `%s` label but `absent()` is not passing it."
	msgOn          = "Template is using `%s` label but the query uses `on(...)` without it being set there, this label will be missing from the query result."
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
		"pathPrefix":         dummyFuncMap,
		"externalURL":        dummyFuncMap,
		"parseDuration":      dummyFuncMap,
		"toTime":             dummyFuncMap,
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
		IsOnline: false,
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

	aggrs := utils.HasOuterAggregation(rule.AlertingRule.Expr.Query)
	absentCalls := utils.HasOuterAbsent(rule.AlertingRule.Expr.Query)
	binExpr := utils.HasOuterBinaryExpr(rule.AlertingRule.Expr.Query)
	vectors := utils.HasVectorSelector(rule.AlertingRule.Expr.Query)

	var safeLabels []string
	for _, be := range binaryExprs(rule.AlertingRule.Expr.Query) {
		if be.VectorMatching != nil {
			safeLabels = append(safeLabels, be.VectorMatching.MatchingLabels...)
			safeLabels = append(safeLabels, be.VectorMatching.Include...)
		}
	}
	for _, cl := range calls(rule.AlertingRule.Expr.Query, "label_replace") {
		for i, v := range cl.Args {
			if i == 1 {
				if s, ok := v.(*promParser.StringLiteral); ok {
					safeLabels = append(safeLabels, s.Val)
				}
				break
			}
		}
	}

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
					Text:     fmt.Sprintf("Template failed to parse with this error: `%s`.", err),
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
					Text:     msg,
					Severity: Bug,
				})
			}

			for _, aggr := range aggrs {
				for _, msg := range checkMetricLabels(msgAggregation, label.Key.Value, label.Value.Value, aggr.Grouping, aggr.Without, safeLabels) {
					problems = append(problems, Problem{
						Lines: parser.LineRange{
							First: label.Key.Lines.First,
							Last:  label.Value.Lines.Last,
						},
						Reporter: c.Reporter(),
						Text:     msg,
						Details:  TemplateCheckAggregationDetails,
						Severity: Bug,
					})
				}
			}

			for _, call := range absentCalls {
				if len(utils.HasOuterAggregation(call.Fragment)) > 0 {
					continue
				}
				for _, msg := range checkMetricLabels(msgAbsent, label.Key.Value, label.Value.Value, absentLabels(call), false, safeLabels) {
					problems = append(problems, Problem{
						Lines: parser.LineRange{
							First: label.Key.Lines.First,
							Last:  label.Value.Lines.Last,
						},
						Reporter: c.Reporter(),
						Text:     msg,
						Details:  TemplateCheckAbsentDetails,
						Severity: Bug,
					})
				}
			}

			if binExpr != nil && binExpr.VectorMatching != nil && binExpr.VectorMatching.Card == promParser.CardOneToOne && binExpr.VectorMatching.On && len(binExpr.VectorMatching.MatchingLabels) > 0 {
				for _, msg := range checkMetricLabels(msgOn, label.Key.Value, label.Value.Value, binExpr.VectorMatching.MatchingLabels, false, safeLabels) {
					problems = append(problems, Problem{
						Lines: parser.LineRange{
							First: label.Key.Lines.First,
							Last:  label.Value.Lines.Last,
						},
						Reporter: c.Reporter(),
						Text:     msg,
						Details:  TemplateCheckOnDetails,
						Severity: Bug,
					})
				}
			}

			labelNames := getTemplateLabels(label.Key.Value, label.Value.Value)
			if len(labelNames) > 0 && len(vectors) == 0 {
				for _, name := range labelNames {
					problems = append(problems, Problem{
						Lines: parser.LineRange{
							First: label.Key.Lines.First,
							Last:  label.Value.Lines.Last,
						},
						Reporter: c.Reporter(),
						Text:     fmt.Sprintf("Template is using `%s` label but the query doesn't produce any labels.", name),
						Details:  TemplateCheckLabelsDetails,
						Severity: Bug,
					})
				}
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
					Text:     fmt.Sprintf("Template failed to parse with this error: `%s`.", err),
					Details:  TemplateCheckSyntaxDetails,
					Severity: Fatal,
				})
			}

			for _, aggr := range aggrs {
				for _, msg := range checkMetricLabels(msgAggregation, annotation.Key.Value, annotation.Value.Value, aggr.Grouping, aggr.Without, safeLabels) {
					problems = append(problems, Problem{
						Lines: parser.LineRange{
							First: annotation.Key.Lines.First,
							Last:  annotation.Value.Lines.Last,
						},
						Reporter: c.Reporter(),
						Text:     msg,
						Details:  TemplateCheckAggregationDetails,
						Severity: Bug,
					})
				}
			}

			for _, call := range absentCalls {
				if len(utils.HasOuterAggregation(call.Fragment)) > 0 {
					continue
				}
				if call.BinExpr != nil &&
					call.BinExpr.VectorMatching != nil &&
					(call.BinExpr.VectorMatching.Card == promParser.CardManyToOne ||
						call.BinExpr.VectorMatching.Card == promParser.CardOneToMany) &&
					len(call.BinExpr.VectorMatching.Include) == 0 {
					continue
				}
				for _, msg := range checkMetricLabels(msgAbsent, annotation.Key.Value, annotation.Value.Value, absentLabels(call), false, safeLabels) {
					problems = append(problems, Problem{
						Lines: parser.LineRange{
							First: annotation.Key.Lines.First,
							Last:  annotation.Value.Lines.Last,
						},
						Reporter: c.Reporter(),
						Text:     msg,
						Details:  TemplateCheckAbsentDetails,
						Severity: Bug,
					})
				}
			}

			labelNames := getTemplateLabels(annotation.Key.Value, annotation.Value.Value)
			if len(labelNames) > 0 && len(vectors) == 0 {
				for _, name := range labelNames {
					problems = append(problems, Problem{
						Lines: parser.LineRange{
							First: annotation.Key.Lines.First,
							Last:  annotation.Value.Lines.Last,
						},
						Reporter: c.Reporter(),
						Text:     fmt.Sprintf("Template is using `%s` label but the query doesn't produce any labels.", name),
						Details:  TemplateCheckLabelsDetails,
						Severity: Bug,
					})
				}
			}

			if binExpr != nil && binExpr.VectorMatching != nil && binExpr.VectorMatching.Card == promParser.CardOneToOne && binExpr.VectorMatching.On && len(binExpr.VectorMatching.MatchingLabels) > 0 {
				for _, msg := range checkMetricLabels(msgOn, annotation.Key.Value, annotation.Value.Value, binExpr.VectorMatching.MatchingLabels, false, safeLabels) {
					problems = append(problems, Problem{
						Lines: parser.LineRange{
							First: annotation.Key.Lines.First,
							Last:  annotation.Value.Lines.Last,
						},
						Reporter: c.Reporter(),
						Text:     msg,
						Details:  TemplateCheckOnDetails,
						Severity: Bug,
					})
				}
			}

			if hasValue(annotation.Key.Value, annotation.Value.Value) && !hasHumanize(annotation.Key.Value, annotation.Value.Value) {
				for _, problem := range c.checkHumanizeIsNeeded(rule.AlertingRule.Expr.Query) {
					problems = append(problems, Problem{
						Lines: parser.LineRange{
							First: annotation.Key.Lines.First,
							Last:  annotation.Value.Lines.Last,
						},
						Reporter: c.Reporter(),
						Text:     problem.text,
						Severity: problem.severity,
					})
				}
			}
		}
	}

	return problems
}

func (c TemplateCheck) checkHumanizeIsNeeded(node *parser.PromQLNode) (problems []exprProblem) {
	for _, call := range utils.HasOuterRate(node) {
		problems = append(problems, exprProblem{
			expr:     call.String(),
			text:     fmt.Sprintf("Using the value of `%s` inside this annotation might be hard to read, consider using one of humanize template functions to make it more human friendly.", call),
			details:  "[Click here](https://prometheus.io/docs/prometheus/latest/configuration/template_reference/) for a full list of all available template functions.",
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

func getTemplateLabels(name, text string) (names []string) {
	vars, aliases, ok := findTemplateVariables(name, text)
	if !ok {
		return nil
	}

	labelsAliases := aliases.varAliases(".Labels")
	for _, v := range vars {
		for _, a := range labelsAliases {
			if len(v) > 1 && v[0] == a {
				names = append(names, v[1])
			}
		}
	}

	return names
}

func checkMetricLabels(msg, name, text string, metricLabels []string, excludeLabels bool, safeLabels []string) (msgs []string) {
	vars, aliases, ok := findTemplateVariables(name, text)
	if !ok {
		return nil
	}

	done := map[string]struct{}{}
	labelsAliases := aliases.varAliases(".Labels")
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
					if _, ok := done[v[1]]; !ok && !slices.Contains(safeLabels, v[1]) {
						msgs = append(msgs, fmt.Sprintf(msg, v[1]))
						done[v[1]] = struct{}{}
					}
				}
			}
		}
	}

	return msgs
}

func absentLabels(f utils.PromQLFragment) []string {
	labelMap := map[string]struct{}{}

	for _, child := range f.Fragment.Children {
		for _, v := range utils.HasVectorSelector(child) {
			for _, lm := range v.LabelMatchers {
				if lm.Type == labels.MatchEqual {
					labelMap[lm.Name] = struct{}{}
				}
			}
		}
	}

	if f.BinExpr != nil && f.BinExpr.VectorMatching != nil {
		for _, name := range f.BinExpr.VectorMatching.Include {
			labelMap[name] = struct{}{}
		}
	}

	names := make([]string, 0, len(labelMap))
	for name := range labelMap {
		names = append(names, name)
	}

	return names
}

func binaryExprs(node *parser.PromQLNode) (be []*promParser.BinaryExpr) {
	if n, ok := node.Expr.(*promParser.BinaryExpr); ok {
		be = append(be, n)
	}

	for _, child := range node.Children {
		be = append(be, binaryExprs(child)...)
	}

	return be
}

func calls(node *parser.PromQLNode, name string) (cl []*promParser.Call) {
	if n, ok := node.Expr.(*promParser.Call); ok && n.Func.Name == name {
		cl = append(cl, n)
	}

	for _, child := range node.Children {
		cl = append(cl, calls(child, name)...)
	}

	return cl
}
