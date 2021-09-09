package checks_test

import (
	"regexp"
	"testing"

	"github.com/cloudflare/pint/internal/checks"
)

func TestWithoutCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "ignores rules with syntax errors",
			content:     "- record: foo\n  expr: sum(foo) without(\n",
			checker:     checks.NewWithoutCheck(regexp.MustCompile("^.+$"), "job", true, checks.Warning),
		},
		{
			description: "name must match",
			content:     "- record: foo\n  expr: sum(foo) without(job)\n",
			checker:     checks.NewWithoutCheck(regexp.MustCompile("^bar$"), "job", true, checks.Warning),
		},
		{
			description: "uses label from labels map",
			content:     "- record: foo\n  expr: sum(foo) without(job)\n  labels:\n    job: foo\n",
			checker:     checks.NewWithoutCheck(regexp.MustCompile("^.+$"), "job", true, checks.Warning),
		},
		{
			description: "must keep job label / warning",
			content:     "- record: foo\n  expr: sum(foo) without(instance, job)\n",
			checker:     checks.NewWithoutCheck(regexp.MustCompile("^.+$"), "job", true, checks.Warning),
			problems: []checks.Problem{
				{
					Fragment: "sum(foo) without(instance, job)",
					Lines:    []int{2},
					Reporter: "promql/without",
					Text:     "job label is required and should be preserved when aggregating \"^.+$\" rules, remove job from without()",
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "must keep job label / bug",
			content:     "- record: foo\n  expr: sum(foo) without(instance, job)\n",
			checker:     checks.NewWithoutCheck(regexp.MustCompile("^.+$"), "job", true, checks.Bug),
			problems: []checks.Problem{
				{
					Fragment: "sum(foo) without(instance, job)",
					Lines:    []int{2},
					Reporter: "promql/without",
					Text:     "job label is required and should be preserved when aggregating \"^.+$\" rules, remove job from without()",
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "must strip job label",
			content:     "- record: foo\n  expr: sum(foo) without(instance)\n",
			checker:     checks.NewWithoutCheck(regexp.MustCompile("^.+$"), "job", false, checks.Warning),
			problems: []checks.Problem{
				{
					Fragment: "sum(foo) without(instance)",
					Lines:    []int{2},
					Reporter: "promql/without",
					Text:     "job label should be removed when aggregating \"^.+$\" rules, use without(job, ...)",
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "must strip job label / being stripped",
			content:     "- record: foo\n  expr: sum(foo) without(instance,job)\n",
			checker:     checks.NewWithoutCheck(regexp.MustCompile("^.+$"), "job", false, checks.Warning),
		},
		{
			description: "must strip job label / empty without",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker:     checks.NewWithoutCheck(regexp.MustCompile("^.+$"), "job", false, checks.Warning),
		},
		{
			description: "nested rule must keep job label",
			content:     "- record: foo\n  expr: sum(sum(foo) without(job)) by(job)\n",
			checker:     checks.NewWithoutCheck(regexp.MustCompile("^.+$"), "job", true, checks.Warning),
			problems: []checks.Problem{
				{
					Fragment: "sum without(job) (foo)",
					Lines:    []int{2},
					Reporter: "promql/without",
					Text:     "job label is required and should be preserved when aggregating \"^.+$\" rules, remove job from without()",
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "passing most outer aggregation should stop further strip checks",
			content:     "- record: foo\n  expr: sum(sum(foo) without(foo)) without(instance)\n",
			checker:     checks.NewWithoutCheck(regexp.MustCompile("^.+$"), "instance", false, checks.Warning),
		},
		{
			description: "passing most outer aggregation should stop further checks",
			content:     "- record: foo\n  expr: sum(sum(foo) without(foo)) without(bar)\n",
			checker:     checks.NewWithoutCheck(regexp.MustCompile("^.+$"), "instance", false, checks.Warning),
			problems: []checks.Problem{
				{
					Fragment: "sum(sum(foo) without(foo)) without(bar)",
					Lines:    []int{2},
					Reporter: "promql/without",
					Text:     `instance label should be removed when aggregating "^.+$" rules, use without(instance, ...)`,
					Severity: checks.Warning,
				},
				{
					Fragment: "sum without(foo) (foo)",
					Lines:    []int{2},
					Reporter: "promql/without",
					Text:     `instance label should be removed when aggregating "^.+$" rules, use without(instance, ...)`,
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "passing most outer aggregation should continue further keep checks",
			content:     "- record: foo\n  expr: sum(sum(foo) without(job)) without(instance)\n",
			checker:     checks.NewWithoutCheck(regexp.MustCompile("^.+$"), "job", true, checks.Warning),
			problems: []checks.Problem{
				{
					Fragment: "sum without(job) (foo)",
					Lines:    []int{2},
					Reporter: "promql/without",
					Text:     "job label is required and should be preserved when aggregating \"^.+$\" rules, remove job from without()",
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "Right hand side of AND is ignored",
			content:     "- record: foo\n  expr: foo AND on(instance) max(bar) without()\n",
			checker:     checks.NewWithoutCheck(regexp.MustCompile("^.+$"), "job", true, checks.Warning),
		},
		{
			description: "Left hand side of AND is checked",
			content:     "- record: foo\n  expr: max (foo) without(job) AND on(instance) bar\n",
			checker:     checks.NewWithoutCheck(regexp.MustCompile("^.+$"), "job", true, checks.Warning),
			problems: []checks.Problem{
				{
					Fragment: "max without(job) (foo)",
					Lines:    []int{2},
					Reporter: "promql/without",
					Text:     "job label is required and should be preserved when aggregating \"^.+$\" rules, remove job from without()",
					Severity: checks.Warning,
				},
			},
		},
	}
	runTests(t, testCases)
}
