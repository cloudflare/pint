package utils

import (
	promParser "github.com/prometheus/prometheus/promql/parser"
)

func RemoveConditions(source string) promParser.Node {
	node, _ := promParser.ParseExpr(source)
	switch n := node.(type) {
	case *promParser.AggregateExpr:
		n.Expr = RemoveConditions(n.Expr.String()).(promParser.Expr)
		return n
	case *promParser.BinaryExpr:
		lhs := RemoveConditions(n.LHS.String())
		rhs := RemoveConditions(n.RHS.String())
		_, ln := lhs.(*promParser.NumberLiteral)
		if v, ok := lhs.(*promParser.VectorSelector); ok && v.Name == "" {
			ln = true
		}
		_, rn := rhs.(*promParser.NumberLiteral)
		if v, ok := rhs.(*promParser.VectorSelector); ok && v.Name == "" {
			rn = true
		}
		if ln && rn {
			return &promParser.VectorSelector{}
		}
		if ln {
			return rhs
		}
		if rn {
			return lhs
		}
		n.LHS = lhs.(promParser.Expr)
		n.RHS = rhs.(promParser.Expr)
		return n
	case *promParser.Call:
		ret := promParser.Expressions{}
		for _, e := range n.Args {
			ret = append(ret, RemoveConditions(e.String()).(promParser.Expr))
		}
		n.Args = ret
		return n
	case *promParser.SubqueryExpr:
		n.Expr = RemoveConditions(n.Expr.String()).(promParser.Expr)
		return n
	case *promParser.ParenExpr:
		n.Expr = RemoveConditions(n.Expr.String()).(promParser.Expr)
		switch n.Expr.(type) {
		case *promParser.NumberLiteral, *promParser.StringLiteral, *promParser.VectorSelector, *promParser.MatrixSelector:
			return n.Expr
		}
		return n
	case *promParser.UnaryExpr:
		n.Expr = RemoveConditions(n.Expr.String()).(promParser.Expr)
		return n
	case *promParser.StepInvariantExpr:
		n.Expr = RemoveConditions(n.Expr.String()).(promParser.Expr)
		return n
	case *promParser.NumberLiteral, *promParser.StringLiteral, *promParser.VectorSelector, *promParser.MatrixSelector:
		return node
	default:
		return node
	}
}
