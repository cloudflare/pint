package utils

import (
	promParser "github.com/prometheus/prometheus/promql/parser"

	"github.com/cloudflare/pint/internal/parser"
)

func HasOuterBinaryExpr(node *parser.PromQLNode) *promParser.BinaryExpr {
	if n, ok := node.Expr.(*promParser.BinaryExpr); ok {
		return n
	}

	for _, child := range node.Children {
		if be := HasOuterBinaryExpr(child); be != nil {
			return be
		}
	}

	return nil
}
