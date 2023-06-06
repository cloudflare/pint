package utils

import (
	"github.com/cloudflare/pint/internal/parser"

	promParser "github.com/prometheus/prometheus/promql/parser"
	"github.com/rs/zerolog/log"
)

type PromQLFragment struct {
	Fragment *parser.PromQLNode
	BinExpr  *promParser.BinaryExpr
}

func HasOuterAbsent(node *parser.PromQLNode) (calls []PromQLFragment) {
	if n, ok := node.Node.(*promParser.Call); ok && n.Func.Name == "absent" {
		calls = append(calls, PromQLFragment{Fragment: node})
		return calls
	}

	if n, ok := node.Node.(*promParser.BinaryExpr); ok {
		if n.VectorMatching != nil {
			switch n.VectorMatching.Card {
			// bar / absent(foo)
			// absent(foo) / bar
			case promParser.CardOneToOne:

			// absent(foo{job="bar"}) * on(job) group_left(xxx) bar
			// bar * on() group_left(xxx) absent(foo{job="bar"})
			case promParser.CardManyToOne:
				if ln, ok := n.LHS.(*promParser.Call); ok && ln.Func.Name == "absent" {
					calls = append(calls, PromQLFragment{
						Fragment: node.Children[0],
						BinExpr:  n,
					})
				}

			// bar * on() group_right(xxx) absent(foo{job="bar"})
			// absent(foo{job="bar"}) * on(job) group_right(xxx) bar
			case promParser.CardOneToMany:
				if rn, ok := n.RHS.(*promParser.Call); ok && rn.Func.Name == "absent" {
					calls = append(calls, PromQLFragment{
						Fragment: node.Children[1],
						BinExpr:  n,
					})
				}

			// bar AND absent(foo{job="bar"})
			// bar OR absent(foo{job="bar"})
			// bar UNLESS absent(foo{job="bar"})
			case promParser.CardManyToMany:
				if n.Op == promParser.LOR {
					for _, child := range node.Children {
						calls = append(calls, HasOuterAbsent(child)...)
					}
				} else {
					if ln, ok := n.LHS.(*promParser.Call); ok && ln.Func.Name == "absent" {
						calls = append(calls, PromQLFragment{
							Fragment: node.Children[0],
							BinExpr:  n,
						})
					}
				}

			default:
				log.Warn().Str("matching", n.VectorMatching.Card.String()).Msg("Unsupported VectorMatching operation")
			}
			return calls
		}
	}

	for _, child := range node.Children {
		calls = append(calls, HasOuterAbsent(child)...)
	}

	return calls
}
