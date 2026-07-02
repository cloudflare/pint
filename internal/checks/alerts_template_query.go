package checks

import (
	"fmt"
	"maps"
	"text/template/parse"

	"github.com/prometheus/prometheus/promql/parser/posrange"

	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/parser/source"
)

// templateQuery is a query string and where it is in the template.
type templateQuery struct {
	expr string
	pos  posrange.PositionRange
}

// templateQueryLabel is a label name and where it is in the template.
type templateQueryLabel struct {
	name string
	pos  posrange.PositionRange
}

// templateQueryUse is a query and the labels read from its results.
type templateQueryUse struct {
	labels []templateQueryLabel
	query  templateQuery
}

// checkTemplateQueries checks queries passed to the query function for syntax
// errors and for labels that the query can't return.
func (c TemplateCheck) checkTemplateQueries(rule parser.Rule, label *parser.YamlKeyValue, meta templateMeta) (problems []Problem) {
	for _, use := range meta.uses {
		query, err := parser.DecodeExpr(use.query.expr)
		if err != nil {
			problems = append(problems, Problem{
				Anchor: AnchorAfter,
				Lines: diags.LineRange{
					First: label.Key.Pos.Lines().First,
					Last:  label.Value.Pos.Lines().Last,
				},
				Reporter: c.Reporter(),
				Summary:  "template query syntax error",
				Details:  TemplateCheckSyntaxDetails,
				Severity: Fatal,
				Diagnostics: []diags.Diagnostic{
					{
						Message:     fmt.Sprintf("Template query failed to parse with this error: `%s`.", err),
						Pos:         label.Value.Pos,
						Expr:        nil,
						FirstColumn: int(use.query.pos.Start) + 1,
						LastColumn:  int(use.query.pos.End),
						Kind:        diags.Issue,
					},
				},
			})
			continue
		}
		problems = append(problems, c.templateQueryLabelProblems(rule, label, use, query)...)
	}

	return problems
}

// templateQueryLabelProblems reports labels read from a query that the query
// can't return.
func (c TemplateCheck) templateQueryLabelProblems(rule parser.Rule, label *parser.YamlKeyValue, use templateQueryUse, query *parser.PromQLNode) (problems []Problem) {
	if len(use.labels) == 0 {
		return nil
	}

	src := source.LabelsSource(use.query.expr, query.Expr)
	done := map[string]struct{}{}
	for _, ql := range use.labels {
		if _, ok := done[ql.name]; ok {
			continue
		}
		done[ql.name] = struct{}{}

		if reason, _, ok := missingLabel(src, ql.name); ok {
			problems = append(problems, c.nonExistentLabelProblem(
				rule,
				diags.Diagnostic{
					Message:     fmt.Sprintf("The template query is using `%s` label but the query results won't have this label.", ql.name),
					Pos:         label.Value.Pos,
					Expr:        nil,
					FirstColumn: int(ql.pos.Start) + 1,
					LastColumn:  int(ql.pos.End),
					Kind:        diags.Issue,
				},
				diags.Diagnostic{
					Message:     reason,
					Pos:         label.Value.Pos,
					Expr:        nil,
					FirstColumn: int(use.query.pos.Start) + 1,
					LastColumn:  int(use.query.pos.End),
					Kind:        diags.Context,
				},
			))
		}
	}
	return problems
}

// queryScope keeps track of which variables hold a query and which query is
// bound to the dot inside a with or range block.
type queryScope struct {
	aliases map[string]templateQuery
	dot     *templateQuery
}

func (s queryScope) child() queryScope {
	aliases := make(map[string]templateQuery, len(s.aliases))
	maps.Copy(aliases, s.aliases)
	return queryScope{aliases: aliases, dot: s.dot}
}

// walkTemplateNode looks for query uses in a node and its children.
func walkTemplateNode(node parse.Node, scope queryScope) (uses []templateQueryUse) {
	switch n := node.(type) {
	case *parse.ListNode:
		for _, child := range n.Nodes {
			uses = append(uses, walkTemplateNode(child, scope)...)
		}
	case *parse.ActionNode:
		uses = append(uses, usesFromPipe(n.Pipe, scope)...)
	case *parse.IfNode:
		uses = append(uses, walkBranch(n.Pipe, n.List, n.ElseList, scope)...)
	case *parse.WithNode:
		uses = append(uses, walkBranch(n.Pipe, n.List, n.ElseList, scope)...)
	case *parse.RangeNode:
		uses = append(uses, walkBranch(n.Pipe, n.List, n.ElseList, scope)...)
	}
	return uses
}

// walkBranch handles if, with and range blocks. If the block runs a query then
// the dot inside it points at that query's results.
func walkBranch(pipe *parse.PipeNode, list, elseList *parse.ListNode, scope queryScope) (uses []templateQueryUse) {
	uses = append(uses, usesFromPipe(pipe, scope)...)

	bodyScope := scope.child()
	if q, ok := queryFromPipe(pipe, scope); ok {
		bodyScope.dot = &q
	}
	if list != nil {
		uses = append(uses, walkTemplateNode(list, bodyScope)...)
	}
	if elseList != nil {
		uses = append(uses, walkTemplateNode(elseList, scope.child())...)
	}
	return uses
}

// usesFromPipe remembers any variable set to a query and returns the query uses
// in the pipeline.
func usesFromPipe(pipe *parse.PipeNode, scope queryScope) (uses []templateQueryUse) {
	if len(pipe.Decl) == 1 && !pipe.IsAssign {
		if q, ok := queryStringFromPipe(pipe, scope); ok {
			scope.aliases[pipe.Decl[0].Ident[0]] = q
		}
	}
	if q, ok := queryFromPipe(pipe, scope); ok {
		uses = append(uses, templateQueryUse{labels: labelsFromPipe(pipe, scope), query: q})
	}
	if scope.dot != nil {
		uses = append(uses, templateQueryUse{labels: labelsFromPipe(pipe, scope), query: *scope.dot})
	}
	uses = append(uses, chainUses(pipe, scope)...)
	return uses
}

// chainUses handles reads like (query "up" | first).Labels.name, where the
// query and the label read are both inside a single chained expression.
func chainUses(pipe *parse.PipeNode, scope queryScope) (uses []templateQueryUse) {
	for _, cmd := range pipe.Cmds {
		for _, arg := range cmd.Args {
			n, ok := arg.(*parse.ChainNode)
			if !ok {
				continue
			}
			// A chain always wraps a parenthesized pipeline, e.g. (query "up").Labels.x.
			inner := n.Node.(*parse.PipeNode)
			q, ok := queryFromPipe(inner, scope)
			if !ok || len(n.Field) != 2 || n.Field[0] != "Labels" {
				continue
			}
			field := "." + n.Field[0] + "." + n.Field[1]
			start := int(n.Pos) - templateDefsLen
			uses = append(uses, templateQueryUse{
				query: q,
				labels: []templateQueryLabel{{
					name: n.Field[1],
					pos: posrange.PositionRange{
						Start: posrange.Pos(start),
						End:   posrange.Pos(start + len(field)),
					},
				}},
			})
		}
	}
	return uses
}

// queryFromPipe returns the query given to query, written either as
// query "up" or "up" | query.
func queryFromPipe(pipe *parse.PipeNode, scope queryScope) (templateQuery, bool) {
	for i, cmd := range pipe.Cmds {
		if !isCommand(cmd, "query") {
			continue
		}
		if len(cmd.Args) > 1 {
			return queryStringFromNode(cmd.Args[1], scope)
		}
		if i > 0 {
			return queryStringFromCommand(pipe.Cmds[i-1], scope)
		}
	}
	return templateQuery{expr: "", pos: posrange.PositionRange{Start: 0, End: 0}}, false
}

func queryStringFromPipe(pipe *parse.PipeNode, scope queryScope) (templateQuery, bool) {
	if len(pipe.Cmds) != 1 {
		return templateQuery{expr: "", pos: posrange.PositionRange{Start: 0, End: 0}}, false
	}
	return queryStringFromCommand(pipe.Cmds[0], scope)
}

// queryStringFromCommand reads the query from a command. It can be a plain
// string, a variable, or a printf call whose format is a string and whose
// arguments are filled with "pint".
func queryStringFromCommand(cmd *parse.CommandNode, scope queryScope) (templateQuery, bool) {
	if len(cmd.Args) == 1 {
		return queryStringFromNode(cmd.Args[0], scope)
	}
	if isCommand(cmd, "printf") && len(cmd.Args) > 1 {
		if n, ok := cmd.Args[1].(*parse.StringNode); ok {
			args := make([]any, len(cmd.Args)-2)
			for i := range args {
				args[i] = "pint"
			}
			return templateQuery{expr: fmt.Sprintf(n.Text, args...), pos: stringNodePos(n)}, true
		}
	}
	return templateQuery{expr: "", pos: posrange.PositionRange{Start: 0, End: 0}}, false
}

// queryStringFromNode reads the query from one node: a string, or a variable
// that was set to a query earlier.
func queryStringFromNode(node parse.Node, scope queryScope) (templateQuery, bool) {
	switch n := node.(type) {
	case *parse.StringNode:
		return templateQuery{expr: n.Text, pos: stringNodePos(n)}, true
	case *parse.VariableNode:
		if len(n.Ident) == 1 {
			q, ok := scope.aliases[n.Ident[0]]
			return q, ok
		}
	}
	return templateQuery{expr: "", pos: posrange.PositionRange{Start: 0, End: 0}}, false
}

// labelsFromPipe finds labels read from a query result, written as
// label "name" or .Labels.name.
func labelsFromPipe(pipe *parse.PipeNode, scope queryScope) (labels []templateQueryLabel) {
	for i, cmd := range pipe.Cmds {
		if isCommand(cmd, "label") && len(cmd.Args) > 1 {
			if n, ok := cmd.Args[1].(*parse.StringNode); ok {
				labels = append(labels, templateQueryLabel{name: n.Text, pos: stringNodePos(n)})
			}
		}
		if scope.dot != nil || (i > 0 && isCommand(pipe.Cmds[i-1], "first")) {
			labels = append(labels, resultLabelFields(cmd)...)
		}
	}
	return labels
}

// resultLabelFields finds .Labels.<name> uses in a command's arguments.
func resultLabelFields(cmd *parse.CommandNode) (labels []templateQueryLabel) {
	const prefix = ".Labels."
	for _, arg := range cmd.Args {
		n, ok := arg.(*parse.FieldNode)
		if !ok || len(n.Ident) != 2 || n.Ident[0] != "Labels" {
			continue
		}
		// n.Pos points at the last name, so step back to the start of ".Labels.".
		start := int(n.Pos) + 1 - len(prefix) - templateDefsLen
		labels = append(labels, templateQueryLabel{
			name: n.Ident[1],
			pos: posrange.PositionRange{
				Start: posrange.Pos(start),
				End:   posrange.Pos(start + len(prefix) + len(n.Ident[1])),
			},
		})
	}
	return labels
}

// stringNodePos returns where a string is in the template, quotes included.
func stringNodePos(n *parse.StringNode) posrange.PositionRange {
	return posrange.PositionRange{
		Start: posrange.Pos(int(n.Pos) - templateDefsLen),
		End:   posrange.Pos(int(n.Pos) + len(n.Quoted) - templateDefsLen),
	}
}

func isCommand(cmd *parse.CommandNode, name string) bool {
	ident, ok := cmd.Args[0].(*parse.IdentifierNode)
	return ok && ident.Ident == name
}
