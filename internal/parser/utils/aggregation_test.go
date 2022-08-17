package utils_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/parser/utils"
)

func TestHasOuterAggregation(t *testing.T) {
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
			expr:   "sum(foo) by(job)",
			output: []string{"sum by (job) (foo)"},
		},
		{
			expr:   "sum(foo) without(job)",
			output: []string{"sum without (job) (foo)"},
		},
		{
			expr:   "1 + sum(foo)",
			output: []string{"sum(foo)"},
		},
		{
			expr:   "vector(0) or sum(foo)",
			output: []string{"sum(foo)"},
		},
		{
			expr:   "sum(foo) or vector(0)",
			output: []string{"sum(foo)"},
		},
		{
			expr:   "sum(foo) + sum(bar)",
			output: []string{"sum(foo)", "sum(bar)"},
		},
		{
			expr: "foo / on(bbb) sum(bar)",
		},
		{
			expr:   "sum(foo) / on(bbb) sum(bar)",
			output: []string{"sum(foo)"},
		},
		{
			expr:   "sum(foo) OR sum(bar) by(job)",
			output: []string{"sum(foo)", "sum by (job) (bar)"},
		},
		{
			expr:   "foo OR sum(foo) OR sum(bar) by(job)",
			output: []string{"sum(foo)", "sum by (job) (bar)"},
		},
		{
			expr:   "1 + sum(foo) by(job) + sum(foo) by(notjob)",
			output: []string{"sum by (job) (foo)", "sum by (notjob) (foo)"},
		},
		{
			expr:   "sum(foo) by (job) > count(bar)",
			output: []string{"sum by (job) (foo)"},
		},
		{
			expr:   "sum(foo) by (job) > count(foo) / 2 or sum(bar) by (job) > count(bar)",
			output: []string{"sum by (job) (foo)", "sum by (job) (bar)"},
		},
		{
			expr:   "(foo unless on(instance, version, package) bar) and on(instance) (sum(enabled) by(instance) > 0)",
			output: []string{},
		},
		{
			expr:   "count(build_info) by (instance, version) != ignoring(bar) group_left(package) count(foo) by (instance, version, package)",
			output: []string{"count by (instance, version, package) (build_info)"},
		},
		{
			expr:   "sum(foo) without() != on() group_left(instance) sum(vector(0))",
			output: []string{"sum without () (foo)"},
		},
		{
			expr:   "sum(foo) != on() group_right(instance) sum(vector(0))",
			output: []string{"sum by (instance) (vector(0))"},
		},
		{
			expr:   "min(foo) by(bar) and max by(bar) (foo)",
			output: []string{"min by (bar) (foo)"},
		},
		{
			expr:   "max(foo)",
			output: []string{"max(foo)"},
		},
		{
			expr:   "avg(foo) by(bar)",
			output: []string{"avg by (bar) (foo)"},
		},
		{
			expr:   "group(foo)",
			output: []string{"group(foo)"},
		},
		{
			expr:   "stddev(foo)",
			output: []string{"stddev(foo)"},
		},
		{
			expr:   "(stdvar(foo))",
			output: []string{"stdvar(foo)"},
		},
		{
			expr:   `(1 - count_values("job", foo))`,
			output: []string{`count_values("job", foo)`},
		},
		{
			expr: "(topk(5, foo))",
		},
		{
			expr: "(bottomk(5, foo))",
		},
		{
			expr:   "(quantile(0.9, foo))",
			output: []string{"quantile(0.9, foo)"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.expr, func(t *testing.T) {
			n, err := parser.DecodeExpr(tc.expr)
			if err != nil {
				t.Error(err)
				t.FailNow()
			}
			aggs := utils.HasOuterAggregation(n)
			if len(aggs) == 0 {
				if len(tc.output) > 0 {
					t.Errorf("HasOuterAggregation() returned nil, expected %s", tc.output)
				}
			} else {
				output := []string{}
				for _, a := range aggs {
					output = append(output, a.String())
				}
				require.Equal(t, tc.output, output, "HasOuterAggregation() returned wrong output")
			}
		})
	}
}
