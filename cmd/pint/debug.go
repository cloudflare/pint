package main

import (
	"fmt"
	"strings"

	"github.com/prometheus/prometheus/promql/parser"
	"github.com/urfave/cli/v2"
)

const levelStep = 2

func print(ident int, format string, a ...interface{}) {
	prefix := strings.Repeat(" ", ident)
	fmt.Printf(prefix+format+"\n", a...)
}

func parseNode(node parser.Node, level int) {
	print(level, "++ node: %v", node)
	level += levelStep

	switch n := node.(type) {
	case parser.Expressions:
		print(level, "Expressions:")
		for _, e := range n {
			parseNode(e, level+levelStep)
		}
	case *parser.AggregateExpr:
		print(level, "AggregateExpr:")
		level += levelStep
		print(level, "* Type: %v", n.Type())
		print(level, "* Op: %v", n.Op)
		print(level, "* Expr: %v", n.Expr)
		print(level, "* Param: %v", n.Param)
		print(level, "* Grouping: %v", n.Grouping)
		print(level, "* Without: %v", n.Without)
		parseNode(n.Expr, level+levelStep)
	case *parser.BinaryExpr:
		print(level, "BinaryExpr:")
		level += levelStep
		print(level, "* Type: %v", n.Type())
		print(level, "* Op: %v", n.Op)
		print(level, "* LHS: %v", n.LHS)
		print(level, "* RHS: %v", n.RHS)
		print(level, "* VectorMatching: %v", n.VectorMatching)
		print(level, "* ReturnBool: %v", n.ReturnBool)
	case *parser.EvalStmt:
		print(level, "EvalStmt:")
		level += levelStep
		print(level, "* Expr: %v", n.Expr)
		print(level, "* Start: %v", n.Start)
		print(level, "* End: %v", n.End)
		print(level, "* Interval: %v", n.Interval)
		parseNode(n.Expr, level+levelStep)
	case *parser.Call:
		print(level, "Call:")
		level += levelStep
		print(level, "* Type: %v", n.Type())
		print(level, "* Func: %v", n.Func.Name)
		print(level, "* Args: %v", n.Args)
		parseNode(n.Args, level+levelStep)
	case *parser.ParenExpr:
		print(level, "ParenExpr:")
		level += levelStep
		print(level, "* Type: %v", n.Type())
		print(level, "* Expr: %v", n.Expr)
		parseNode(n.Expr, level+levelStep)
	case *parser.UnaryExpr:
		print(level, "UnaryExpr:")
		print(level, "* Type: %v", n.Type())
		print(level, "* Op: %v", n.Op)
		print(level, "* Expr: %v", n.Expr)
		parseNode(n.Expr, level+levelStep)
	case *parser.SubqueryExpr:
		print(level, "SubqueryExpr:")
		level += levelStep
		print(level, "* Type: %v", n.Type())
		print(level, "* Expr: %v", n.Expr)
		print(level, "* Step: %v", n.Step)
		print(level, "* Range: %v", n.Range)
		print(level, "* Offset: %v", n.Offset)
		parseNode(n.Expr, level+levelStep)
	case *parser.MatrixSelector:
		print(level, "MatrixSelector:")
		level += levelStep
		print(level, "* Type: %v", n.Type())
		print(level, "* VectorSelector: %v", n.VectorSelector)
		print(level, "* Range: %v", n.Range)
	case *parser.VectorSelector:
		print(level, "VectorSelector:")
		level += levelStep
		print(level, "* Type: %v", n.Type())
		print(level, "* Name: %v", n.Name)
		print(level, "* Offset: %v", n.Offset)
		print(level, "* LabelMatchers: %v", n.LabelMatchers)
	case *parser.NumberLiteral:
		print(level, "NumberLiteral:")
		level += levelStep
		print(level, "* Type: %v", n.Type())
	case *parser.StringLiteral:
		print(level, "StringLiteral:")
		level += levelStep
		print(level, "* Type: %v", n.Type())
	default:
		print(level, "! Unsupported node")
	}
}

func parseQuery(query string) error {
	expr, err := parser.ParseExpr(query)
	if err != nil {
		return err
	}

	parseNode(expr, 0)
	for _, c := range parser.Children(expr) {
		parseNode(c, levelStep)
	}
	return nil
}

func actionParse(c *cli.Context) (err error) {
	err = initLogger(c.String(logLevelFlag))
	if err != nil {
		return fmt.Errorf("failed to set log level: %s", err)
	}

	parts := c.Args().Slice()
	if len(parts) == 0 {
		return fmt.Errorf("a query string is required")
	}
	query := strings.Join(parts, " ")
	return parseQuery(query)
}
