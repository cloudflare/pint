package output_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/output"
)

func TestInjectDiagnostics(t *testing.T) {
	type testCaseT struct {
		input               string
		output              string
		diags               []output.Diagnostic
		firstLine, lastLine int
	}

	testCases := []testCaseT{
		{
			input:     "foo(bar) by()",
			firstLine: 1,
			lastLine:  1,
			diags: []output.Diagnostic{
				{Line: 1, FirstColumn: 0, LastColumn: 13, Message: "this is bad"},
			},
			output: `1 | foo(bar) by()
    ^^^^^^^^^^^^^ this is bad
`,
		},
		{
			input:     "foo(bar) on()",
			firstLine: 1,
			lastLine:  1,
			diags: []output.Diagnostic{
				{Line: 1, FirstColumn: 10, LastColumn: 11, Message: "oops"},
			},
			output: `1 | foo(bar) on()
             ^^ oops
`,
		},
		{
			input: `
sum(foo{job="bar"})
/ on(a,b)
sum(foo)
`,
			firstLine: 1,
			lastLine:  4,
			diags: []output.Diagnostic{
				{Line: 2, FirstColumn: 23, LastColumn: 29, Message: "abc"},
				{Line: 2, FirstColumn: 26, LastColumn: 28, Message: "efg"},
			},
			output: `1 | 
2 | sum(foo{job="bar"})
3 | / on(a,b)
         ^^^ efg
      ^^^^^^^ abc
4 | sum(foo)
`,
		},
		{
			input: `
sum(bar{job="foo"})
/ on(c,d)
sum(bar)
`,
			firstLine: 2,
			lastLine:  4,
			diags: []output.Diagnostic{
				{Line: 3, FirstColumn: 3, LastColumn: 9, Message: "abc"},
				{Line: 4, FirstColumn: 1, LastColumn: 3, Message: "efg"},
			},
			output: `2 | sum(bar{job="foo"})
3 | / on(c,d)
      ^^^^^^^ abc
4 | sum(bar)
    ^^^ efg
`,
		},
		{
			input: `
sum(bar{job="foo"})
/ on(c,d)
sum(bar)
`,
			firstLine: 2,
			lastLine:  4,
			diags: []output.Diagnostic{
				{Line: 3, FirstColumn: 3, LastColumn: 9, Message: "abc"},
				{Line: 3, FirstColumn: 3, LastColumn: 9, Message: "efg"},
			},
			output: `2 | sum(bar{job="foo"})
3 | / on(c,d)
      ^^^^^^^ abc
              efg
4 | sum(bar)
`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			out := output.InjectDiagnostics(tc.input, tc.diags, output.None, tc.firstLine, tc.lastLine)
			require.Equal(t, tc.output, out)
		})
	}
}
