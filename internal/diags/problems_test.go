package diags

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/output"
)

func TestInjectDiagnostics(t *testing.T) {
	type testCaseT struct {
		input  string
		output string
		diags  []Diagnostic
	}

	testCases := []testCaseT{
		{
			input: "expr: foo(bar) by()",
			diags: []Diagnostic{
				{FirstColumn: 1, LastColumn: 13, Message: "this is bad"},
			},
			output: `1 | expr: foo(bar) by()
          ^^^^^^^^^^^^^
          this is bad
`,
		},
		{
			input: "expr: foo(bar) on()",
			diags: []Diagnostic{
				{FirstColumn: 10, LastColumn: 11, Message: "oops"},
			},
			output: `1 | expr: foo(bar) on()
                   ^^
                   oops
`,
		},
		{
			input: `
expr: sum(foo{job="bar"})
      / on(a,b)
      sum(foo)
`,
			diags: []Diagnostic{
				{FirstColumn: 23, LastColumn: 29, Message: "abc"},
				{FirstColumn: 26, LastColumn: 28, Message: "efg"},
			},
			output: `2 | expr: sum(foo{job="bar"})
3 |       / on(a,b)
               ^^^
               efg
            ^^^^^^^
            abc
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
			diags: []Diagnostic{
				{FirstColumn: 23, LastColumn: 24, Message: "123"},
				{FirstColumn: 31, LastColumn: 33, Message: "456"},
			},
			output: `3 |   sum(bar{job="foo"})
4 |   / on(c,d)
        ^^
        123
5 |   sum(bar)
      ^^^
      456
`,
		},
		{
			input: `
expr:
  sum(bar{job="foo"})
  / on(c,d)
  sum(bar)
`,
			diags: []Diagnostic{
				{FirstColumn: 23, LastColumn: 29, Message: "abc"},
				{FirstColumn: 23, LastColumn: 29, Message: "efg"},
			},
			output: `3 |   sum(bar{job="foo"})
4 |   / on(c,d)
        ^^^^^^^
        abc
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
			diags: []Diagnostic{
				{FirstColumn: 23, LastColumn: 29, Message: "abc"},
				{FirstColumn: 23, LastColumn: 29, Message: "efg"},
			},
			output: `4 |   sum(bar{job="foo"})
5 |   / on(c,d)
        ^^^^^^^
        abc
        efg
6 |   sum(bar)
`,
		},
		{
			input: "expr: cnt(bar) by()",
			diags: []Diagnostic{
				{FirstColumn: 14, LastColumn: 14, Message: "this is bad"},
			},
			output: `1 | expr: cnt(bar) by()
                      ^
                      this is bad
`,
		},
		{
			input: `
expr: |
  foo{
  job="bar"
  }
`,
			diags: []Diagnostic{
				{FirstColumn: 1, LastColumn: 16, Message: "this is bad"},
			},
			output: `3 |   foo{
4 |   job="bar"
5 |   }
      ^
      this is bad
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

			out := InjectDiagnostics(tc.input, diags, output.None)
			require.Equal(t, tc.output, out)
		})
	}
}

func TestCountDigits(t *testing.T) {
	type testCaseT struct {
		name     string
		input    int
		expected int
	}

	testCases := []testCaseT{
		{name: "single digit", input: 1, expected: 1},
		{name: "two digits", input: 10, expected: 2},
		{name: "three digits", input: 100, expected: 3},
		{name: "zero", input: 0, expected: 0},
		{name: "four digits", input: 1000, expected: 4},
		{name: "five digits", input: 99999, expected: 5},
		{name: "six digits", input: 123456, expected: 6},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, countDigits(tc.input))
		})
	}
}

func TestLineCoverage(t *testing.T) {
	type testCaseT struct {
		name     string
		diags    []Diagnostic
		expected []int
	}

	testCases := []testCaseT{
		{
			name: "multiple lines",
			diags: []Diagnostic{
				{Pos: PositionRanges{{Line: 1}, {Line: 2}}},
				{Pos: PositionRanges{{Line: 2}, {Line: 3}}},
			},
			expected: []int{1, 2, 3},
		},
		{
			name:     "empty",
			diags:    []Diagnostic{},
			expected: nil,
		},
		{
			name: "single line",
			diags: []Diagnostic{
				{Pos: PositionRanges{{Line: 5}}},
			},
			expected: []int{5},
		},
		{
			name: "duplicates",
			diags: []Diagnostic{
				{Pos: PositionRanges{{Line: 3}, {Line: 3}}},
				{Pos: PositionRanges{{Line: 3}, {Line: 5}}},
			},
			expected: []int{3, 5},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := lineCoverage(tc.diags)
			if tc.expected == nil {
				require.Empty(t, result)
			} else {
				require.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestInjectDiagnosticsKind(t *testing.T) {
	input := "expr: foo(bar) by()"
	diags := []Diagnostic{
		{FirstColumn: 1, LastColumn: 13, Message: "this is bad", Kind: Issue},
		{FirstColumn: 1, LastColumn: 13, Message: "this is context", Kind: Context},
	}
	key, val := parseYaml(input)
	pos := NewPositionRange(strings.Split(input, "\n"), val, key.Column+2)
	for i := range diags {
		diags[i].Pos = pos
	}
	out := InjectDiagnostics(input, diags, output.None)
	expected := `1 | expr: foo(bar) by()
          ^^^^^^^^^^^^^
          this is bad
          this is context
`
	require.Equal(t, expected, out)
}

func TestInjectDiagnosticsOrder(t *testing.T) {
	input := "expr: foo(bar) by()"
	diags := []Diagnostic{
		{FirstColumn: 1, LastColumn: 13, Message: "this is bad", Kind: Issue},
		{FirstColumn: 10, LastColumn: 13, Message: "this is context", Kind: Context},
	}
	key, val := parseYaml(input)
	pos := NewPositionRange(strings.Split(input, "\n"), val, key.Column+2)
	for i := range diags {
		diags[i].Pos = pos
	}
	out := InjectDiagnostics(input, diags, output.None)
	expected := `1 | expr: foo(bar) by()
                   ^^^^
                   this is context
          ^^^^^^^^^^^^^
          this is bad
`
	require.Equal(t, expected, out)
}

func TestInjectDiagnosticsTrimmed(t *testing.T) {
	type testCaseT struct {
		name  string
		input string
		diags []Diagnostic
	}

	testCases := []testCaseT{
		{
			// The first operand of the division is an 82-byte AggregateExpr that
			// does not overlap with the diagnostic at column 104; it gets replaced
			// with "..." and the remaining line fits under the width limit.
			name: "ast_trims_large_subexpr",
			input: `
expr: sum by (instance) (rate(http_requests_total{job="api",status=~"5.."}[5m])) / sum by (instance) (rate(up{job="api"}[5m])) > 0.01`,
			diags: []Diagnostic{
				{FirstColumn: 104, LastColumn: 107, Message: "dead code"},
			},
		},
		{
			// The VectorSelector inside sum() is 88 bytes and does not overlap
			// with the diagnostic on "by(x)"; it is the only node replaced.
			name: "ast_trims_vector_selector_inside_sum",
			input: `
expr: sum(oooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooo) by(x)`,
			diags: []Diagnostic{
				{FirstColumn: 97, LastColumn: 101, Message: "by(x) issue"},
			},
		},
		{
			// Missing closing ")" for the first sum() makes the expression
			// unparsable; AST trimming is skipped and the full line is kept.
			name: "invalid_promql_missing_paren_kept_full",
			input: `
expr: sum(rate(http_requests_total{job="api",status=~"5.."}[5m]) / sum(rate(up{job="api"}[5m])) > 0.01`,
			diags: []Diagnostic{
				{FirstColumn: 1, LastColumn: 3, Message: "syntax error"},
			},
		},
		{
			// Extra closing ")" after rate(...) makes the expression unparsable;
			// AST trimming is skipped and the full line is kept.
			name: "invalid_promql_extra_paren_kept_full",
			input: `
expr: sum(rate(http_requests_total{job="api",status=~"5.."}[5m])) / sum(rate(up{job="api"}[5m]))) > 0.01`,
			diags: []Diagnostic{
				{FirstColumn: 105, LastColumn: 108, Message: "syntax error"},
			},
		},
		{
			// A long message starting at a high column leaves little room,
			// forcing it to wrap onto multiple lines.
			name: "long_message_wraps",
			input: `
expr: sum(foo) without(colo_id, instance, node_type, region, node_status, job, colo_name)`,
			diags: []Diagnostic{
				{FirstColumn: 18, LastColumn: 21, Message: "Query is using aggregation with `without(colo_id, instance, node_type, region, node_status, job, colo_name)`, all labels included inside `without(...)` will be removed from the results. `job` label is required and should be preserved when aggregating all rules."},
			},
		},
	}

	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok, "can't get caller function")
	file = strings.TrimSuffix(filepath.Base(file), ".go")
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
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

			out := InjectDiagnostics(tc.input, diags, output.None)
			snaps.WithConfig(snaps.Dir("."), snaps.Filename(file)).MatchSnapshot(t, out)
		})
	}
}

func TestParseASTRanges(t *testing.T) {
	// Valid PromQL.
	ranges := parseASTRanges("sum(foo) by(bar)")
	require.NotNil(t, ranges)
	require.NotEmpty(t, ranges)

	// Invalid PromQL returns nil.
	ranges = parseASTRanges("sum(foo bar")
	require.Nil(t, ranges)
}

func TestIsSingleLineExpr(t *testing.T) {
	require.False(t, isSingleLineExpr(Diagnostic{}, 1))
	require.True(t, isSingleLineExpr(Diagnostic{Pos: PositionRanges{{Line: 1}}}, 1))
	require.False(t, isSingleLineExpr(Diagnostic{Pos: PositionRanges{{Line: 1}, {Line: 2}}}, 1))
}

func TestExtractExprFromLine(t *testing.T) {
	// No positions on the requested line.
	_, _, ok := extractExprFromLine(
		Diagnostic{Pos: PositionRanges{{Line: 1, FirstColumn: 1, LastColumn: 3}}},
		"foo", 2,
	)
	require.False(t, ok)

	// Out of bounds (column past end of line).
	_, _, ok = extractExprFromLine(
		Diagnostic{Pos: PositionRanges{{Line: 1, FirstColumn: 1, LastColumn: 100}}},
		"foo", 1,
	)
	require.False(t, ok)

	// Success.
	expr, start, ok := extractExprFromLine(
		Diagnostic{Pos: PositionRanges{{Line: 1, FirstColumn: 7, LastColumn: 10}}},
		"expr: sum(foo)", 1,
	)
	require.True(t, ok)
	require.Equal(t, 6, start)
	require.Equal(t, "sum(", expr)
}

func TestOffsetForCol(t *testing.T) {
	// No replacements.
	require.Equal(t, 0, offsetForCol(nil, 10, 5))

	// Replacement before the column shifts it.
	// Replace chars 0-5 (length 5) with "..." (length 3) => shift by -2.
	require.Equal(t, -2, offsetForCol([][2]int{{0, 5}}, 10, 20))

	// Replacement after the column does nothing.
	require.Equal(t, 0, offsetForCol([][2]int{{5, 10}}, 10, 14))
}

func TestAstTrimLine(t *testing.T) {
	t.Run("multi-line diag skipped", func(t *testing.T) {
		line := "expr: sum(foo) by(bar)"
		dp := []PositionRanges{{{Line: 1, FirstColumn: 7, LastColumn: 9}, {Line: 2, FirstColumn: 1, LastColumn: 3}}}
		newLine, ok := astTrimLine(line, []Diagnostic{{Pos: dp[0]}}, dp, 1)
		require.False(t, ok)
		require.Equal(t, line, newLine)
	})

	t.Run("short expression skipped", func(t *testing.T) {
		line := "expr: sum(foo)"
		dp := []PositionRanges{{{Line: 1, FirstColumn: 7, LastColumn: 9}}}
		newLine, ok := astTrimLine(line, []Diagnostic{{Pos: dp[0]}}, dp, 1)
		require.False(t, ok)
		require.Equal(t, line, newLine)
	})

	t.Run("invalid promql skipped", func(t *testing.T) {
		line := "expr: sum(foo bar baz) by(x)"
		dp := []PositionRanges{{{Line: 1, FirstColumn: 7, LastColumn: 28}}}
		newLine, ok := astTrimLine(line, []Diagnostic{{Pos: dp[0]}}, dp, 1)
		require.False(t, ok)
		require.Equal(t, line, newLine)
	})

	t.Run("no replaceable nodes", func(t *testing.T) {
		// Diagnostic covers the whole expression, so no AST node is fully outside.
		line := "expr: sum(rate(foo[5m])) by(x)"
		dp := []PositionRanges{{{Line: 1, FirstColumn: 7, LastColumn: 30}}}
		newLine, ok := astTrimLine(line, []Diagnostic{{Pos: dp[0]}}, dp, 1)
		require.False(t, ok)
		require.Equal(t, line, newLine)
	})

	t.Run("nested replacements deduplicated", func(t *testing.T) {
		// sum(rate(very_long_metric_name{job="api"}[5m])) - the VectorSelector
		// inside rate() does not overlap with the diagnostic on by(instance).
		line := `expr: sum(rate(very_long_metric_name{job="api"}[5m])) by(instance)`
		// Full expression Pos so extractExprFromLine gets valid PromQL.
		fullPos := PositionRanges{{
			Line:        1,
			FirstColumn: 7,
			LastColumn:  66,
		}}
		// diagPositions only covers by(instance) so the VectorSelector is replaced.
		byPos := PositionRanges{{
			Line:        1,
			FirstColumn: 49,
			LastColumn:  60,
		}}
		newLine, ok := astTrimLine(line, []Diagnostic{{Pos: fullPos}}, []PositionRanges{byPos}, 1)
		require.True(t, ok)
		require.Contains(t, newLine, "sum(rate(...[5m])) by(instance)")
	})
}

func TestExtractExprFromLineMultiplePositions(t *testing.T) {
	// Multiple positions on the same line should compute min/max columns.
	diag := Diagnostic{Pos: PositionRanges{
		{Line: 1, FirstColumn: 15, LastColumn: 18},
		{Line: 1, FirstColumn: 10, LastColumn: 12},
	}}
	expr, start, ok := extractExprFromLine(diag, "expr: sum(foo) by(bar)", 1)
	require.True(t, ok)
	require.Equal(t, 9, start)
	require.Equal(t, "(foo) by(", expr)
}

func TestCountLeadingSpace(t *testing.T) {
	require.Equal(t, 0, countLeadingSpace("foo"))
	require.Equal(t, 3, countLeadingSpace("   foo"))
	require.Equal(t, 5, countLeadingSpace("     "))
}

func TestInjectDiagnosticsWithGap(t *testing.T) {
	// Diagnostics on non-consecutive lines should produce [...] between them.
	input := "line1: foo\nline2: bar\nline3: baz\n"
	diags := []Diagnostic{
		{Message: "err1", Pos: PositionRanges{{Line: 1, FirstColumn: 8, LastColumn: 10}}, FirstColumn: 8, LastColumn: 10},
		{Message: "err3", Pos: PositionRanges{{Line: 3, FirstColumn: 8, LastColumn: 10}}, FirstColumn: 8, LastColumn: 10},
	}
	out := InjectDiagnostics(input, diags, output.None)
	require.Contains(t, out, "[...]")
}

func TestAstTrimLineMultiLinePositions(t *testing.T) {
	// diagPositions with positions on multiple lines should hit the
	// pos.Line != lineNum continue paths in both diagRanges building
	// and column adjustment.
	line := `expr: sum(rate(very_long_metric_name{job="api"}[5m])) by(instance)`
	fullPos := PositionRanges{{
		Line:        1,
		FirstColumn: 7,
		LastColumn:  66,
	}}
	byPos := PositionRanges{
		{Line: 1, FirstColumn: 49, LastColumn: 60},
		{Line: 2, FirstColumn: 1, LastColumn: 5}, // different line, should be skipped
	}
	newLine, ok := astTrimLine(line, []Diagnostic{{Pos: fullPos}}, []PositionRanges{byPos}, 1)
	require.True(t, ok)
	require.Contains(t, newLine, "sum(rate(...[5m])) by(instance)")
}

func TestWrapTextEmpty(t *testing.T) {
	require.Nil(t, wrapText("", 80))
	require.Nil(t, wrapText("   ", 80))
}

func TestWriteWrappedMessage(t *testing.T) {
	var buf strings.Builder

	// Empty message should write nothing.
	buf.Reset()
	writeWrappedMessage(&buf, "", output.None, 4, 80)
	require.Equal(t, "", buf.String())

	// Short message that fits within width.
	buf.Reset()
	writeWrappedMessage(&buf, "short msg", output.None, 4, 80)
	require.Equal(t, "    short msg\n", buf.String())

	// Long message that needs wrapping.
	buf.Reset()
	writeWrappedMessage(&buf, "this is a long message that exceeds the width", output.None, 4, 20)
	require.Equal(t, "    this is a long\n    message that exceeds\n    the width\n", buf.String())
}
