package utils

import (
	"github.com/cloudflare/pint/internal/parser"

	promParser "github.com/prometheus/prometheus/promql/parser"
	"github.com/rs/zerolog/log"
)

func HasOuterAbsent(node *parser.PromQLNode) (calls []*parser.PromQLNode) {
	if n, ok := node.Node.(*promParser.Call); ok && n.Func.Name == "absent" {
		calls = append(calls, node)
		return calls
	}

	if n, ok := node.Node.(*promParser.BinaryExpr); ok {
		if n.VectorMatching != nil {
			switch n.VectorMatching.Card {
			case promParser.CardOneToOne:
			case promParser.CardOneToMany:
				for i, child := range node.Children {
					if i == len(node.Children)-1 {
						return HasOuterAbsent(child)
					}
				}
			case promParser.CardManyToOne:
				return HasOuterAbsent(node.Children[0])
			case promParser.CardManyToMany:
			default:
				log.Warn().Str("matching", n.VectorMatching.Card.String()).Msg("Unsupported VectorMatching operation")
			}
		}

		if n.Op.IsComparisonOperator() {
			for i, child := range node.Children {
				if n.VectorMatching != nil {
					return HasOuterAbsent(child)
				}
				if i == 0 {
					return HasOuterAbsent(child)
				}
			}
		} else {
			switch n.Op {
			case promParser.LOR:
				for _, child := range node.Children {
					calls = append(calls, HasOuterAbsent(child)...)
				}
				return calls
			case promParser.DIV, promParser.LUNLESS, promParser.LAND:
				for _, child := range node.Children {
					return HasOuterAbsent(child)
				}
			}
		}
	}

	for _, child := range node.Children {
		calls = append(calls, HasOuterAbsent(child)...)
	}

	return calls
}
