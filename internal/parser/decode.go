package parser

import (
	promparser "github.com/prometheus/prometheus/promql/parser"
)

func decodeExpr(expr string) (*PromQLNode, error) {
	node, err := promparser.ParseExpr(expr)
	if err != nil {
		pqe := PromQLError{Err: err}
		pqe.node = &PromQLNode{
			Expr: expr,
			Node: node,
		}
		if perrs, ok := err.(promparser.ParseErrors); ok {
			for _, perr := range perrs {
				pqe.Err = perr.Err
				pqe.node.Expr = perr.Query
			}
		}
		return nil, pqe
	}

	pn := PromQLNode{
		Expr: expr,
		Node: node,
	}

	for _, child := range promparser.Children(node) {
		c, err := decodeExpr(child.String())
		if err != nil {
			return nil, err
		}
		pn.Children = append(pn.Children, c)
	}

	return &pn, nil
}
