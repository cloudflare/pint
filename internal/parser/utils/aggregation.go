package utils

import (
	"log/slog"

	"github.com/cloudflare/pint/internal/parser"

	promParser "github.com/prometheus/prometheus/promql/parser"
)

func HasOuterAggregation(node *parser.PromQLNode) (aggs []*promParser.AggregateExpr) {
	if n, ok := node.Expr.(*promParser.AggregateExpr); ok {
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
		aggs = append(aggs, n)
		return aggs
	}

NEXT:
	if n, ok := node.Expr.(*promParser.BinaryExpr); ok {
		if n.VectorMatching != nil {
			switch n.VectorMatching.Card {
			case promParser.CardOneToOne:
			case promParser.CardOneToMany:
				for i, child := range node.Children {
					if i == len(node.Children)-1 {
						a := HasOuterAggregation(child)
						if len(a) > 0 && !a[0].Without {
							a[0].Grouping = append(a[0].Grouping, n.VectorMatching.Include...)
						}
						return a
					}
				}
			case promParser.CardManyToOne:
				a := HasOuterAggregation(node.Children[0])
				if len(a) > 0 && !a[0].Without {
					a[0].Grouping = append(a[0].Grouping, n.VectorMatching.Include...)
				}
				return a
			case promParser.CardManyToMany:
			default:
				slog.Warn("Unsupported VectorMatching operation", slog.String("matching", n.VectorMatching.Card.String()))
			}
		}

		if n.Op.IsComparisonOperator() {
			for i, child := range node.Children {
				if n.VectorMatching != nil {
					a := HasOuterAggregation(child)
					if len(a) > 0 && !a[0].Without {
						a[0].Grouping = append(a[0].Grouping, n.VectorMatching.Include...)
					}
					return a
				}
				if i == 0 {
					return HasOuterAggregation(child)
				}
			}
		} else {
			switch n.Op {
			case promParser.LOR:
				for _, child := range node.Children {
					aggs = append(aggs, HasOuterAggregation(child)...)
				}
				return aggs
			case promParser.LUNLESS, promParser.LAND:
				for _, child := range node.Children {
					return HasOuterAggregation(child)
				}
			case promParser.DIV, promParser.SUB, promParser.ADD:
				if _, ok := n.LHS.(*promParser.NumberLiteral); ok {
					goto CHILDREN
				}
				for _, child := range node.Children {
					return HasOuterAggregation(child)
				}
			}
		}
	}

CHILDREN:
	for _, child := range node.Children {
		aggs = append(aggs, HasOuterAggregation(child)...)
	}

	return aggs
}
