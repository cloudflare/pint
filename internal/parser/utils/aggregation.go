package utils

import (
	"github.com/cloudflare/pint/internal/parser"

	promParser "github.com/prometheus/prometheus/promql/parser"
	"github.com/rs/zerolog/log"
)

func HasOuterAggregation(node *parser.PromQLNode) *promParser.AggregateExpr {
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
		return n
	}

NEXT:
	if n, ok := node.Node.(*promParser.BinaryExpr); ok && n.VectorMatching != nil {
		return HasOuterAggregation(node.Children[0])
	}

	for _, child := range node.Children {
		if a := HasOuterAggregation(child); a != nil {
			return a
		}
	}

	return nil
}
