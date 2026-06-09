package checks_test

import (
	"testing"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/promapi"
)

func newNaNCheck(_ *promapi.FailoverGroup) checks.RuleChecker {
	return checks.NewNaNCheck()
}

func TestNaNCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "syntax error is ignored",
			content:     "- record: foo\n  expr: up ==\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
		},
		{
			description: "division without aggregation is allowed",
			content:     "- record: foo\n  expr: foo / bar\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
		},
		{
			description: "modulo without aggregation is allowed",
			content:     "- record: foo\n  expr: foo % bar\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
		},
		{
			description: "addition inside aggregation is allowed",
			content:     "- record: foo\n  expr: sum(foo + bar)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
		},
		{
			description: "multiplication inside aggregation is allowed",
			content:     "- record: foo\n  expr: sum(foo * bar)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
		},
		{
			description: "division inside sum is reported",
			content:     "- record: foo\n  expr: sum(foo / bar)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "modulo inside avg is reported",
			content:     "- record: foo\n  expr: avg(foo % bar)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "division inside min is safe",
			content:     "- record: foo\n  expr: min(foo / bar)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
		},
		{
			description: "division inside count is safe",
			content:     "- record: foo\n  expr: count(foo / bar)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
		},
		{
			description: "division inside count_values is safe",
			content:     "- record: foo\n  expr: count_values(\"val\", foo / bar)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
		},
		{
			description: "division inside group is safe",
			content:     "- record: foo\n  expr: group(foo / bar)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
		},
		{
			description: "division inside max is safe",
			content:     "- record: foo\n  expr: max(foo / bar)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
		},
		{
			description: "division inside stddev is reported",
			content:     "- record: foo\n  expr: stddev(foo / bar)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "division inside stdvar is reported",
			content:     "- record: foo\n  expr: stdvar(foo / bar)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "division inside topk is safe",
			content:     "- record: foo\n  expr: topk(5, foo / bar)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
		},
		{
			description: "division inside bottomk is safe",
			content:     "- record: foo\n  expr: bottomk(5, foo / bar)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
		},
		{
			description: "division inside quantile is safe",
			content:     "- record: foo\n  expr: quantile(0.9, foo / bar)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
		},
		{
			description: "division inside limitk is safe",
			content:     "- record: foo\n  expr: limitk(5, foo / bar)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
		},
		{
			description: "division inside limit_ratio is safe",
			content:     "- record: foo\n  expr: limit_ratio(0.5, foo / bar)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
		},
		{
			description: "mixed safe and unsafe aggregations only reports unsafe",
			content:     "- record: foo\n  expr: sum(foo / bar) + count(foo / bar)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "nested sum wrapping division is reported",
			content:     "- record: foo\n  expr: sum(foo / bar) / sum(baz)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "division between aggregation results is allowed",
			content:     "- record: foo\n  expr: sum(foo) / sum(bar)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
		},
		{
			description: "rate divided by rate inside sum is reported",
			content:     "- record: foo\n  expr: sum(rate(foo[5m]) / rate(bar[5m]))\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "recording rule producing NaN inside aggregation is reported",
			content:     "- record: output\n  expr: sum(rec_rule)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			problems:    true,
			entries:     mustParseContent("- record: rec_rule\n  expr: foo / bar\n"),
		},
		{
			description: "recording rule without NaN inside aggregation is allowed",
			content:     "- record: output\n  expr: sum(rec_rule)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			entries:     mustParseContent("- record: rec_rule\n  expr: foo + bar\n"),
		},
		{
			description: "removed recording rule with NaN is ignored",
			content:     "- record: output\n  expr: sum(rec_rule)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			entries: parseWithState(
				"- record: rec_rule\n  expr: foo / bar\n",
				discovery.Removed,
			),
		},
		{
			description: "transitive recording rule producing NaN inside aggregation is reported",
			content:     "- record: output\n  expr: sum(rec_rule2)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			problems:    true,
			entries: mustParseContent(
				"- record: rec_rule1\n  expr: foo / bar\n" +
					"- record: rec_rule2\n  expr: rec_rule1 + baz\n",
			),
		},
		{
			description: "transitive safe recording rule chain inside aggregation is allowed",
			content:     "- record: output\n  expr: sum(rec_rule2)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			entries: mustParseContent(
				"- record: rec_rule1\n  expr: foo + bar\n" +
					"- record: rec_rule2\n  expr: rec_rule1 + baz\n",
			),
		},
		{
			description: "transitive unsafe branch among safe recording rules inside aggregation is reported",
			content:     "- record: output\n  expr: sum(rec_rule3)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			problems:    true,
			entries: mustParseContent(
				"- record: rec_rule1\n  expr: foo + bar\n" +
					"- record: rec_rule2\n  expr: baz / qux\n" +
					"- record: rec_rule3\n  expr: rec_rule1 + rec_rule2\n",
			),
		},
		{
			description: "cyclic safe recording rule chain inside aggregation is allowed",
			content:     "- record: output\n  expr: sum(rec_rule1)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			entries: mustParseContent(
				"- record: rec_rule1\n  expr: rec_rule2 + foo\n" +
					"- record: rec_rule2\n  expr: rec_rule1 + bar\n",
			),
		},
		{
			description: "cyclic recording rule chain with transitive unsafe dependency inside aggregation is reported",
			content:     "- record: output\n  expr: sum(rec_rule1)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			problems:    true,
			entries: mustParseContent(
				"- record: rec_rule1\n  expr: rec_rule2 + foo\n" +
					"- record: rec_rule2\n  expr: rec_rule3 + bar\n" +
					"- record: rec_rule3\n  expr: rec_rule1 + baz / qux\n",
			),
		},
		{
			description: "alerting rule with division inside sum is reported",
			content:     "- alert: foo\n  expr: sum(foo / bar) > 0\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "division with guarded divisor using greater than is allowed",
			content:     "- record: foo\n  expr: sum(foo / (bar > 0))\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
		},
		{
			description: "division with guarded divisor using not equal is allowed",
			content:     "- record: foo\n  expr: sum(foo / (bar != 0))\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
		},
		{
			description: "modulo with guarded divisor is allowed",
			content:     "- record: foo\n  expr: sum(foo % (bar > 0))\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
		},
		{
			description: "guarded recording rule inside aggregation is allowed",
			content:     "- record: output\n  expr: sum(rec_rule)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			entries:     mustParseContent("- record: rec_rule\n  expr: foo / (bar > 0)\n"),
		},
		{
			description: "division with bool modifier on divisor is still reported",
			content:     "- record: foo\n  expr: sum(foo / (bar > bool 0))\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "division with greater or equal guard on divisor is allowed",
			content:     "- record: foo\n  expr: sum(foo / (bar >= 1))\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
		},
		{
			description: "scalar division by constant inside sum is reported",
			content:     "- record: foo\n  expr: sum(1 / bar)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "division by non-zero scalar inside sum is allowed",
			content:     "- record: foo\n  expr: sum(foo / 2)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
		},
		{
			description: "modulo by non-zero scalar inside sum is allowed",
			content:     "- record: foo\n  expr: sum(foo % 2)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
		},
		{
			description: "division by zero scalar inside sum is reported",
			content:     "- record: foo\n  expr: sum(foo / 0)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "modulo by zero scalar inside sum is reported",
			content:     "- record: foo\n  expr: sum(foo % 0)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "division by negative scalar inside sum is allowed",
			content:     "- record: foo\n  expr: sum(foo / -2)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
		},
		{
			description: "division by vector call with non-zero arg inside sum is allowed",
			content:     "- record: foo\n  expr: sum(foo / vector(2))\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
		},
		{
			description: "division by vector call with zero arg inside sum is reported",
			content:     "- record: foo\n  expr: sum(foo / vector(0))\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "division by clamp_min with positive floor inside sum is allowed",
			content:     "- record: foo\n  expr: sum(foo / clamp_min(bar, 1))\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
		},
		{
			description: "modulo by clamp_min with positive floor inside sum is allowed",
			content:     "- record: foo\n  expr: sum(foo % clamp_min(bar, 1))\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
		},
		{
			description: "division by clamp_min with zero floor inside sum is reported",
			content:     "- record: foo\n  expr: sum(foo / clamp_min(bar, 0))\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "division by clamp_min with negative floor inside sum is reported",
			content:     "- record: foo\n  expr: sum(foo / clamp_min(bar, -1))\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "division by clamp_min with non-constant scalar floor inside sum is reported",
			content:     "- record: foo\n  expr: sum(foo / clamp_min(bar, scalar(baz)))\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "scalar division by clamp_min with positive floor inside sum is allowed",
			content:     "- record: foo\n  expr: sum(1 / clamp_min(bar, 1))\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
		},
		{
			description: "division by clamp_max with negative ceiling inside sum is allowed",
			content:     "- record: foo\n  expr: sum(foo / clamp_max(bar, -1))\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
		},
		{
			description: "modulo by clamp_max with negative ceiling inside sum is allowed",
			content:     "- record: foo\n  expr: sum(foo % clamp_max(bar, -1))\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
		},
		{
			description: "division by clamp_max with zero ceiling inside sum is reported",
			content:     "- record: foo\n  expr: sum(foo / clamp_max(bar, 0))\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "division by clamp_max with positive ceiling inside sum is reported",
			content:     "- record: foo\n  expr: sum(foo / clamp_max(bar, 1))\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "division by clamp_max with non-constant scalar ceiling inside sum is reported",
			content:     "- record: foo\n  expr: sum(foo / clamp_max(bar, scalar(baz)))\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "scalar division by clamp_max with negative ceiling inside sum is allowed",
			content:     "- record: foo\n  expr: sum(1 / clamp_max(bar, -1))\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
		},
		{
			description: "division by clamp with positive range inside sum is allowed",
			content:     "- record: foo\n  expr: sum(foo / clamp(bar, 1, 10))\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
		},
		{
			description: "division by clamp with negative range inside sum is allowed",
			content:     "- record: foo\n  expr: sum(foo / clamp(bar, -10, -1))\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
		},
		{
			description: "division by clamp with range crossing zero inside sum is reported",
			content:     "- record: foo\n  expr: sum(foo / clamp(bar, -1, 1))\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "division by clamp with zero lower bound inside sum is reported",
			content:     "- record: foo\n  expr: sum(foo / clamp(bar, 0, 1))\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "division by clamp with zero upper bound inside sum is reported",
			content:     "- record: foo\n  expr: sum(foo / clamp(bar, -1, 0))\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "division by clamp with non-constant scalar bounds inside sum is reported",
			content:     "- record: foo\n  expr: sum(foo / clamp(bar, scalar(lo), scalar(hi)))\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "scalar division by clamp with positive range inside sum is allowed",
			content:     "- record: foo\n  expr: sum(1 / clamp(bar, 1, 10))\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
		},
		{
			description: "division by parenthesized scalar inside sum is allowed",
			content:     "- record: foo\n  expr: sum(foo / (2))\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
		},
		{
			description: "deeply nested parens around division with scalar is allowed",
			content:     "- record: foo\n  expr: sum(((foo) / 2))\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
		},
		{
			description: "guard on numerator does not protect divisor",
			content:     "- record: foo\n  expr: sum((foo > 0) / bar)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "division by equality guard keeping only zeros is reported",
			content:     "- record: foo\n  expr: sum(foo / (bar == 0))\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "division by less-than-or-equal guard keeping zeros is reported",
			content:     "- record: foo\n  expr: sum(foo / (bar <= 0))\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "division by greater-than-or-equal guard keeping zeros is reported",
			content:     "- record: foo\n  expr: sum(foo / (bar >= 0))\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "division by guard with unknown value is reported",
			content:     "- record: foo\n  expr: sum(foo / (bar > baz))\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "division by greater-than guard with negative threshold keeping zeros is reported",
			content:     "- record: foo\n  expr: sum(foo / (bar > -1))\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "division by less-than guard excluding zeros is allowed",
			content:     "- record: foo\n  expr: sum(foo / (bar < 0))\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
		},
		{
			description: "division by less-than guard keeping zeros is reported",
			content:     "- record: foo\n  expr: sum(foo / (bar < 1))\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "division by less-than-or-equal guard excluding zeros is allowed",
			content:     "- record: foo\n  expr: sum(foo / (bar <= -1))\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
		},
		{
			description: "division by not-equal guard with non-zero threshold keeping zeros is reported",
			content:     "- record: foo\n  expr: sum(foo / (bar != 1))\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "recording rule with division used in addition inside aggregation is reported",
			content:     "- record: foo\n  expr: sum(rec_rule + bar)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			problems:    true,
			entries:     mustParseContent("- record: rec_rule\n  expr: foo / bar\n"),
		},
		{
			description: "two safe recording rules added inside aggregation are allowed",
			content:     "- record: foo\n  expr: sum(rec_rule1 + rec_rule2)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			entries: mustParseContent(
				"- record: rec_rule1\n  expr: foo + bar\n" +
					"- record: rec_rule2\n  expr: baz * qux\n",
			),
		},
		{
			description: "unsafe LHS recording rule added to safe RHS recording rule inside aggregation is reported",
			content:     "- record: foo\n  expr: sum(rec_rule1 + rec_rule2)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			problems:    true,
			entries: mustParseContent(
				"- record: rec_rule1\n  expr: foo / bar\n" +
					"- record: rec_rule2\n  expr: baz * qux\n",
			),
		},
		{
			description: "safe LHS recording rule added to unsafe RHS recording rule inside aggregation is reported",
			content:     "- record: foo\n  expr: sum(rec_rule1 + rec_rule2)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			problems:    true,
			entries: mustParseContent(
				"- record: rec_rule1\n  expr: foo + bar\n" +
					"- record: rec_rule2\n  expr: baz / qux\n",
			),
		},
		{
			description: "two unsafe recording rules added inside aggregation is reported",
			content:     "- record: foo\n  expr: sum(rec_rule1 + rec_rule2)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			problems:    true,
			entries: mustParseContent(
				"- record: rec_rule1\n  expr: foo / bar\n" +
					"- record: rec_rule2\n  expr: baz / qux\n",
			),
		},
		{
			description: "two safe recording rules divided inside aggregation is reported",
			content:     "- record: foo\n  expr: sum(rec_rule1 / rec_rule2)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			problems:    true,
			entries: mustParseContent(
				"- record: rec_rule1\n  expr: foo + bar\n" +
					"- record: rec_rule2\n  expr: baz * qux\n",
			),
		},
		{
			description: "multiplication and division in same aggregation reports division",
			content:     "- record: foo\n  expr: sum(foo * bar / baz)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "guarded division followed by unguarded division in same aggregation reports unguarded",
			content:     "- record: foo\n  expr: sum(foo / (bar > 0) * baz / qux)\n",
			checker:     newNaNCheck,
			prometheus:  noProm,
			problems:    true,
		},
	}

	runTests(t, testCases)
}
