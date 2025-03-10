package diags

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/output"
)

func TestInjectDiagnostics(t *testing.T) {
	type testCaseT struct {
		input               string
		output              string
		diags               []Diagnostic
		firstLine, lastLine int
	}

	testCases := []testCaseT{
		{
			input:     "expr: foo(bar) by()",
			firstLine: 1,
			lastLine:  1,
			diags: []Diagnostic{
				{FirstColumn: 1, LastColumn: 13, Message: "this is bad"},
			},
			output: `1 | expr: foo(bar) by()
          ^^^^^^^^^^^^^ this is bad
`,
		},
		{
			input:     "expr: foo(bar) on()",
			firstLine: 1,
			lastLine:  1,
			diags: []Diagnostic{
				{FirstColumn: 10, LastColumn: 11, Message: "oops"},
			},
			output: `1 | expr: foo(bar) on()
                   ^^ oops
`,
		},
		{
			input: `
expr: sum(foo{job="bar"})
      / on(a,b)
      sum(foo)
`,
			firstLine: 2,
			lastLine:  4,
			diags: []Diagnostic{
				{FirstColumn: 23, LastColumn: 29, Message: "abc"},
				{FirstColumn: 26, LastColumn: 28, Message: "efg"},
			},
			output: `2 | expr: sum(foo{job="bar"})
3 |       / on(a,b)
            ^^^^^^^ abc
               ^^^ efg
4 |       sum(foo)
`,
		},
		{
			input: `
expr: |
  sum(bar{job="foo"})
  / on(c,d)
  sum(bar)
`,
			firstLine: 2,
			lastLine:  5,
			diags: []Diagnostic{
				{FirstColumn: 23, LastColumn: 24, Message: "123"},
				{FirstColumn: 31, LastColumn: 33, Message: "456"},
			},
			output: `2 | expr: |
3 |   sum(bar{job="foo"})
4 |   / on(c,d)
        ^^ 123
5 |   sum(bar)
      ^^^ 456
`,
		},
		{
			input: `
expr:
  sum(bar{job="foo"})
  / on(c,d)
  sum(bar)
`,
			firstLine: 2,
			lastLine:  5,
			diags: []Diagnostic{
				{FirstColumn: 23, LastColumn: 29, Message: "abc"},
				{FirstColumn: 23, LastColumn: 29, Message: "efg"},
			},
			output: `2 | expr:
3 |   sum(bar{job="foo"})
4 |   / on(c,d)
        ^^^^^^^ abc
                efg
5 |   sum(bar)
`,
		},
		{
			input: `
### BEGIN ###
expr: >-
  sum(bar{job="foo"})
  / on(c,d)
  sum(bar)
### END ###
`,
			firstLine: 3,
			lastLine:  6,
			diags: []Diagnostic{
				{FirstColumn: 23, LastColumn: 29, Message: "abc"},
				{FirstColumn: 23, LastColumn: 29, Message: "efg"},
			},
			output: `3 | expr: >-
4 |   sum(bar{job="foo"})
5 |   / on(c,d)
        ^^^^^^^ abc
                efg
6 |   sum(bar)
`,
		},
		{
			input:     "expr: cnt(bar) by()",
			firstLine: 1,
			lastLine:  1,
			diags: []Diagnostic{
				{FirstColumn: 14, LastColumn: 14, Message: "this is bad"},
			},
			output: `1 | expr: cnt(bar) by()
                      ^ this is bad
`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			key, val := parseYaml(tc.input)
			require.NotNil(t, key)
			require.NotNil(t, val)
			pos := NewPositionRange(strings.Split(tc.input, "\n"), val, key.Column+2)
			require.NotEmpty(t, pos)

			diags := make([]Diagnostic, 0, len(tc.diags))
			for _, diag := range tc.diags {
				diags = append(diags, Diagnostic{
					Message:     diag.Message,
					Pos:         pos,
					FirstColumn: diag.FirstColumn,
					LastColumn:  diag.LastColumn,
				})
			}

			out := InjectDiagnostics(tc.input, diags, output.None, tc.firstLine, tc.lastLine)
			require.Equal(t, tc.output, out)
		})
	}
}
