package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
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

func printNode(w io.Writer, ident int, format string, a ...any) {
	prefix := strings.Repeat(" ", ident)
	_, _ = fmt.Fprintf(w, prefix+format+"\n", a...)
}

func parseNode(w io.Writer, node promParser.Node, level int) {
	printNode(w, level, "++ node: %v", node)
	level += levelStep

	switch n := node.(type) {
	case promParser.Expressions:
		printNode(w, level, "Expressions:")
		for _, e := range n {
			parseNode(w, e, level+levelStep)
		}
	case *promParser.AggregateExpr:
		printNode(w, level, "AggregateExpr:")
		level += levelStep
		printNode(w, level, "* Type: %v", n.Type())
		printNode(w, level, "* Op: %v", n.Op)
		printNode(w, level, "* Expr: %v", n.Expr)
		printNode(w, level, "* Param: %v", n.Param)
		printNode(w, level, "* Grouping: %v", n.Grouping)
		printNode(w, level, "* Without: %v", n.Without)
		parseNode(w, n.Expr, level+levelStep)
	case *promParser.BinaryExpr:
		printNode(w, level, "BinaryExpr:")
		level += levelStep
		printNode(w, level, "* Type: %v", n.Type())
		printNode(w, level, "* Op: %v", n.Op)
		printNode(w, level, "* LHS: %v", n.LHS)
		printNode(w, level, "* RHS: %v", n.RHS)
		printNode(w, level, "* VectorMatching:")
		if n.VectorMatching != nil {
			printNode(w, level+levelStep, "* Card: %v", n.VectorMatching.Card)
			printNode(w, level+levelStep, "* MatchingLabels: %v", n.VectorMatching.MatchingLabels)
			printNode(w, level+levelStep, "* On: %v", n.VectorMatching.On)
			printNode(w, level+levelStep, "* Include: %v", n.VectorMatching.Include)
			if n.VectorMatching.FillValues.LHS != nil {
				printNode(w, level+levelStep, "* FillLHS: %v", *n.VectorMatching.FillValues.LHS)
			}
			if n.VectorMatching.FillValues.RHS != nil {
				printNode(w, level+levelStep, "* FillRHS: %v", *n.VectorMatching.FillValues.RHS)
			}
		}
		printNode(w, level, "* ReturnBool: %v", n.ReturnBool)
		parseNode(w, n.LHS, level+levelStep)
		parseNode(w, n.RHS, level+levelStep)
	case *promParser.Call:
		printNode(w, level, "Call:")
		level += levelStep
		printNode(w, level, "* Type: %v", n.Type())
		printNode(w, level, "* Func: %v", n.Func.Name)
		printNode(w, level, "* Args: %v", n.Args)
		parseNode(w, n.Args, level+levelStep)
	case *promParser.ParenExpr:
		printNode(w, level, "ParenExpr:")
		level += levelStep
		printNode(w, level, "* Type: %v", n.Type())
		printNode(w, level, "* Expr: %v", n.Expr)
		parseNode(w, n.Expr, level+levelStep)
	case *promParser.SubqueryExpr:
		printNode(w, level, "SubqueryExpr:")
		level += levelStep
		printNode(w, level, "* Type: %v", n.Type())
		printNode(w, level, "* Expr: %v", n.Expr)
		printNode(w, level, "* Step: %v", n.Step)
		if n.StepExpr != nil {
			printNode(w, level, "* StepExpr: %v", n.StepExpr)
			parseNode(w, n.StepExpr, level+levelStep)
		}
		printNode(w, level, "* Range: %v", n.Range)
		if n.RangeExpr != nil {
			printNode(w, level, "* RangeExpr: %v", n.RangeExpr)
			parseNode(w, n.RangeExpr, level+levelStep)
		}
		printNode(w, level, "* Offset: %v", n.Offset)
		if n.OriginalOffsetExpr != nil {
			printNode(w, level, "* OriginalOffsetExpr: %v", n.OriginalOffsetExpr)
			parseNode(w, n.OriginalOffsetExpr, level+levelStep)
		}
		parseNode(w, n.Expr, level+levelStep)
	case *promParser.MatrixSelector:
		printNode(w, level, "MatrixSelector:")
		level += levelStep
		printNode(w, level, "* Type: %v", n.Type())
		printNode(w, level, "* VectorSelector: %v", n.VectorSelector)
		printNode(w, level, "* Range: %v", n.Range)
		if n.RangeExpr != nil {
			printNode(w, level, "* RangeExpr: %v", n.RangeExpr)
			parseNode(w, n.RangeExpr, level+levelStep)
		}
	case *promParser.VectorSelector:
		printNode(w, level, "VectorSelector:")
		level += levelStep
		printNode(w, level, "* Type: %v", n.Type())
		printNode(w, level, "* Name: %v", n.Name)
		printNode(w, level, "* Offset: %v", n.Offset)
		if n.OriginalOffsetExpr != nil {
			printNode(w, level, "* OriginalOffsetExpr: %v", n.OriginalOffsetExpr)
			parseNode(w, n.OriginalOffsetExpr, level+levelStep)
		}
		printNode(w, level, "* LabelMatchers: %v", n.LabelMatchers)
		if n.Anchored {
			printNode(w, level, "* Anchored: true")
		}
		if n.Smoothed {
			printNode(w, level, "* Smoothed: true")
		}
	case *promParser.NumberLiteral:
		printNode(w, level, "NumberLiteral:")
		level += levelStep
		printNode(w, level, "* Type: %v", n.Type())
	case *promParser.StringLiteral:
		printNode(w, level, "StringLiteral:")
		level += levelStep
		printNode(w, level, "* Type: %v", n.Type())
	case *promParser.DurationExpr:
		printNode(w, level, "DurationExpr:")
		level += levelStep
		printNode(w, level, "* Type: %v", n.Type())
		printNode(w, level, "* Op: %v", n.Op)
		printNode(w, level, "* LHS: %v", n.LHS)
		printNode(w, level, "* RHS: %v", n.RHS)
		printNode(w, level, "* Wrapped: %v", n.Wrapped)
	case *promParser.UnaryExpr:
		printNode(w, level, "UnaryExpr:")
		level += levelStep
		printNode(w, level, "* Type: %v", n.Type())
		printNode(w, level, "* Op: %v", n.Op)
		printNode(w, level, "* Expr: %v", n.Expr)
	default:
		printNode(w, level, "! Unsupported node")
	}
}

func parseQuery(w io.Writer, query string) error {
	expr, err := parser.PromQLParser.ParseExpr(query)
	if err != nil {
		return err
	}
	parseNode(w, expr, 0)
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
	return parseQuery(os.Stdout, query)
}
