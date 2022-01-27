package checks_test

import (
	"testing"

	"github.com/cloudflare/pint/internal/checks"
)

func TestFragileCheck(t *testing.T) {
	text := "aggregation using without() can be fragile when used inside binary expression because both sides must have identical sets of labels to produce any results, adding or removing labels to metrics used here can easily break the query, consider aggregating using by() to ensure consistent labels"

	testCases := []checkTest{
		{
			description: "ignores syntax errors",
			content:     "- record: foo\n  expr: up ==\n",
			checker:     checks.NewFragileCheck(),
		},
		{
			description: "ignores simple comparison",
			content:     "- record: foo\n  expr: up == 0\n",
			checker:     checks.NewFragileCheck(),
		},
		{
			description: "ignores simple division",
			content:     "- record: foo\n  expr: foo / bar\n",
			checker:     checks.NewFragileCheck(),
		},
		{
			description: "ignores unless",
			content:     "- record: foo\n  expr: foo unless sum(bar) without(job)\n",
			checker:     checks.NewFragileCheck(),
		},
		{
			description: "ignores safe division",
			content:     "- record: foo\n  expr: foo / sum(bar)\n",
			checker:     checks.NewFragileCheck(),
		},
		{
			description: "warns about fragile division",
			content:     "- record: foo\n  expr: foo / sum(bar) without(job)\n",
			checker:     checks.NewFragileCheck(),
			problems: []checks.Problem{
				{
					Fragment: `foo / sum(bar) without(job)`,
					Lines:    []int{2},
					Reporter: "promql/fragile",
					Text:     text,
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "warns about fragile sum",
			content:     "- record: foo\n  expr: sum(foo) without(job) + sum(bar) without(job)\n",
			checker:     checks.NewFragileCheck(),
			problems: []checks.Problem{
				{
					Fragment: `sum(foo) without(job) + sum(bar) without(job)`,
					Lines:    []int{2},
					Reporter: "promql/fragile",
					Text:     text,
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "warns about fragile sum inside a condition",
			content:     "- alert: foo\n  expr: (sum(foo) without(job) + sum(bar) without(job)) > 1\n",
			checker:     checks.NewFragileCheck(),
			problems: []checks.Problem{
				{
					Fragment: `(sum(foo) without(job) + sum(bar) without(job)) > 1`,
					Lines:    []int{2},
					Reporter: "promql/fragile",
					Text:     text,
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "warns about fragile division inside a condition",
			content:     "- alert: foo\n  expr: (foo / sum(bar) without(job)) > 1\n",
			checker:     checks.NewFragileCheck(),
			problems: []checks.Problem{
				{
					Fragment: `foo / sum without(job) (bar)`,
					Lines:    []int{2},
					Reporter: "promql/fragile",
					Text:     text,
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "warns about fragile sum inside a complex rule",
			content:     "- alert: foo\n  expr: (sum(foo) without(job) + sum(bar)) > 1 unless sum(bob) without(job) < 10\n",
			checker:     checks.NewFragileCheck(),
			problems: []checks.Problem{
				{
					Fragment: `(sum without(job) (foo) + sum(bar)) > 1`,
					Lines:    []int{2},
					Reporter: "promql/fragile",
					Text:     text,
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "ignores safe division",
			content:     "- record: foo\n  expr: sum(foo) + sum(bar)\n",
			checker:     checks.NewFragileCheck(),
		},
		{
			description: "ignores division if source metric is the same",
			content:     "- record: foo\n  expr: sum(foo) without(bar) + sum(foo) without(bar)\n",
			checker:     checks.NewFragileCheck(),
		},
	}

	runTests(t, testCases)
}
