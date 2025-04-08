package checks_test

import (
	"testing"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/promapi"
)

func TestAggregationCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "ignores rules with syntax errors",
			content:     "- record: foo\n  expr: sum(foo) without(\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp(".+"),
					"job",
					true,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
		},
		{
			description: "name must match / recording",
			content:     "- record: foo\n  expr: sum(foo) without(job)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp("bar"),
					"job",
					true,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
		},
		{
			description: "name must match  /alerting",
			content:     "- alert: foo\n  expr: sum(foo) without(job)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp("bar"),
					"job",
					true,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
		},
		{
			description: "uses label from labels map / recording",
			content:     "- record: foo\n  expr: sum(foo) without(job)\n  labels:\n    job: foo\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp(".+"),
					"job",
					true,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
		},
		{
			description: "uses label from labels map / alerting",
			content:     "- alert: foo\n  expr: sum(foo) without(job)\n  labels:\n    job: foo\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp(".+"),
					"job",
					true,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
		},
		{
			description: "must keep job label / warning",
			content: `- record: foo
  expr: sum(foo) without(instance, job)
`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp(".+"),
					"job",
					true,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "must keep job label / bug",
			content:     "- record: foo\n  expr: sum(foo) without(instance, job)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp(".+"),
					"job",
					true,
					"some text",
					checks.Bug,
				)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "must strip job label",
			content: `- record: foo
  expr: sum(foo) without(instance)`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp(".+"),
					"job",
					false,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "must strip job label / being stripped",
			content:     "- record: foo\n  expr: sum(foo) without(instance,job)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp(".+"),
					"job",
					false,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
		},
		{
			description: "must strip job label / empty without",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp(".+"),
					"job",
					false,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
		},
		{
			description: "nested rule must keep job label",
			content: `- record: foo
  expr: sum(sum(foo) without(job)) by(job)`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp(".+"),
					"job",
					true,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "passing most outer aggregation should stop further strip checks",
			content:     "- record: foo\n  expr: sum(sum(foo) without(foo)) without(instance)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp(".+"),
					"instance",
					false,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
		},
		{
			description: "passing most outer aggregation should stop further checks",
			content: `- record: foo
  expr: sum(sum(foo) without(foo)) without(bar)`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp(".+"),
					"instance",
					false,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "passing most outer aggregation should continue further keep checks",
			content:     "- record: foo\n  expr: sum(sum(foo) without(job)) without(instance)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp(".+"),
					"job",
					true,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "Right hand side of AND is ignored",
			content:     "- record: foo\n  expr: foo AND on(instance) max(bar) without()\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp(".+"),
					"job",
					true,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
		},
		{
			description: "Left hand side of AND is checked",
			content:     "- record: foo\n  expr: max (foo) without(job) AND on(instance) bar\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp(".+"),
					"job",
					true,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "Right hand side of group_left() is ignored",
			content:     "- record: foo\n  expr: sum without(id) (foo) / on(type) group_left() sum without(job) (bar)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp(".+"),
					"job",
					true,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
		},
		{
			description: "Left hand side of group_left() is checked",
			content:     "- record: foo\n  expr: sum without(job) (foo) / on(type) group_left() sum without(job) (bar)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp(".+"),
					"job",
					true,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "Left hand side of group_right() is ignored",
			content:     "- record: foo\n  expr: sum without(job) (foo) / on(type) group_right() sum without(id) (bar)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp(".+"),
					"job",
					true,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
		},
		{
			description: "Right hand side of group_right() is checked",
			content:     "- record: foo\n  expr: sum without(job) (foo) / on(type) group_right() sum without(job) (bar)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp(".+"),
					"job",
					true,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "nested count",
			content:     "- record: foo\n  expr: count(count(bar) without ())\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp(".+"),
					"instance",
					false,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
		},

		{
			description: "ignores rules with syntax errors",
			content:     "- record: foo\n  expr: sum(foo) without(\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp(".+"),
					"job",
					true,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
		},
		{
			description: "name must match",
			content:     "- record: foo\n  expr: sum(foo) without(job)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp("bar"),
					"job",
					true,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
		},
		{
			description: "uses label from labels map",
			content:     "- record: foo\n  expr: sum(foo) by(instance)\n  labels:\n    job: foo\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp(".+"),
					"job",
					true,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
		},
		{
			description: "must keep job label / warning",
			content: `- record: foo
  expr: sum(foo) by(instance)`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp(".+"),
					"job",
					true,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "must keep job label / bug",
			content:     "- record: foo\n  expr: sum(foo) by(instance)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp(".+"),
					"job",
					true,
					"",
					checks.Bug,
				)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "must strip job label",
			content: `- record: foo
  expr: sum(foo) by(job)`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp(".+"),
					"job",
					false,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "must strip job label / being stripped",
			content:     "- record: foo\n  expr: sum(foo) by(instance)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp(".+"),
					"job",
					false,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
		},
		{
			description: "nested rule must keep job label",
			content: `- record: foo
  expr: sum(sum(foo) by(instance)) by(job)`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp(".+"),
					"job",
					true,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "Right hand side of AND is ignored",
			content:     "- record: foo\n  expr: foo AND on(instance) max(bar)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp(".+"),
					"job",
					true,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
		},
		{
			description: "Left hand side of AND is checked",
			content: `- record: foo
  expr: max (foo) by(instance) AND on(instance) bar`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp(".+"),
					"job",
					true,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "Right hand side of group_left() is ignored",
			content:     "- record: foo\n  expr: sum by(job) (foo) / on(type) group_left() sum by(type) (bar)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp(".+"),
					"job",
					true,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
		},
		{
			description: "Left hand side of group_left() is checked",
			content: `- record: foo
  expr: sum by(type) (foo) / on(type) group_left() sum by(job) (bar)`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp(".+"),
					"job",
					true,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "Left hand side of group_right() is ignored",
			content:     "- record: foo\n  expr: sum by(type) (foo) / on(type) group_right() sum by(job) (bar)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp(".+"),
					"job",
					true,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
		},
		{
			description: "Right hand side of group_right() is checked",
			content: `- record: foo
  expr: sum by(job) (foo) / on(type) group_right() sum by(type) (bar)`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp(".+"),
					"job",
					true,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "nested count",
			content:     "- record: foo\n  expr: count(count(bar) by (instance))\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp(".+"),
					"instance",
					false,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
		},
		{
			description: "nested count AND nested count",
			content:     "- record: foo\n  expr: count(count(bar) by (instance)) AND count(count(bar) by (instance))\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp(".+"),
					"instance",
					false,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
		},
		{
			description: "nested by(without())",
			content: `- record: foo
  expr: sum(sum(foo) by(instance)) without(job)`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp(".+"),
					"job",
					true,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "nested by(without())",
			content:     "- record: foo\n  expr: sum(sum(foo) by(instance,job)) without(job)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp(".+"),
					"job",
					true,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "nested by(without())",
			content: `- record: foo
  expr: sum(sum(foo) by(instance)) without(instance)`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp(".+"),
					"job",
					true,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "nested by(without())",
			content: `- record: foo
  expr: sum(sum(foo) by(instance)) without(job)
`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp(".+"),
					"instance",
					false,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "must keep job label / sum()",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp(".+"),
					"job",
					true,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "must keep job label / sum() by()",
			content:     "- record: foo\n  expr: sum(foo) by()\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp(".+"),
					"job",
					true,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "must strip job label / sum() without()",
			content:     "- record: foo\n  expr: sum(foo) without()\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewAggregationCheck(
					checks.MustTemplatedRegexp(".+"),
					"job",
					false,
					"",
					checks.Warning,
				)
			},
			prometheus: noProm,
			problems:   true,
		},
	}
	runTests(t, testCases)
}
