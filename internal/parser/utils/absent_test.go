package utils_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/parser/utils"
)

func TestHasOuterAbsent(t *testing.T) {
	type testCaseT struct {
		expr   string
		output []string
	}

	testCases := []testCaseT{
		{
			expr: "foo",
		},
		{
			expr:   "absent(foo)",
			output: []string{"absent(foo)"},
		},
		{
			expr:   `absent(foo{job="bar"})`,
			output: []string{`absent(foo{job="bar"})`},
		},
		{
			expr:   `absent(foo{job="bar"}) AND on(job) bar`,
			output: []string{`absent(foo{job="bar"})`},
		},
		{
			expr:   `vector(1) or absent(foo{job="bar"}) AND on(job) bar`,
			output: []string{`absent(foo{job="bar"})`},
		},
		{
			expr:   `up == 0 or absent(foo{job="bar"}) AND on(job) bar`,
			output: []string{`absent(foo{job="bar"})`},
		},
		{
			expr:   `up == 0 or absent(foo{job="bar"}) or absent(bar)`,
			output: []string{`absent(foo{job="bar"})`, `absent(bar)`},
		},
		{
			expr:   `absent(sum(nonexistent{job="myjob"}))`,
			output: []string{`absent(sum(nonexistent{job="myjob"}))`},
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
				output := []string{}
				for _, a := range calls {
					output = append(output, a.Node.String())
				}
				require.Equal(t, tc.output, output, "HasOuterAbsent() returned wrong output")
			}
		})
	}
}
