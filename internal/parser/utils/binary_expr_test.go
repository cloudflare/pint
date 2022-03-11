package utils_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/parser/utils"
)

func TestHasOuterBinaryExpr(t *testing.T) {
	type testCaseT struct {
		expr   string
		output string
	}

	testCases := []testCaseT{
		{
			expr: "foo",
		},
		{
			expr:   "foo / bar",
			output: "foo / bar",
		},
		{
			expr:   "(foo / bar)",
			output: "foo / bar",
		},
		{
			expr:   "(foo / bar) / bob",
			output: "(foo / bar) / bob",
		},
		{
			expr:   "foo / bar / bob",
			output: "foo / bar / bob",
		},
		{
			expr:   "foo / bar > 0",
			output: "foo / bar > 0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.expr, func(t *testing.T) {
			n, err := parser.DecodeExpr(tc.expr)
			if err != nil {
				t.Error(err)
				t.FailNow()
			}
			bin := utils.HasOuterBinaryExpr(n)
			if bin == nil {
				if tc.output != "" {
					t.Errorf("HasOuterBinaryExpr() returned nil, expected %s", tc.output)
				}
			} else {
				require.Equal(t, tc.output, bin.String(), "HasOuterBinaryExpr() returned wrong output")
			}
		})
	}
}
