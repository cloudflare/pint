package utils_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/parser/utils"
)

func TestHasOuterAbsent(t *testing.T) {
	type callT struct {
		call    string
		binExpr string
	}

	type testCaseT struct {
		expr   string
		output []callT
	}

	testCases := []testCaseT{
		{
			expr: "foo",
		},
		{
			expr:   "absent(foo)",
			output: []callT{{call: "absent(foo)"}},
		},
		{
			expr:   `absent(foo{job="bar"})`,
			output: []callT{{call: `absent(foo{job="bar"})`}},
		},
		{
			expr: `absent(foo{job="bar"}) AND on(job) bar`,
			output: []callT{{
				call:    `absent(foo{job="bar"})`,
				binExpr: `absent(foo{job="bar"}) and on(job) bar`,
			}},
		},
		{
			expr: `vector(1) or absent(foo{job="bar"}) AND on(job) bar`,
			output: []callT{{
				call:    `absent(foo{job="bar"})`,
				binExpr: `absent(foo{job="bar"}) and on(job) bar`,
			}},
		},
		{
			expr: `up == 0 or absent(foo{job="bar"}) AND on(job) bar`,
			output: []callT{{
				call:    `absent(foo{job="bar"})`,
				binExpr: `absent(foo{job="bar"}) and on(job) bar`,
			}},
		},
		{
			expr: `up == 0 or absent(foo{job="bar"}) or absent(bar)`,
			output: []callT{
				{call: `absent(foo{job="bar"})`},
				{call: `absent(bar)`},
			},
		},
		{
			expr: `absent(sum(nonexistent{job="myjob"}))`,
			output: []callT{
				{call: `absent(sum(nonexistent{job="myjob"}))`},
			},
		},
		{
			expr: `up == 0 or absent(foo{job="bar"}) * on(job) group_left(xxx) bar`,
			output: []callT{{
				call:    `absent(foo{job="bar"})`,
				binExpr: `absent(foo{job="bar"}) * on(job) group_left(xxx) bar`,
			}},
		},
		{
			expr:   `bar * on() group_left(xxx) absent(foo{job="bar"})`,
			output: []callT{},
		},
		{
			expr: `up == 0 or absent(foo{job="bar"}) * on(job) group_left() bar`,
			output: []callT{{
				call:    `absent(foo{job="bar"})`,
				binExpr: `absent(foo{job="bar"}) * on(job) group_left() bar`,
			}},
		},
		{
			expr: `bar * on() group_right(xxx) absent(foo{job="bar"})`,
			output: []callT{{
				call:    `absent(foo{job="bar"})`,
				binExpr: `bar * on() group_right(xxx) absent(foo{job="bar"})`,
			}},
		},
		{
			expr:   `absent(foo{job="bar"}) * on(job) group_right(xxx) bar`,
			output: []callT{},
		},
		{
			expr: `absent(foo{job="bar"}) OR bar`,
			output: []callT{{
				call: `absent(foo{job="bar"})`,
			}},
		},
		{
			expr: `absent(foo{job="bar"}) OR absent(foo{job="bob"})`,
			output: []callT{
				{call: `absent(foo{job="bar"})`},
				{call: `absent(foo{job="bob"})`},
			},
		},
		{
			expr: `absent(foo{job="bar"}) UNLESS absent(foo{job="bob"})`,
			output: []callT{
				{
					call:    `absent(foo{job="bar"})`,
					binExpr: `absent(foo{job="bar"}) unless absent(foo{job="bob"})`,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.expr, func(t *testing.T) {
			n, err := parser.DecodeExpr(tc.expr)
			if err != nil {
				t.Error(err)
				t.FailNow()
			}
			calls := utils.HasOuterAbsent(n)
			if len(calls) == 0 {
				if len(tc.output) > 0 {
					t.Errorf("HasOuterAbsent() returned nil, expected %s", tc.output)
				}
			} else {
				output := []callT{}
				for _, a := range calls {
					var c callT
					if a.Fragment != nil {
						c.call = a.Fragment.Node.String()
					}
					if a.BinExpr != nil {
						c.binExpr = a.BinExpr.String()
					}
					output = append(output, c)
				}
				require.Equal(t, tc.output, output, "HasOuterAbsent() returned wrong output")
			}
		})
	}
}
