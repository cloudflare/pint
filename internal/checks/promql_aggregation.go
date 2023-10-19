package checks

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"

	promParser "github.com/prometheus/prometheus/promql/parser"
)

const (
	AggregationCheckName = "promql/aggregate"
)

func NewAggregationCheck(nameRegex *TemplatedRegexp, label string, keep bool, severity Severity) AggregationCheck {
	return AggregationCheck{nameRegex: nameRegex, label: label, keep: keep, severity: severity}
}

type AggregationCheck struct {
	nameRegex *TemplatedRegexp
	label     string
	keep      bool
	severity  Severity
}

func (c AggregationCheck) Meta() CheckMeta {
	return CheckMeta{IsOnline: false}
}

func (c AggregationCheck) String() string {
	return fmt.Sprintf("%s(%s:%v)", AggregationCheckName, c.label, c.keep)
}

func (c AggregationCheck) Reporter() string {
	return AggregationCheckName
}

func (c AggregationCheck) Check(_ context.Context, _ string, rule parser.Rule, _ []discovery.Entry) (problems []Problem) {
	expr := rule.Expr()
	if expr.SyntaxError != nil {
		return nil
	}

	if c.nameRegex != nil {
		if rule.RecordingRule != nil && !c.nameRegex.MustExpand(rule).MatchString(rule.RecordingRule.Record.Value.Value) {
			return nil
		}
		if rule.AlertingRule != nil && !c.nameRegex.MustExpand(rule).MatchString(rule.AlertingRule.Alert.Value.Value) {
			return nil
		}
	}

	if rule.RecordingRule != nil && rule.RecordingRule.Labels != nil {
		if val := rule.RecordingRule.Labels.GetValue(c.label); val != nil {
			return nil
		}
	}

	if rule.AlertingRule != nil && rule.AlertingRule.Labels != nil {
		if val := rule.AlertingRule.Labels.GetValue(c.label); val != nil {
			return nil
		}
	}

	for _, problem := range c.checkNode(expr.Query) {
		problems = append(problems, Problem{
			Fragment: problem.expr,
			Lines:    expr.Lines(),
			Reporter: c.Reporter(),
			Text:     problem.text,
			Severity: c.severity,
		})
	}

	return problems
}

func (c AggregationCheck) checkNode(node *parser.PromQLNode) (problems []exprProblem) {
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
			slog.Warn("Unsupported aggregation operation", slog.String("op", n.Op.String()))
		}

		if !n.Without && !c.keep && len(n.Grouping) == 0 {
			// most outer aggregation is stripping a label that we want to get rid of
			// we can skip further checks
			return problems
		}

		var found bool
		for _, g := range n.Grouping {
			if g == c.label {
				found = true
				break
			}
		}

		if n.Without {
			if found && c.keep {
				problems = append(problems, exprProblem{
					expr: node.Expr,
					text: fmt.Sprintf("%s label is required and should be preserved when aggregating %q rules, remove %s from without()", c.label, c.nameRegex.anchored, c.label),
				})
			}

			if !found && !c.keep {
				problems = append(problems, exprProblem{
					expr: node.Expr,
					text: fmt.Sprintf("%s label should be removed when aggregating %q rules, use without(%s, ...)", c.label, c.nameRegex.anchored, c.label),
				})
			}

			// most outer aggregation is stripping a label that we want to get rid of
			// we can skip further checks
			if found && !c.keep {
				return problems
			}
		} else {
			if found && !c.keep {
				problems = append(problems, exprProblem{
					expr: node.Expr,
					text: fmt.Sprintf("%s label should be removed when aggregating %q rules, remove %s from by()", c.label, c.nameRegex.anchored, c.label),
				})
			}

			if !found && c.keep {
				problems = append(problems, exprProblem{
					expr: node.Expr,
					text: fmt.Sprintf("%s label is required and should be preserved when aggregating %q rules, use by(%s, ...)", c.label, c.nameRegex.anchored, c.label),
				})
			}

			// most outer aggregation is stripping a label that we want to get rid of
			// we can skip further checks
			if !found && !c.keep {
				return problems
			}
		}
	}

NEXT:
	if n, ok := node.Node.(*promParser.BinaryExpr); ok && n.VectorMatching != nil {
		switch n.VectorMatching.Card {
		case promParser.CardOneToOne:
			// sum() + sum()
		case promParser.CardManyToOne, promParser.CardManyToMany:
			problems = append(problems, c.checkNode(node.Children[0])...)
			return problems
		case promParser.CardOneToMany:
			problems = append(problems, c.checkNode(node.Children[1])...)
			return problems
		default:
			slog.Warn("Unsupported VectorMatching operation", slog.String("matching", n.VectorMatching.Card.String()))
		}
	}

	for _, child := range node.Children {
		problems = append(problems, c.checkNode(child)...)
	}

	return problems
}
