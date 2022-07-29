package utils_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/parser/utils"
)

func TestHasOuterRate(t *testing.T) {
	type testCaseT struct {
		expr   string
		output []string
	}

	testCases := []testCaseT{
		{
			expr: "foo",
		},
		{
			expr:   "rate(foo[2m])",
			output: []string{"rate(foo[2m])"},
		},
		{
			expr:   "rate(foo[2m]) > 0",
			output: []string{"rate(foo[2m])"},
		},
		{
			expr:   "rate(foo[2m]) / sum(rate(bar[2m]))",
			output: []string{"rate(foo[2m])"},
		},
		{
			expr:   "sum(rate(foo[2m])) > 0",
			output: []string{"rate(foo[2m])"},
		},
		{
			expr: "count(rate(foo[2m])) > 0",
		},
		{
			expr: "count_values(\"foo\", rate(foo[2m])) > 0",
		},
		{
			expr: "floor(rate(foo[2m])) > 0",
		},
		{
			expr: "ceil(rate(foo[2m])) > 0",
		},
		{
			expr:   "rate(foo[2m]) or irate(foo[2m])",
			output: []string{"rate(foo[2m])", "irate(foo[2m])"},
		},
		{
			expr:   "rate(foo[2m]) * on() irate(foo[2m])",
			output: []string{"rate(foo[2m])", "irate(foo[2m])"},
		},
		{
			expr: "sum(foo) without() * on() group_left(instance) sum(deriv(foo[2m]))",
		},
		{
			expr:   "sum(foo) without(job) * on() group_right(instance) sum(deriv(foo[2m]))",
			output: []string{"deriv(foo[2m])"},
		},
		{
			expr: "2 > foo",
		},
		{
			expr: "2 > rate(foo[2m])",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.expr, func(t *testing.T) {
			n, err := parser.DecodeExpr(tc.expr)
			if err != nil {
				t.Error(err)
				t.FailNow()
			}
			calls := utils.HasOuterRate(n)
			if len(calls) == 0 {
				if len(tc.output) > 0 {
					t.Errorf("HasOuterRate() returned nil, expected %s", tc.output)
				}
			} else {
				output := []string{}
				for _, a := range calls {
					output = append(output, a.String())
				}
				require.Equal(t, tc.output, output, "HasOuterRate() returned wrong output")
			}
		})
	}
}
