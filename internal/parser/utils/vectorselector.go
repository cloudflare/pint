package utils

import (
	"github.com/cloudflare/pint/internal/parser"

	promParser "github.com/prometheus/prometheus/promql/parser"
)

func HasVectorSelector(node *parser.PromQLNode) (vs []*promParser.VectorSelector) {
	if n, ok := node.Expr.(*promParser.VectorSelector); ok {
		vs = append(vs, n)
	}

	for _, child := range node.Children {
		vs = append(vs, HasVectorSelector(child)...)
	}

	return vs
}
