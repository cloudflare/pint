package utils

import (
	"github.com/cloudflare/pint/internal/parser"

	promParser "github.com/prometheus/prometheus/promql/parser"
	"github.com/rs/zerolog/log"
)

func HasOuterAggregation(node *parser.PromQLNode) (aggs []*promParser.AggregateExpr) {
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
		aggs = append(aggs, n)
		return aggs
	}

NEXT:
	if n, ok := node.Node.(*promParser.BinaryExpr); ok {
		if n.Op.IsComparisonOperator() {
			for _, child := range node.Children {
				return HasOuterAggregation(child)
			}
		} else {
			switch n.Op {
			case promParser.LOR:
				for _, child := range node.Children {
					aggs = append(aggs, HasOuterAggregation(child)...)
				}
				return aggs
			case promParser.DIV, promParser.LUNLESS, promParser.LAND:
				for _, child := range node.Children {
					return HasOuterAggregation(child)
				}
			}
		}
	}

	for _, child := range node.Children {
		aggs = append(aggs, HasOuterAggregation(child)...)
	}

	return aggs
}
