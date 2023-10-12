package utils

import (
	"log/slog"

	promParser "github.com/prometheus/prometheus/promql/parser"

	"github.com/cloudflare/pint/internal/parser"
)

func HasOuterSum(node *parser.PromQLNode) (calls []*promParser.AggregateExpr) {
	if n, ok := node.Node.(*promParser.AggregateExpr); ok {
		if n.Op == promParser.SUM {
			calls = append(calls, n)
			return calls
		}
	}

	if n, ok := node.Node.(*promParser.AggregateExpr); ok {
		switch n.Op {
		case promParser.COUNT:
			return nil
		case promParser.COUNT_VALUES:
			return nil
		}
	}

	if n, ok := node.Node.(*promParser.BinaryExpr); ok {
		if n.VectorMatching != nil {
			switch n.VectorMatching.Card {
			case promParser.CardOneToOne:
			case promParser.CardOneToMany:
				for i, child := range node.Children {
					if i == len(node.Children)-1 {
						return HasOuterSum(child)
					}
				}
			case promParser.CardManyToOne:
				return HasOuterSum(node.Children[0])
			case promParser.CardManyToMany:
			default:
				slog.Warn("Unsupported VectorMatching operation", slog.String("matching", n.VectorMatching.Card.String()))
			}
		}

		if n.Op.IsComparisonOperator() {
			for i, child := range node.Children {
				if i == 0 {
					return HasOuterSum(child)
				}
			}
		} else {
			switch n.Op {
			case promParser.LOR:
				for _, child := range node.Children {
					calls = append(calls, HasOuterSum(child)...)
				}
				return calls
			case promParser.DIV, promParser.LUNLESS, promParser.LAND:
				for _, child := range node.Children {
					return HasOuterSum(child)
				}
			}
		}
	}

	for _, child := range node.Children {
		calls = append(calls, HasOuterSum(child)...)
	}

	return calls
}
