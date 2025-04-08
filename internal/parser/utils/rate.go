package utils

import (
	"log/slog"

	promParser "github.com/prometheus/prometheus/promql/parser"

	"github.com/cloudflare/pint/internal/parser"
)

func HasOuterRate(node *parser.PromQLNode) (calls []*promParser.Call) {
	if n, ok := node.Expr.(*promParser.Call); ok {
		switch n.Func.Name {
		case "rate", "irate", "deriv":
			calls = append(calls, n)
			return calls
		case "ceil", "floor", "round":
			return nil
		}
	}

	if n, ok := node.Expr.(*promParser.AggregateExpr); ok {
		switch n.Op {
		case promParser.COUNT:
			return nil
		case promParser.COUNT_VALUES:
			return nil
		}
	}

	if n, ok := node.Expr.(*promParser.BinaryExpr); ok {
		if n.VectorMatching != nil {
			switch n.VectorMatching.Card {
			case promParser.CardOneToOne:
			case promParser.CardOneToMany:
				for i, child := range node.Children {
					if i == len(node.Children)-1 {
						return HasOuterRate(child)
					}
				}
			case promParser.CardManyToOne:
				return HasOuterRate(node.Children[0])
			case promParser.CardManyToMany:
			default:
				slog.Warn(
					"Unsupported VectorMatching operation",
					slog.String("matching", n.VectorMatching.Card.String()),
				)
			}
		}

		if n.Op.IsComparisonOperator() {
			for i, child := range node.Children {
				if i == 0 {
					return HasOuterRate(child)
				}
			}
		} else {
			switch n.Op {
			case promParser.LOR:
				for _, child := range node.Children {
					calls = append(calls, HasOuterRate(child)...)
				}
				return calls
			case promParser.DIV, promParser.LUNLESS, promParser.LAND:
				for _, child := range node.Children {
					return HasOuterRate(child)
				}
			}
		}
	}

	for _, child := range node.Children {
		calls = append(calls, HasOuterRate(child)...)
	}

	return calls
}
