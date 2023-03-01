package utils_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/parser/utils"
)

func TestHasOuterSum(t *testing.T) {
	type testCaseT struct {
		expr   string
		output []string
	}

	testCases := []testCaseT{
		{
			expr: "foo",
		},
		{
			expr:   "sum(foo)",
			output: []string{"sum(foo)"},
		},
		{
			expr:   "sum(foo) > 0",
			output: []string{"sum(foo)"},
		},
		{
			expr:   "sum(foo) / sum(rate(bar[2m]))",
			output: []string{"sum(foo)"},
		},
		{
			expr:   "sum(sum(foo)) > 0",
			output: []string{"sum(sum(foo))"},
		},
		{
			expr: "count(sum(foo)) > 0",
		},
		{
			expr: "count_values(\"foo\", sum(foo)) > 0",
		},
		{
			expr:   "floor(sum(foo)) > 0",
			output: []string{"sum(foo)"},
		},
		{
			expr:   "ceil(sum(foo)) > 0",
			output: []string{"sum(foo)"},
		},
		{
			expr:   "sum(foo) or sum(bar)",
			output: []string{"sum(foo)", "sum(bar)"},
		},
		{
			expr:   "sum(foo) * on() sum(bar)",
			output: []string{"sum(foo)", "sum(bar)"},
		},
		{
			expr:   "sum(foo) without() * on() group_left(instance) sum(deriv(foo[2m]))",
			output: []string{"sum without () (foo)"},
		},
		{
			expr:   "sum(foo) without(job) * on() group_right(instance) sum(deriv(foo[2m]))",
			output: []string{"sum(deriv(foo[2m]))"},
		},
		{
			expr: "2 > foo",
		},
		{
			expr: "2 > sum(foo)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.expr, func(t *testing.T) {
			n, err := parser.DecodeExpr(tc.expr)
			if err != nil {
				t.Error(err)
				t.FailNow()
			}
			calls := utils.HasOuterSum(n)
			if len(calls) == 0 {
				if len(tc.output) > 0 {
					t.Errorf("HasOuterSum() returned nil, expected %s", tc.output)
				}
			} else {
				output := []string{}
				for _, a := range calls {
					output = append(output, a.String())
				}
				require.Equal(t, tc.output, output, "HasOuterSum() returned wrong output")
			}
		})
	}
}
