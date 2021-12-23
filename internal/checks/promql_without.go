package checks

import (
	"fmt"
	"regexp"

	"github.com/cloudflare/pint/internal/parser"
	"github.com/rs/zerolog/log"

	promParser "github.com/prometheus/prometheus/promql/parser"
)

const (
	WithoutCheckName = "promql/without"
)

func NewWithoutCheck(nameRegex *regexp.Regexp, label string, keep bool, severity Severity) WithoutCheck {
	return WithoutCheck{nameRegex: nameRegex, label: label, keep: keep, severity: severity}
}

type WithoutCheck struct {
	nameRegex *regexp.Regexp
	label     string
	keep      bool
	severity  Severity
}

func (c WithoutCheck) String() string {
	return fmt.Sprintf("%s(%s:%v)", WithoutCheckName, c.label, c.keep)

}

func (c WithoutCheck) Check(rule parser.Rule) (problems []Problem) {
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
			Reporter: WithoutCheckName,
			Text:     problem.text,
			Severity: c.severity,
		})
	}

	return
}

func (c WithoutCheck) checkNode(node *parser.PromQLNode) (problems []exprProblem) {
	if n, ok := node.Node.(*promParser.AggregateExpr); ok {
		switch n.Op {
		case promParser.SUM:
		case promParser.MIN:
		case promParser.MAX:
		case promParser.AVG:
		case promParser.GROUP:
		case promParser.STDDEV:
		case promParser.STDVAR:
		case promParser.COUNT:
		case promParser.COUNT_VALUES:
		case promParser.BOTTOMK:
			goto NEXT
		case promParser.TOPK:
			goto NEXT
		case promParser.QUANTILE:
		default:
			log.Warn().Str("op", n.Op.String()).Msg("Unsupported aggregation operation")
		}

		if !n.Without && len(n.Grouping) == 0 {
			// most outer aggregation is stripping a label that we want to get rid of
			// we can skip further checks
			return
		}

		if !n.Without {
			goto NEXT
		}

		var found bool
		for _, g := range n.Grouping {
			if g == c.label {
				found = true
				break
			}
		}

		if found && c.keep {
			problems = append(problems, exprProblem{
				expr: node.Expr,
				text: fmt.Sprintf("%s label is required and should be preserved when aggregating %q rules, remove %s from without()", c.label, c.nameRegex, c.label),
			})
		}

		if !found && !c.keep {
			problems = append(problems, exprProblem{
				expr: node.Expr,
				text: fmt.Sprintf("%s label should be removed when aggregating %q rules, use without(%s, ...)", c.label, c.nameRegex, c.label),
			})
		}

		// most outer aggregation is stripping a label that we want to get rid of
		// we can skip further checks
		if found && !c.keep {
			return
		}
	}

NEXT:
	if n, ok := node.Node.(*promParser.BinaryExpr); ok && n.VectorMatching != nil {
		switch n.VectorMatching.Card {
		case promParser.CardOneToOne:
			// sum() + sum()
		case promParser.CardManyToOne, promParser.CardManyToMany:
			problems = append(problems, c.checkNode(node.Children[0])...)
			return
		case promParser.CardOneToMany:
			problems = append(problems, c.checkNode(node.Children[1])...)
			return
		default:
			log.Warn().Str("matching", n.VectorMatching.Card.String()).Msg("Unsupported VectorMatching operation")
		}
	}

	for _, child := range node.Children {
		problems = append(problems, c.checkNode(child)...)
	}

	return
}
