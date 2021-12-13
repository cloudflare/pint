package utils_test

import (
	"testing"

	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/parser/utils"
	"github.com/google/go-cmp/cmp"
)

func TestHasVectorSelector(t *testing.T) {
	type testCaseT struct {
		expr   string
		output []string
	}

	testCases := []testCaseT{
		{
			expr:   "foo",
			output: []string{"foo"},
		},
		{
			expr:   "sum(foo)",
			output: []string{"foo"},
		},
		{
			expr:   `foo{job="bar"}`,
			output: []string{`foo{job="bar"}`},
		},
		{
			expr:   `rate(foo{job="bar"}[5m])`,
			output: []string{`foo{job="bar"}`},
		},
		{
			expr:   `(foo{job="bar", a="b"}) / bar`,
			output: []string{`foo{a="b",job="bar"}`, "bar"},
		},
		{
			expr:   `absent(rate(foo{job="bar"}[5m]))`,
			output: []string{`foo{job="bar"}`},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.expr, func(t *testing.T) {
			n, err := parser.DecodeExpr(tc.expr)
			if err != nil {
				t.Error(err)
				t.FailNow()
			}
			vs := utils.HasVectorSelector(n)
			if len(vs) == 0 {
				if len(tc.output) > 0 {
					t.Errorf("HasVectorSelector() returned nil, expected %s", tc.output)
				}
			} else {
				var output = []string{}
				for _, v := range vs {
					output = append(output, v.String())
				}
				if diff := cmp.Diff(tc.output, output); diff != "" {
					t.Errorf("HasVectorSelector() returned wrong result (-want +got):\n%s", diff)
					return
				}
			}
		})
	}
}
