package checks

import (
	"testing"

	"github.com/prometheus/prometheus/promql/parser/posrange"
	"github.com/stretchr/testify/require"
)

func TestHighlightProblem(t *testing.T) {
	type testCaseT struct {
		input  string
		output string
		pos    posrange.PositionRange
	}

	testCases := []testCaseT{
		{
			input:  "foo(bar) by()",
			pos:    posrange.PositionRange{Start: 0, End: 13},
			output: "foo(bar) by()\n~~~~~~~~~~~~~\n",
		},
		{
			input:  "foo(bar) on()",
			pos:    posrange.PositionRange{Start: 9, End: 10},
			output: "foo(bar) on()\n         ~~  \n",
		},
		{
			input: `
sum(foo{job="bar"})
/ on(a,b)
sum(foo)
`,
			pos: posrange.PositionRange{Start: 23, End: 30},
			output: `
sum(foo{job="bar"})
/ on(a,b)
  ~~~~~~~
sum(foo)
`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			out := highlightProblem(tc.input, tc.pos)
			require.Equal(t, tc.output, out)
		})
	}
}
