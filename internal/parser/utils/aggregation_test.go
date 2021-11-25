package utils_test

import (
	"testing"

	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/parser/utils"
	"github.com/google/go-cmp/cmp"
)

func TestHasOuterAggregation(t *testing.T) {
	type testCaseT struct {
		expr   string
		output string
	}

	testCases := []testCaseT{
		{
			expr: "foo",
		},
		{
			expr:   "sum(foo)",
			output: "sum(foo)",
		},
		{
			expr:   "sum(foo) by(job)",
			output: "sum by(job) (foo)",
		},
		{
			expr:   "sum(foo) without(job)",
			output: "sum without(job) (foo)",
		},
		{
			expr:   "1 + sum(foo)",
			output: "sum(foo)",
		},
		{
			expr:   "sum(foo) + sum(bar)",
			output: "sum(foo)",
		},
		{
			expr: "foo / on(bbb) sum(bar)",
		},
		{
			expr:   "sum(foo) / on(bbb) sum(bar)",
			output: "sum(foo)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.expr, func(t *testing.T) {
			n, err := parser.DecodeExpr(tc.expr)
			if err != nil {
				t.Error(err)
				t.FailNow()
			}
			output := utils.HasOuterAggregation(n)
			if output == nil {
				if tc.output != "" {
					t.Errorf("HasOuterAggregation() returned nil, expected %q", tc.output)
				}
			} else {
				if diff := cmp.Diff(tc.output, output.String()); diff != "" {
					t.Errorf("HasOuterAggregation() returned wrong result (-want +got):\n%s", diff)
					return
				}
			}
		})
	}
}
