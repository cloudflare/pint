package parser_test

import (
	"testing"

	promParser "github.com/prometheus/prometheus/promql/parser"
	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/parser"
)

func TestWalkUpExpr(t *testing.T) {
	type testCaseT struct {
		walkFromChild func(*parser.PromQLNode) *parser.PromQLNode
		name          string
		expr          string
		expected      int
	}

	testCases := []testCaseT{
		{
			name:          "nil node returns empty",
			expr:          "",
			walkFromChild: nil,
			expected:      0,
		},
		{
			name: "single VectorSelector matches itself",
			expr: "foo",
			walkFromChild: func(n *parser.PromQLNode) *parser.PromQLNode {
				return n
			},
			expected: 1,
		},
		{
			name: "VectorSelector inside rate - walk from selector finds both",
			expr: "rate(foo[5m])",
			walkFromChild: func(n *parser.PromQLNode) *parser.PromQLNode {
				// rate -> MatrixSelector -> VectorSelector, walk from deepest
				return n.Children[0].Children[0]
			},
			expected: 1, // only VectorSelector matches
		},
		{
			name: "sum(rate(foo[5m])) - walk from VectorSelector",
			expr: "sum(rate(foo[5m]))",
			walkFromChild: func(n *parser.PromQLNode) *parser.PromQLNode {
				// sum(AggregateExpr) -> rate(Call) -> MatrixSelector -> VectorSelector
				return n.Children[0].Children[0].Children[0]
			},
			expected: 1, // only the VectorSelector matches
		},
		{
			name: "binary expr - walk from left side",
			expr: "foo + bar",
			walkFromChild: func(n *parser.PromQLNode) *parser.PromQLNode {
				// BinaryExpr -> left VectorSelector
				return n.Children[0]
			},
			expected: 1, // left VectorSelector
		},
		{
			name: "nested aggregation - walk from innermost",
			expr: "sum(avg(foo))",
			walkFromChild: func(n *parser.PromQLNode) *parser.PromQLNode {
				// sum -> avg -> foo, walk from foo
				return n.Children[0].Children[0]
			},
			expected: 1, // only foo is VectorSelector
		},
		{
			name: "no match in tree",
			expr: "1 + 2",
			walkFromChild: func(n *parser.PromQLNode) *parser.PromQLNode {
				return n
			},
			expected: 0, // no VectorSelector in numeric expression
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.walkFromChild == nil {
				// Test nil node case
				nodes := parser.WalkUpExpr[*promParser.VectorSelector](nil)
				require.Len(t, nodes, tc.expected)
				return
			}

			query, err := parser.DecodeExpr(tc.expr)
			require.NoError(t, err)

			startNode := tc.walkFromChild(query)
			nodes := parser.WalkUpExpr[*promParser.VectorSelector](startNode)
			require.Len(t, nodes, tc.expected)
		})
	}
}

func TestWalkUpExprForCallNodes(t *testing.T) {
	type testCaseT struct {
		walkFromChild func(*parser.PromQLNode) *parser.PromQLNode
		name          string
		expr          string
		expected      int
	}

	testCases := []testCaseT{
		{
			name: "rate(foo[5m]) - walk from selector finds one Call",
			expr: "rate(foo[5m])",
			walkFromChild: func(n *parser.PromQLNode) *parser.PromQLNode {
				// rate(Call) -> MatrixSelector -> VectorSelector
				return n.Children[0].Children[0]
			},
			expected: 1,
		},
		{
			name: "sum(rate(foo[5m])) - walk from selector finds one Call (sum is AggregateExpr)",
			expr: "sum(rate(foo[5m]))",
			walkFromChild: func(n *parser.PromQLNode) *parser.PromQLNode {
				// sum(AggregateExpr) -> rate(Call) -> MatrixSelector -> VectorSelector
				return n.Children[0].Children[0].Children[0]
			},
			expected: 1, // only rate is Call, sum is AggregateExpr
		},
		{
			name: "histogram_quantile(0.9, sum(rate(foo[5m]))) - two Calls",
			expr: "histogram_quantile(0.9, sum(rate(foo[5m])))",
			walkFromChild: func(n *parser.PromQLNode) *parser.PromQLNode {
				// histogram_quantile(Call) -> sum(AggregateExpr) -> rate(Call) -> MatrixSelector -> VectorSelector
				return n.Children[1].Children[0].Children[0].Children[0]
			},
			expected: 2, // histogram_quantile and rate are Calls
		},
		{
			name: "plain metric - no Call nodes",
			expr: "foo",
			walkFromChild: func(n *parser.PromQLNode) *parser.PromQLNode {
				return n
			},
			expected: 0,
		},
		{
			name: "clamp(foo, 0, 1) - walk from root finds Call",
			expr: "clamp(foo, 0, 1)",
			walkFromChild: func(n *parser.PromQLNode) *parser.PromQLNode {
				return n
			},
			expected: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			query, err := parser.DecodeExpr(tc.expr)
			require.NoError(t, err)

			startNode := tc.walkFromChild(query)
			nodes := parser.WalkUpExpr[*promParser.Call](startNode)
			require.Len(t, nodes, tc.expected)
		})
	}
}

func TestWalkUpExprForAggregateExpr(t *testing.T) {
	type testCaseT struct {
		walkFromChild func(*parser.PromQLNode) *parser.PromQLNode
		name          string
		expr          string
		expected      int
	}

	testCases := []testCaseT{
		{
			name: "sum(foo) - walk from selector finds one AggregateExpr",
			expr: "sum(foo)",
			walkFromChild: func(n *parser.PromQLNode) *parser.PromQLNode {
				// sum(AggregateExpr) -> VectorSelector
				return n.Children[0]
			},
			expected: 1,
		},
		{
			name: "sum(avg(foo)) - walk from selector finds two AggregateExprs",
			expr: "sum(avg(foo))",
			walkFromChild: func(n *parser.PromQLNode) *parser.PromQLNode {
				// sum -> avg -> foo
				return n.Children[0].Children[0]
			},
			expected: 2,
		},
		{
			name: "rate(foo[5m]) - no AggregateExpr",
			expr: "rate(foo[5m])",
			walkFromChild: func(n *parser.PromQLNode) *parser.PromQLNode {
				return n.Children[0].Children[0]
			},
			expected: 0,
		},
		{
			name: "sum by (job) (rate(foo[5m])) - walk from selector",
			expr: "sum by (job) (rate(foo[5m]))",
			walkFromChild: func(n *parser.PromQLNode) *parser.PromQLNode {
				// sum -> rate -> MatrixSelector -> VectorSelector
				return n.Children[0].Children[0].Children[0]
			},
			expected: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			query, err := parser.DecodeExpr(tc.expr)
			require.NoError(t, err)

			startNode := tc.walkFromChild(query)
			nodes := parser.WalkUpExpr[*promParser.AggregateExpr](startNode)
			require.Len(t, nodes, tc.expected)
		})
	}
}
