package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	promParser "github.com/prometheus/prometheus/promql/parser"
	"github.com/urfave/cli/v3"

	"github.com/cloudflare/pint/internal/parser"
)

const levelStep = 2

var parseCmd = &cli.Command{
	Name:   "parse",
	Usage:  "Parse a query and print AST, use it for debugging or understanding query details.",
	Action: actionParse,
}

func printNode(ident int, format string, a ...any) {
	prefix := strings.Repeat(" ", ident)
	fmt.Printf(prefix+format+"\n", a...)
}

func parseNode(node promParser.Node, level int) {
	printNode(level, "++ node: %v", node)
	level += levelStep

	switch n := node.(type) {
	case promParser.Expressions:
		printNode(level, "Expressions:")
		for _, e := range n {
			parseNode(e, level+levelStep)
		}
	case *promParser.AggregateExpr:
		printNode(level, "AggregateExpr:")
		level += levelStep
		printNode(level, "* Type: %v", n.Type())
		printNode(level, "* Op: %v", n.Op)
		printNode(level, "* Expr: %v", n.Expr)
		printNode(level, "* Param: %v", n.Param)
		printNode(level, "* Grouping: %v", n.Grouping)
		printNode(level, "* Without: %v", n.Without)
		parseNode(n.Expr, level+levelStep)
	case *promParser.BinaryExpr:
		printNode(level, "BinaryExpr:")
		level += levelStep
		printNode(level, "* Type: %v", n.Type())
		printNode(level, "* Op: %v", n.Op)
		printNode(level, "* LHS: %v", n.LHS)
		printNode(level, "* RHS: %v", n.RHS)
		printNode(level, "* VectorMatching:")
		if n.VectorMatching != nil {
			printNode(level+levelStep, "* Card: %v", n.VectorMatching.Card)
			printNode(level+levelStep, "* MatchingLabels: %v", n.VectorMatching.MatchingLabels)
			printNode(level+levelStep, "* On: %v", n.VectorMatching.On)
			printNode(level+levelStep, "* Include: %v", n.VectorMatching.Include)
			if n.VectorMatching.FillValues.LHS != nil {
				printNode(level+levelStep, "* FillLHS: %v", *n.VectorMatching.FillValues.LHS)
			}
			if n.VectorMatching.FillValues.RHS != nil {
				printNode(level+levelStep, "* FillRHS: %v", *n.VectorMatching.FillValues.RHS)
			}
		}
		printNode(level, "* ReturnBool: %v", n.ReturnBool)
		parseNode(n.LHS, level+levelStep)
		parseNode(n.RHS, level+levelStep)
	case *promParser.Call:
		printNode(level, "Call:")
		level += levelStep
		printNode(level, "* Type: %v", n.Type())
		printNode(level, "* Func: %v", n.Func.Name)
		printNode(level, "* Args: %v", n.Args)
		parseNode(n.Args, level+levelStep)
	case *promParser.ParenExpr:
		printNode(level, "ParenExpr:")
		level += levelStep
		printNode(level, "* Type: %v", n.Type())
		printNode(level, "* Expr: %v", n.Expr)
		parseNode(n.Expr, level+levelStep)
	case *promParser.SubqueryExpr:
		printNode(level, "SubqueryExpr:")
		level += levelStep
		printNode(level, "* Type: %v", n.Type())
		printNode(level, "* Expr: %v", n.Expr)
		printNode(level, "* Step: %v", n.Step)
		if n.StepExpr != nil {
			printNode(level, "* StepExpr: %v", n.StepExpr)
		}
		printNode(level, "* Range: %v", n.Range)
		if n.RangeExpr != nil {
			printNode(level, "* RangeExpr: %v", n.RangeExpr)
		}
		printNode(level, "* Offset: %v", n.Offset)
		if n.OriginalOffsetExpr != nil {
			printNode(level, "* OriginalOffsetExpr: %v", n.OriginalOffsetExpr)
		}
		parseNode(n.Expr, level+levelStep)
	case *promParser.MatrixSelector:
		printNode(level, "MatrixSelector:")
		level += levelStep
		printNode(level, "* Type: %v", n.Type())
		printNode(level, "* VectorSelector: %v", n.VectorSelector)
		printNode(level, "* Range: %v", n.Range)
		if n.RangeExpr != nil {
			printNode(level, "* RangeExpr: %v", n.RangeExpr)
		}
	case *promParser.VectorSelector:
		printNode(level, "VectorSelector:")
		level += levelStep
		printNode(level, "* Type: %v", n.Type())
		printNode(level, "* Name: %v", n.Name)
		printNode(level, "* Offset: %v", n.Offset)
		if n.OriginalOffsetExpr != nil {
			printNode(level, "* OriginalOffsetExpr: %v", n.OriginalOffsetExpr)
		}
		printNode(level, "* LabelMatchers: %v", n.LabelMatchers)
		if n.Anchored {
			printNode(level, "* Anchored: true")
		}
		if n.Smoothed {
			printNode(level, "* Smoothed: true")
		}
	case *promParser.NumberLiteral:
		printNode(level, "NumberLiteral:")
		level += levelStep
		printNode(level, "* Type: %v", n.Type())
	case *promParser.StringLiteral:
		printNode(level, "StringLiteral:")
		level += levelStep
		printNode(level, "* Type: %v", n.Type())
	case *promParser.UnaryExpr:
		printNode(level, "UnaryExpr:")
		level += levelStep
		printNode(level, "* Type: %v", n.Type())
		printNode(level, "* Op: %v", n.Op)
		printNode(level, "* Expr: %v", n.Expr)
	default:
		printNode(level, "! Unsupported node")
	}
}

func parseQuery(query string) error {
	expr, err := parser.PromQLParser.ParseExpr(query)
	if err != nil {
		return err
	}
	parseNode(expr, 0)
	return nil
}

func actionParse(_ context.Context, c *cli.Command) (err error) {
	err = initLogger(c.String(logLevelFlag), c.Bool(noColorFlag))
	if err != nil {
		return fmt.Errorf("failed to set log level: %w", err)
	}

	parts := c.Args().Slice()
	if len(parts) == 0 {
		return errors.New("a query string is required")
	}

	query := strings.Join(parts, " ")
	return parseQuery(query)
}
