package checks

import (
	"fmt"
	"regexp"

	"github.com/cloudflare/pint/internal/parser"

	promParser "github.com/prometheus/prometheus/promql/parser"
)

const (
	ByCheckName = "promql/by"
)

func NewByCheck(nameRegex *regexp.Regexp, label string, keep bool, severity Severity) ByCheck {
	return ByCheck{nameRegex: nameRegex, label: label, keep: keep, severity: severity}
}

type ByCheck struct {
	nameRegex *regexp.Regexp
	label     string
	keep      bool
	severity  Severity
}

func (c ByCheck) String() string {
	return fmt.Sprintf("%s(%s:%v)", ByCheckName, c.label, c.keep)
}

func (c ByCheck) Check(rule parser.Rule) (problems []Problem) {
	expr := rule.Expr()
	if expr.SyntaxError != nil {
		return nil
	}

	if c.nameRegex != nil &&
		rule.RecordingRule != nil &&
		!c.nameRegex.MatchString(rule.RecordingRule.Record.Value.Value) {
		return nil
	}

	if rule.RecordingRule != nil && rule.RecordingRule.Labels != nil {
		if val := rule.RecordingRule.Labels.GetValue(c.label); val != nil {
			return nil
		}
	}

	for _, problem := range c.checkNode(expr.Query) {
		problems = append(problems, Problem{
			Fragment: problem.expr,
			Lines:    expr.Lines(),
			Reporter: ByCheckName,
			Text:     problem.text,
			Severity: c.severity,
		})
	}

	return
}

func (c ByCheck) checkNode(node *parser.PromQLNode) (problems []exprProblem) {
	if n, ok := node.Node.(*promParser.AggregateExpr); ok && !n.Without && n.Op != promParser.TOPK {
		var found bool
		for _, g := range n.Grouping {
			if g == c.label {
				found = true
				break
			}
		}

		if found && !c.keep {
			problems = append(problems, exprProblem{
				expr: node.Expr,
				text: fmt.Sprintf("%s label should be removed when aggregating %q rules, remove %s from by()", c.label, c.nameRegex, c.label),
			})
		}

		if !found && c.keep {
			problems = append(problems, exprProblem{
				expr: node.Expr,
				text: fmt.Sprintf("%s label is required and should be preserved when aggregating %q rules, use by(%s, ...)", c.label, c.nameRegex, c.label),
			})
		}
	}

	for _, child := range node.Children {
		problems = append(problems, c.checkNode(child)...)
	}

	return
}
