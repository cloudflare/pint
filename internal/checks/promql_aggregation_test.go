package checks_test

import (
	"regexp"
	"testing"

	"github.com/cloudflare/pint/internal/checks"
)

func TestAggregationCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "ignores rules with syntax errors",
			content:     "- record: foo\n  expr: sum(foo) without(\n",
			checker:     checks.NewAggregationCheck(regexp.MustCompile("^.+$"), "job", true, checks.Warning),
		},
		{
			description: "name must match",
			content:     "- record: foo\n  expr: sum(foo) without(job)\n",
			checker:     checks.NewAggregationCheck(regexp.MustCompile("^bar$"), "job", true, checks.Warning),
		},
		{
			description: "uses label from labels map",
			content:     "- record: foo\n  expr: sum(foo) without(job)\n  labels:\n    job: foo\n",
			checker:     checks.NewAggregationCheck(regexp.MustCompile("^.+$"), "job", true, checks.Warning),
		},
		{
			description: "must keep job label / warning",
			content:     "- record: foo\n  expr: sum(foo) without(instance, job)\n",
			checker:     checks.NewAggregationCheck(regexp.MustCompile("^.+$"), "job", true, checks.Warning),
			problems: []checks.Problem{
				{
					Fragment: "sum(foo) without(instance, job)",
					Lines:    []int{2},
					Reporter: "promql/aggregate",
					Text:     "job label is required and should be preserved when aggregating \"^.+$\" rules, remove job from without()",
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "must keep job label / bug",
			content:     "- record: foo\n  expr: sum(foo) without(instance, job)\n",
			checker:     checks.NewAggregationCheck(regexp.MustCompile("^.+$"), "job", true, checks.Bug),
			problems: []checks.Problem{
				{
					Fragment: "sum(foo) without(instance, job)",
					Lines:    []int{2},
					Reporter: "promql/aggregate",
					Text:     "job label is required and should be preserved when aggregating \"^.+$\" rules, remove job from without()",
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "must strip job label",
			content:     "- record: foo\n  expr: sum(foo) without(instance)\n",
			checker:     checks.NewAggregationCheck(regexp.MustCompile("^.+$"), "job", false, checks.Warning),
			problems: []checks.Problem{
				{
					Fragment: "sum(foo) without(instance)",
					Lines:    []int{2},
					Reporter: "promql/aggregate",
					Text:     "job label should be removed when aggregating \"^.+$\" rules, use without(job, ...)",
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "must strip job label / being stripped",
			content:     "- record: foo\n  expr: sum(foo) without(instance,job)\n",
			checker:     checks.NewAggregationCheck(regexp.MustCompile("^.+$"), "job", false, checks.Warning),
		},
		{
			description: "must strip job label / empty without",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker:     checks.NewAggregationCheck(regexp.MustCompile("^.+$"), "job", false, checks.Warning),
		},
		{
			description: "nested rule must keep job label",
			content:     "- record: foo\n  expr: sum(sum(foo) without(job)) by(job)\n",
			checker:     checks.NewAggregationCheck(regexp.MustCompile("^.+$"), "job", true, checks.Warning),
			problems: []checks.Problem{
				{
					Fragment: "sum without(job) (foo)",
					Lines:    []int{2},
					Reporter: "promql/aggregate",
					Text:     "job label is required and should be preserved when aggregating \"^.+$\" rules, remove job from without()",
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "passing most outer aggregation should stop further strip checks",
			content:     "- record: foo\n  expr: sum(sum(foo) without(foo)) without(instance)\n",
			checker:     checks.NewAggregationCheck(regexp.MustCompile("^.+$"), "instance", false, checks.Warning),
		},
		{
			description: "passing most outer aggregation should stop further checks",
			content:     "- record: foo\n  expr: sum(sum(foo) without(foo)) without(bar)\n",
			checker:     checks.NewAggregationCheck(regexp.MustCompile("^.+$"), "instance", false, checks.Warning),
			problems: []checks.Problem{
				{
					Fragment: "sum(sum(foo) without(foo)) without(bar)",
					Lines:    []int{2},
					Reporter: "promql/aggregate",
					Text:     `instance label should be removed when aggregating "^.+$" rules, use without(instance, ...)`,
					Severity: checks.Warning,
				},
				{
					Fragment: "sum without(foo) (foo)",
					Lines:    []int{2},
					Reporter: "promql/aggregate",
					Text:     `instance label should be removed when aggregating "^.+$" rules, use without(instance, ...)`,
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "passing most outer aggregation should continue further keep checks",
			content:     "- record: foo\n  expr: sum(sum(foo) without(job)) without(instance)\n",
			checker:     checks.NewAggregationCheck(regexp.MustCompile("^.+$"), "job", true, checks.Warning),
			problems: []checks.Problem{
				{
					Fragment: "sum without(job) (foo)",
					Lines:    []int{2},
					Reporter: "promql/aggregate",
					Text:     "job label is required and should be preserved when aggregating \"^.+$\" rules, remove job from without()",
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "Right hand side of AND is ignored",
			content:     "- record: foo\n  expr: foo AND on(instance) max(bar) without()\n",
			checker:     checks.NewAggregationCheck(regexp.MustCompile("^.+$"), "job", true, checks.Warning),
		},
		{
			description: "Left hand side of AND is checked",
			content:     "- record: foo\n  expr: max (foo) without(job) AND on(instance) bar\n",
			checker:     checks.NewAggregationCheck(regexp.MustCompile("^.+$"), "job", true, checks.Warning),
			problems: []checks.Problem{
				{
					Fragment: "max without(job) (foo)",
					Lines:    []int{2},
					Reporter: "promql/aggregate",
					Text:     "job label is required and should be preserved when aggregating \"^.+$\" rules, remove job from without()",
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "Right hand side of group_left() is ignored",
			content:     "- record: foo\n  expr: sum without(id) (foo) / on(type) group_left() sum without(job) (bar)\n",
			checker:     checks.NewAggregationCheck(regexp.MustCompile("^.+$"), "job", true, checks.Warning),
		},
		{
			description: "Left hand side of group_left() is checked",
			content:     "- record: foo\n  expr: sum without(job) (foo) / on(type) group_left() sum without(job) (bar)\n",
			checker:     checks.NewAggregationCheck(regexp.MustCompile("^.+$"), "job", true, checks.Warning),
			problems: []checks.Problem{
				{
					Fragment: "sum without(job) (foo)",
					Lines:    []int{2},
					Reporter: "promql/aggregate",
					Text:     "job label is required and should be preserved when aggregating \"^.+$\" rules, remove job from without()",
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "Left hand side of group_right() is ignored",
			content:     "- record: foo\n  expr: sum without(job) (foo) / on(type) group_right() sum without(id) (bar)\n",
			checker:     checks.NewAggregationCheck(regexp.MustCompile("^.+$"), "job", true, checks.Warning),
		},
		{
			description: "Right hand side of group_right() is checked",
			content:     "- record: foo\n  expr: sum without(job) (foo) / on(type) group_right() sum without(job) (bar)\n",
			checker:     checks.NewAggregationCheck(regexp.MustCompile("^.+$"), "job", true, checks.Warning),
			problems: []checks.Problem{
				{
					Fragment: "sum without(job) (bar)",
					Lines:    []int{2},
					Reporter: "promql/aggregate",
					Text:     "job label is required and should be preserved when aggregating \"^.+$\" rules, remove job from without()",
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "nested count",
			content:     "- record: foo\n  expr: count(count(bar) without ())\n",
			checker:     checks.NewAggregationCheck(regexp.MustCompile("^.+$"), "instance", false, checks.Warning),
		},

		{
			description: "ignores rules with syntax errors",
			content:     "- record: foo\n  expr: sum(foo) without(\n",
			checker:     checks.NewAggregationCheck(regexp.MustCompile("^.+$"), "job", true, checks.Warning),
		},
		{
			description: "name must match",
			content:     "- record: foo\n  expr: sum(foo) without(job)\n",
			checker:     checks.NewAggregationCheck(regexp.MustCompile("^bar$"), "job", true, checks.Warning),
		},
		{
			description: "uses label from labels map",
			content:     "- record: foo\n  expr: sum(foo) by(instance)\n  labels:\n    job: foo\n",
			checker:     checks.NewAggregationCheck(regexp.MustCompile("^.+$"), "job", true, checks.Warning),
		},
		{
			description: "must keep job label / warning",
			content:     "- record: foo\n  expr: sum(foo) by(instance)\n",
			checker:     checks.NewAggregationCheck(regexp.MustCompile("^.+$"), "job", true, checks.Warning),
			problems: []checks.Problem{
				{
					Fragment: "sum(foo) by(instance)",
					Lines:    []int{2},
					Reporter: "promql/aggregate",
					Text:     "job label is required and should be preserved when aggregating \"^.+$\" rules, use by(job, ...)",
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "must keep job label / bug",
			content:     "- record: foo\n  expr: sum(foo) by(instance)\n",
			checker:     checks.NewAggregationCheck(regexp.MustCompile("^.+$"), "job", true, checks.Bug),
			problems: []checks.Problem{
				{
					Fragment: "sum(foo) by(instance)",
					Lines:    []int{2},
					Reporter: "promql/aggregate",
					Text:     "job label is required and should be preserved when aggregating \"^.+$\" rules, use by(job, ...)",
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "must strip job label",
			content:     "- record: foo\n  expr: sum(foo) by(job)\n",
			checker:     checks.NewAggregationCheck(regexp.MustCompile("^.+$"), "job", false, checks.Warning),
			problems: []checks.Problem{
				{
					Fragment: "sum(foo) by(job)",
					Lines:    []int{2},
					Reporter: "promql/aggregate",
					Text:     "job label should be removed when aggregating \"^.+$\" rules, remove job from by()",
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "must strip job label / being stripped",
			content:     "- record: foo\n  expr: sum(foo) by(instance)\n",
			checker:     checks.NewAggregationCheck(regexp.MustCompile("^.+$"), "job", false, checks.Warning),
		},
		{
			description: "nested rule must keep job label",
			content:     "- record: foo\n  expr: sum(sum(foo) by(instance)) by(job)\n",
			checker:     checks.NewAggregationCheck(regexp.MustCompile("^.+$"), "job", true, checks.Warning),
			problems: []checks.Problem{
				{
					Fragment: "sum by(instance) (foo)",
					Lines:    []int{2},
					Reporter: "promql/aggregate",
					Text:     "job label is required and should be preserved when aggregating \"^.+$\" rules, use by(job, ...)",
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "Right hand side of AND is ignored",
			content:     "- record: foo\n  expr: foo AND on(instance) max(bar)\n",
			checker:     checks.NewAggregationCheck(regexp.MustCompile("^.+$"), "job", true, checks.Warning),
		},
		{
			description: "Left hand side of AND is checked",
			content:     "- record: foo\n  expr: max (foo) by(instance) AND on(instance) bar\n",
			checker:     checks.NewAggregationCheck(regexp.MustCompile("^.+$"), "job", true, checks.Warning),
			problems: []checks.Problem{
				{
					Fragment: "max by(instance) (foo)",
					Lines:    []int{2},
					Reporter: "promql/aggregate",
					Text:     "job label is required and should be preserved when aggregating \"^.+$\" rules, use by(job, ...)",
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "Right hand side of group_left() is ignored",
			content:     "- record: foo\n  expr: sum by(job) (foo) / on(type) group_left() sum by(type) (bar)\n",
			checker:     checks.NewAggregationCheck(regexp.MustCompile("^.+$"), "job", true, checks.Warning),
		},
		{
			description: "Left hand side of group_left() is checked",
			content:     "- record: foo\n  expr: sum by(type) (foo) / on(type) group_left() sum by(job) (bar)\n",
			checker:     checks.NewAggregationCheck(regexp.MustCompile("^.+$"), "job", true, checks.Warning),
			problems: []checks.Problem{
				{
					Fragment: "sum by(type) (foo)",
					Lines:    []int{2},
					Reporter: "promql/aggregate",
					Text:     "job label is required and should be preserved when aggregating \"^.+$\" rules, use by(job, ...)",
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "Left hand side of group_right() is ignored",
			content:     "- record: foo\n  expr: sum by(type) (foo) / on(type) group_right() sum by(job) (bar)\n",
			checker:     checks.NewAggregationCheck(regexp.MustCompile("^.+$"), "job", true, checks.Warning),
		},
		{
			description: "Right hand side of group_right() is checked",
			content:     "- record: foo\n  expr: sum by(job) (foo) / on(type) group_right() sum by(type) (bar)\n",
			checker:     checks.NewAggregationCheck(regexp.MustCompile("^.+$"), "job", true, checks.Warning),
			problems: []checks.Problem{
				{
					Fragment: "sum by(type) (bar)",
					Lines:    []int{2},
					Reporter: "promql/aggregate",
					Text:     "job label is required and should be preserved when aggregating \"^.+$\" rules, use by(job, ...)",
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "nested count",
			content:     "- record: foo\n  expr: count(count(bar) by (instance))\n",
			checker:     checks.NewAggregationCheck(regexp.MustCompile("^.+$"), "instance", false, checks.Warning),
		},
		{
			description: "nested count AND nested count",
			content:     "- record: foo\n  expr: count(count(bar) by (instance)) AND count(count(bar) by (instance))\n",
			checker:     checks.NewAggregationCheck(regexp.MustCompile("^.+$"), "instance", false, checks.Warning),
		},
		{
			description: "nested by(without())",
			content:     "- record: foo\n  expr: sum(sum(foo) by(instance)) without(job)\n",
			checker:     checks.NewAggregationCheck(regexp.MustCompile("^.+$"), "job", true, checks.Warning),
			problems: []checks.Problem{
				{
					Fragment: "sum(sum(foo) by(instance)) without(job)",
					Lines:    []int{2},
					Reporter: "promql/aggregate",
					Text:     `job label is required and should be preserved when aggregating "^.+$" rules, remove job from without()`,
					Severity: checks.Warning,
				},
				{
					Fragment: "sum by(instance) (foo)",
					Lines:    []int{2},
					Reporter: "promql/aggregate",
					Text:     `job label is required and should be preserved when aggregating "^.+$" rules, use by(job, ...)`,
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "nested by(without())",
			content:     "- record: foo\n  expr: sum(sum(foo) by(instance,job)) without(job)\n",
			checker:     checks.NewAggregationCheck(regexp.MustCompile("^.+$"), "job", true, checks.Warning),
			problems: []checks.Problem{
				{
					Fragment: "sum(sum(foo) by(instance,job)) without(job)",
					Lines:    []int{2},
					Reporter: "promql/aggregate",
					Text:     `job label is required and should be preserved when aggregating "^.+$" rules, remove job from without()`,
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "nested by(without())",
			content:     "- record: foo\n  expr: sum(sum(foo) by(instance)) without(instance)\n",
			checker:     checks.NewAggregationCheck(regexp.MustCompile("^.+$"), "job", true, checks.Warning),
			problems: []checks.Problem{
				{
					Fragment: "sum by(instance) (foo)",
					Lines:    []int{2},
					Reporter: "promql/aggregate",
					Text:     `job label is required and should be preserved when aggregating "^.+$" rules, use by(job, ...)`,
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "nested by(without())",
			content:     "- record: foo\n  expr: sum(sum(foo) by(instance)) without(job)\n",
			checker:     checks.NewAggregationCheck(regexp.MustCompile("^.+$"), "instance", false, checks.Warning),
			problems: []checks.Problem{
				{
					Fragment: "sum(sum(foo) by(instance)) without(job)",
					Lines:    []int{2},
					Reporter: "promql/aggregate",
					Text:     `instance label should be removed when aggregating "^.+$" rules, use without(instance, ...)`,
					Severity: checks.Warning,
				},
				{
					Fragment: "sum by(instance) (foo)",
					Lines:    []int{2},
					Reporter: "promql/aggregate",
					Text:     `instance label should be removed when aggregating "^.+$" rules, remove instance from by()`,
					Severity: checks.Warning,
				},
			},
		},
	}
	runTests(t, testCases)
}
