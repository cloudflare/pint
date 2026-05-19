package diags

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
	promParser "github.com/prometheus/prometheus/promql/parser"
	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/output"
)

func TestInjectDiagnostics(t *testing.T) {
	type testCaseT struct {
		name  string
		input string
		diags []Diagnostic
	}

	testCases := []testCaseT{
		{
			name:  "single diagnostic on one line",
			input: "expr: foo(bar) by()",
			diags: []Diagnostic{
				{FirstColumn: 1, LastColumn: 13, Message: "this is bad"},
			},
		},
		{
			name:  "caret in the middle of a line",
			input: "expr: foo(bar) on()",
			diags: []Diagnostic{
				{FirstColumn: 10, LastColumn: 11, Message: "oops"},
			},
		},
		{
			name: "two diagnostics on different columns",
			input: `
expr: sum(foo{job="bar"})
      / on(a,b)
      sum(foo)
`,
			diags: []Diagnostic{
				{FirstColumn: 23, LastColumn: 29, Message: "abc"},
				{FirstColumn: 26, LastColumn: 28, Message: "efg"},
			},
		},
		{
			name: "YAML literal block scalar",
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
		},
		{
			name: "two diagnostics on same columns",
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
		},
		{
			name: "YAML folded block scalar with surrounding lines",
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
		},
		{
			name:  "single column caret",
			input: "expr: cnt(bar) by()",
			diags: []Diagnostic{
				{FirstColumn: 14, LastColumn: 14, Message: "this is bad"},
			},
		},
		{
			name: "multi-line expression with caret on last line",
			input: `
expr: |
  foo{
  job="bar"
  }
`,
			diags: []Diagnostic{
				{FirstColumn: 1, LastColumn: 16, Message: "this is bad"},
			},
		},
		{
			name:  "issue and context diagnostics on same column",
			input: "expr: foo(bar) by()",
			diags: []Diagnostic{
				{FirstColumn: 1, LastColumn: 13, Message: "this is bad", Kind: Issue},
				{FirstColumn: 1, LastColumn: 13, Message: "this is context", Kind: Context},
			},
		},
		{
			name:  "rightmost caret is printed first",
			input: "expr: foo(bar) by()",
			diags: []Diagnostic{
				{FirstColumn: 1, LastColumn: 13, Message: "this is bad", Kind: Issue},
				{FirstColumn: 10, LastColumn: 13, Message: "this is context", Kind: Context},
			},
		},
		{
			name:  "multiple position ranges on same line compute min and max columns",
			input: `sum by (instance) (rate(http_requests_total{job="api",status=~"5.."}[5m])) / sum by (instance) (rate(up{job="api"}[5m])) > 0.01`,
			diags: []Diagnostic{
				{
					Message: "check this",
					Pos: PositionRanges{
						{Line: 1, FirstColumn: 1, LastColumn: 74},
						{Line: 1, FirstColumn: 78, LastColumn: 120},
					},
					FirstColumn: 98,
					LastColumn:  101,
				},
			},
		},
		{
			name:  "short expression range on long line skips AST trimming",
			input: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa: sum(foo) by(bar)",
			diags: []Diagnostic{
				{
					Message:     "bad",
					Pos:         PositionRanges{{Line: 1, FirstColumn: 96, LastColumn: 99}},
					FirstColumn: 96,
					LastColumn:  99,
				},
			},
		},
		{
			name:  "AST trimming with multi-line positions skips other lines",
			input: `sum by (instance) (rate(http_requests_total{job="api",status=~"5.."}[5m])) / sum by (instance) (rate(up{job="api"}[5m])) > 0.01`,
			diags: []Diagnostic{
				{
					Message:     "dead code",
					Pos:         PositionRanges{{Line: 1, FirstColumn: 1, LastColumn: 120}},
					FirstColumn: 98,
					LastColumn:  101,
				},
				{
					Message: "extra",
					Pos: PositionRanges{
						{Line: 1, FirstColumn: 98, LastColumn: 101},
						{Line: 2, FirstColumn: 1, LastColumn: 5},
					},
					FirstColumn: 98,
					LastColumn:  101,
				},
			},
		},
		{
			name:  "empty message diagnostic produces no message line",
			input: "expr: foo(bar) by()",
			diags: []Diagnostic{
				{FirstColumn: 1, LastColumn: 13, Message: ""},
			},
		},
		{
			name:  "non-consecutive lines produce gap marker",
			input: "line1: foo\nline2: bar\nline3: baz\n",
			diags: []Diagnostic{
				{
					Message:     "err1",
					Pos:         PositionRanges{{Line: 1, FirstColumn: 8, LastColumn: 10}},
					FirstColumn: 8,
					LastColumn:  10,
				},
				{
					Message:     "err3",
					Pos:         PositionRanges{{Line: 3, FirstColumn: 8, LastColumn: 10}},
					FirstColumn: 8,
					LastColumn:  10,
				},
			},
		},
		{
			name: "non-overlapping subexpression is replaced with ellipsis",
			input: `
expr: sum by (instance) (rate(http_requests_total{job="api",status=~"5.."}[5m])) / sum by (instance) (rate(up{job="api"}[5m])) > 0.01`,
			diags: []Diagnostic{
				{FirstColumn: 104, LastColumn: 107, Message: "dead code"},
			},
		},
		{
			name: "large vector selector inside sum is replaced",
			input: `
expr: sum(oooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooo) by(x)`,
			diags: []Diagnostic{
				{FirstColumn: 97, LastColumn: 101, Message: "by(x) issue"},
			},
		},
		{
			name: "invalid PromQL skips trimming and keeps full line",
			input: `
expr: sum(rate(http_requests_total{job="api",status=~"5.."}[5m]) / sum(rate(up{job="api"}[5m])) > 0.01`,
			diags: []Diagnostic{
				{FirstColumn: 1, LastColumn: 3, Message: "syntax error"},
			},
		},
		{
			name: "invalid PromQL with far right caret places message on left",
			input: `
expr: sum(rate(http_requests_total{job="api",status=~"5.."}[5m])) / sum(rate(up{job="api"}[5m]))) > 0.01`,
			diags: []Diagnostic{
				{FirstColumn: 105, LastColumn: 108, Message: "syntax error"},
			},
		},
		{
			name: "replacement after diagnostic does not shift caret",
			input: `
expr: sum(foo) + sum(rate(very_long_metric_name_aaaa{job="api",status=~"5..",instance=~".*"}[5m])) > 0`,
			diags: []Diagnostic{
				{FirstColumn: 1, LastColumn: 8, Message: "bad sum"},
			},
		},
		{
			name: "far right caret places short message on left",
			input: `
expr: sum(foo) without(colo_id, instance, node_type, region, node_status, job, colo_name)`,
			diags: []Diagnostic{
				{FirstColumn: 80, LastColumn: 89, Message: "bad label"},
			},
		},
		{
			name: "far right caret wraps long message on left",
			input: `
expr: sum(foo) without(colo_id, instance, node_type, region, node_status, job, colo_name)`,
			diags: []Diagnostic{
				{FirstColumn: 80, LastColumn: 89, Message: "Using `without(colo_id, instance, node_type, region, node_status, job, colo_name)` removes all these labels from the results."},
			},
		},
		{
			name: "multi-line expression trims individual long lines",
			input: `
expr: |
  sum(rate(very_long_metric_name_that_pushes_past_the_width_limit_aaaa{job="api",status=~"5.."}[5m])) by(instance)
  + sum(rate(another_very_long_metric_name_that_pushes_past_the_width_limit{job="api"}[5m]))`,
			diags: []Diagnostic{
				{FirstColumn: 3, LastColumn: 8, Message: "bad rate"},
			},
		},
		{
			name: "diagnostic covers entire expression so no nodes are replaced",
			input: `
expr: sum(rate(very_long_metric_name_that_pushes_past_the_width_limit_aaaa{job="api",status=~"5.."}[5m])) by(instance)`,
			diags: []Diagnostic{
				{FirstColumn: 1, LastColumn: 109, Message: "bad query"},
			},
		},
		{
			name: "right side message wraps at line width limit",
			input: `
expr: sum(foo) without(colo_id, instance, node_type, region, node_status, job, colo_name)`,
			diags: []Diagnostic{
				{FirstColumn: 18, LastColumn: 21, Message: "Query is using aggregation with `without(colo_id, instance, node_type, region, node_status, job, colo_name)`, all labels included inside `without(...)` will be removed from the results. `job` label is required and should be preserved when aggregating all rules."},
			},
		},
		{
			name: "multi-line expression trims long inner line",
			input: `
expr: >-
  sum by (exporter, colo_name) (rate(otelcol_exporter_send_failed_log_records{node_status="v", exporter=~"(failover|otlp)/.*"}[5m]))
  /
  (
    sum by (exporter, colo_name) (rate(otelcol_exporter_send_failed_log_records{exporter=~"(failover|otlp)/.*"}[5m])) + sum by (exporter, colo_name) (rate(otelcol_exporter_sent_log_records{exporter=~"(failover|otlp)/.*"}[5m]))
  )
  > 0.1
  and
  sum by (exporter, colo_name) (rate(otelcol_exporter_send_failed_log_records{node_status="v", exporter=~"(failover|otlp)/.*"}[5m])) > 10`,
			diags: []Diagnostic{
				{FirstColumn: 323, LastColumn: 352, Message: "smelly regexp selector"},
			},
		},
	}

	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok, "can't get caller function")
	file = strings.TrimSuffix(filepath.Base(file), ".go")
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			diags := make([]Diagnostic, 0, len(tc.diags))
			allHavePos := true
			for _, d := range tc.diags {
				if len(d.Pos) == 0 {
					allHavePos = false
					break
				}
			}

			if allHavePos {
				diags = append(diags, tc.diags...)
			} else {
				key, val := parseYaml(tc.input)
				require.NotNil(t, key)
				require.NotNil(t, val)
				pos := NewPositionRange(strings.Split(tc.input, "\n"), val, key.Column+2)
				require.NotEmpty(t, pos)

				var expr promParser.Node
				if node, err := promParser.NewParser(promParser.Options{}).ParseExpr(val.Value); err == nil {
					expr = node
				}

				for _, diag := range tc.diags {
					diags = append(diags, Diagnostic{
						Message:     diag.Message,
						Kind:        diag.Kind,
						Pos:         pos,
						Expr:        expr,
						FirstColumn: diag.FirstColumn,
						LastColumn:  diag.LastColumn,
					})
				}
			}

			out := InjectDiagnostics(tc.input, diags, output.None)
			snaps.WithConfig(snaps.Dir("."), snaps.Filename(file)).MatchSnapshot(
				t,
				tc.input,
				"",
				out,
			)
		})
	}
}
