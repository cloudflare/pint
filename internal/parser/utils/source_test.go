package utils_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/parser/utils"

	"github.com/prometheus/prometheus/model/labels"
	promParser "github.com/prometheus/prometheus/promql/parser"
	"github.com/prometheus/prometheus/promql/parser/posrange"
)

func mustParse[T any](t *testing.T, s string, offset int) T {
	m, err := promParser.ParseExpr(strings.Repeat(" ", offset) + s)
	require.NoErrorf(t, err, "failed to parse vector selector: %s", s)
	n, ok := m.(T)
	require.True(t, ok, "failed to convert %q to %t\n", s, n)
	return n
}

func TestLabelsSource(t *testing.T) {
	type testCaseT struct {
		expr   string
		output []utils.Source
	}

	testCases := []testCaseT{
		{
			expr: "1",
			output: []utils.Source{
				{
					Type:           utils.NumberSource,
					Returns:        promParser.ValueTypeScalar,
					FixedLabels:    true,
					AlwaysReturns:  true,
					ReturnedNumber: 1,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "This query returns a number value with no labels.",
						},
					},
				},
			},
		},
		{
			expr: "1 / 5",
			output: []utils.Source{
				{
					Type:           utils.NumberSource,
					Returns:        promParser.ValueTypeScalar,
					FixedLabels:    true,
					AlwaysReturns:  true,
					ReturnedNumber: 0.2,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "This query returns a number value with no labels.",
						},
					},
				},
			},
		},
		{
			expr: "(2 ^ 5) == bool 5",
			output: []utils.Source{
				{
					Type:           utils.NumberSource,
					Returns:        promParser.ValueTypeScalar,
					FixedLabels:    true,
					AlwaysReturns:  true,
					IsDead:         true,
					IsDeadReason:   "this query always evaluates to `32 == 5` which is not possible, so it will never return anything",
					ReturnedNumber: 32,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "This query returns a number value with no labels.",
						},
					},
					IsConditional: true,
				},
			},
		},
		{
			expr: "(2 ^ 5 + 11) % 5 <= bool 2",
			output: []utils.Source{
				{
					Type:           utils.NumberSource,
					Returns:        promParser.ValueTypeScalar,
					FixedLabels:    true,
					AlwaysReturns:  true,
					IsDead:         true,
					IsDeadReason:   "this query always evaluates to `3 <= 2` which is not possible, so it will never return anything",
					ReturnedNumber: 3,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "This query returns a number value with no labels.",
						},
					},
					IsConditional: true,
				},
			},
		},
		{
			expr: "(2 ^ 5 + 11) % 5 >= bool 20",
			output: []utils.Source{
				{
					Type:           utils.NumberSource,
					Returns:        promParser.ValueTypeScalar,
					FixedLabels:    true,
					AlwaysReturns:  true,
					IsDead:         true,
					IsDeadReason:   "this query always evaluates to `3 >= 20` which is not possible, so it will never return anything",
					ReturnedNumber: 3,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "This query returns a number value with no labels.",
						},
					},
					IsConditional: true,
				},
			},
		},
		{
			expr: "(2 ^ 5 + 11) % 5 <= bool 3",
			output: []utils.Source{
				{
					Type:           utils.NumberSource,
					Returns:        promParser.ValueTypeScalar,
					FixedLabels:    true,
					AlwaysReturns:  true,
					ReturnedNumber: 3,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "This query returns a number value with no labels.",
						},
					},
					IsConditional: true,
				},
			},
		},
		{
			expr: "(2 ^ 5 + 11) % 5 < bool 1",
			output: []utils.Source{
				{
					Type:           utils.NumberSource,
					Returns:        promParser.ValueTypeScalar,
					FixedLabels:    true,
					AlwaysReturns:  true,
					IsDead:         true,
					IsDeadReason:   "this query always evaluates to `3 < 1` which is not possible, so it will never return anything",
					ReturnedNumber: 3,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "This query returns a number value with no labels.",
						},
					},
					IsConditional: true,
				},
			},
		},
		{
			expr: "20 - 15 < bool 1",
			output: []utils.Source{
				{
					Type:           utils.NumberSource,
					Returns:        promParser.ValueTypeScalar,
					FixedLabels:    true,
					AlwaysReturns:  true,
					IsDead:         true,
					IsDeadReason:   "this query always evaluates to `5 < 1` which is not possible, so it will never return anything",
					ReturnedNumber: 5,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "This query returns a number value with no labels.",
						},
					},
					IsConditional: true,
				},
			},
		},
		{
			expr: "2 * 5",
			output: []utils.Source{
				{
					Type:           utils.NumberSource,
					Returns:        promParser.ValueTypeScalar,
					FixedLabels:    true,
					AlwaysReturns:  true,
					ReturnedNumber: 10,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "This query returns a number value with no labels.",
						},
					},
				},
			},
		},
		{
			expr: "(foo or bar) * 5",
			output: []utils.Source{
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardManyToMany.String(),
					Selector:  mustParse[*promParser.VectorSelector](t, "foo", 1),
				},
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardManyToMany.String(),
					Selector:  mustParse[*promParser.VectorSelector](t, "bar", 8),
				},
			},
		},
		{
			expr: "(foo or vector(2)) * 5",
			output: []utils.Source{
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardManyToMany.String(),
					Selector:  mustParse[*promParser.VectorSelector](t, "foo", 1),
				},
				{
					Type:           utils.FuncSource,
					Returns:        promParser.ValueTypeVector,
					Operation:      "vector",
					FixedLabels:    true,
					AlwaysReturns:  true,
					ReturnedNumber: 10,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Calling `vector()` will return a vector value with no labels.",
						},
					},
					Call: mustParse[*promParser.Call](t, "vector(2)", 8),
				},
			},
		},
		{
			expr: "(foo or vector(5)) * (vector(2) or bar)",
			output: []utils.Source{
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardManyToMany.String(),
					Selector:  mustParse[*promParser.VectorSelector](t, "foo", 1),
					Joins: []utils.Join{
						{
							Src: utils.Source{
								Type:           utils.FuncSource,
								Returns:        promParser.ValueTypeVector,
								Operation:      "vector",
								FixedLabels:    true,
								AlwaysReturns:  true,
								ReturnedNumber: 2,
								Call:           mustParse[*promParser.Call](t, "vector(2)", 22),
								ExcludeReason: map[string]utils.ExcludedLabel{
									"": {
										Reason: "Calling `vector()` will return a vector value with no labels.",
									},
								},
							},
						},
						{
							Src: utils.Source{
								Type:         utils.SelectorSource,
								Returns:      promParser.ValueTypeVector,
								Operation:    promParser.CardManyToMany.String(),
								Selector:     mustParse[*promParser.VectorSelector](t, "bar", 35),
								IsDead:       true,
								IsDeadReason: "the left hand side always returs something and so the right hand side is never used",
							},
						},
					},
				},
				{
					Type:           utils.FuncSource,
					Returns:        promParser.ValueTypeVector,
					Operation:      "vector",
					FixedLabels:    true,
					AlwaysReturns:  true,
					ReturnedNumber: 10,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Calling `vector()` will return a vector value with no labels.",
						},
					},
					Call: mustParse[*promParser.Call](t, "vector(5)", 8),
					Joins: []utils.Join{
						{
							Src: utils.Source{
								Type:           utils.FuncSource,
								Returns:        promParser.ValueTypeVector,
								Operation:      "vector",
								FixedLabels:    true,
								AlwaysReturns:  true,
								ReturnedNumber: 2,
								Call:           mustParse[*promParser.Call](t, "vector(2)", 22),
								ExcludeReason: map[string]utils.ExcludedLabel{
									"": {
										Reason: "Calling `vector()` will return a vector value with no labels.",
									},
								},
							},
						},
						{
							Src: utils.Source{
								Type:         utils.SelectorSource,
								Returns:      promParser.ValueTypeVector,
								Operation:    promParser.CardManyToMany.String(),
								Selector:     mustParse[*promParser.VectorSelector](t, "bar", 35),
								IsDead:       true,
								IsDeadReason: "the left hand side always returs something and so the right hand side is never used",
							},
						},
					},
				},
			},
		},
		{
			expr: `1 > bool 0`,
			output: []utils.Source{
				{
					Type:           utils.NumberSource,
					Returns:        promParser.ValueTypeScalar,
					FixedLabels:    true,
					AlwaysReturns:  true,
					ReturnedNumber: 1,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "This query returns a number value with no labels.",
						},
					},
					IsConditional: true,
				},
			},
		},
		{
			expr: `20 > bool 10`,
			output: []utils.Source{
				{
					Type:           utils.NumberSource,
					Returns:        promParser.ValueTypeScalar,
					FixedLabels:    true,
					AlwaysReturns:  true,
					ReturnedNumber: 20,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "This query returns a number value with no labels.",
						},
					},
					IsConditional: true,
				},
			},
		},
		{
			expr: `"test"`,
			output: []utils.Source{
				{
					Type:          utils.StringSource,
					Returns:       promParser.ValueTypeString,
					FixedLabels:   true,
					AlwaysReturns: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "This query returns a string value with no labels.",
						},
					},
				},
			},
		},
		{
			expr: "foo",
			output: []utils.Source{
				{
					Type:     utils.SelectorSource,
					Returns:  promParser.ValueTypeVector,
					Selector: mustParse[*promParser.VectorSelector](t, "foo", 0),
				},
			},
		},
		{
			expr: "foo offset 5m",
			output: []utils.Source{
				{
					Type:     utils.SelectorSource,
					Returns:  promParser.ValueTypeVector,
					Selector: mustParse[*promParser.VectorSelector](t, "foo offset 5m", 0),
				},
			},
		},
		{
			expr: `foo{job="bar"}`,
			output: []utils.Source{
				{
					Type:             utils.SelectorSource,
					Returns:          promParser.ValueTypeVector,
					Selector:         mustParse[*promParser.VectorSelector](t, `foo{job="bar"}`, 0),
					GuaranteedLabels: []string{"job"},
				},
			},
		},
		{
			expr: `foo{job=""}`,
			output: []utils.Source{
				{
					Type:           utils.SelectorSource,
					Returns:        promParser.ValueTypeVector,
					Selector:       mustParse[*promParser.VectorSelector](t, `foo{job=""}`, 0),
					ExcludedLabels: []string{"job"},
					ExcludeReason: map[string]utils.ExcludedLabel{
						"job": {
							Reason: "Query uses `{job=\"\"}` selector which will filter out any time series with the `job` label set.",
						},
					},
				},
			},
		},
		{
			expr: `foo{job="bar"} or bar{job="foo"}`,
			output: []utils.Source{
				{
					Type:             utils.SelectorSource,
					Returns:          promParser.ValueTypeVector,
					Selector:         mustParse[*promParser.VectorSelector](t, `foo{job="bar"}`, 0),
					Operation:        promParser.CardManyToMany.String(),
					GuaranteedLabels: []string{"job"},
				},
				{
					Type:             utils.SelectorSource,
					Returns:          promParser.ValueTypeVector,
					Selector:         mustParse[*promParser.VectorSelector](t, `bar{job="foo"}`, 18),
					Operation:        promParser.CardManyToMany.String(),
					GuaranteedLabels: []string{"job"},
				},
			},
		},
		{
			expr: `foo{a="bar"} or bar{b="foo"}`,
			output: []utils.Source{
				{
					Type:             utils.SelectorSource,
					Returns:          promParser.ValueTypeVector,
					Selector:         mustParse[*promParser.VectorSelector](t, `foo{a="bar"}`, 0),
					Operation:        promParser.CardManyToMany.String(),
					GuaranteedLabels: []string{"a"},
				},
				{
					Type:             utils.SelectorSource,
					Returns:          promParser.ValueTypeVector,
					Selector:         mustParse[*promParser.VectorSelector](t, `bar{b="foo"}`, 16),
					Operation:        promParser.CardManyToMany.String(),
					GuaranteedLabels: []string{"b"},
				},
			},
		},
		{
			expr: "foo[5m]",
			output: []utils.Source{
				{
					Type:     utils.SelectorSource,
					Returns:  promParser.ValueTypeVector, // FIXME Matrix ??
					Selector: mustParse[*promParser.VectorSelector](t, "foo", 0),
				},
			},
		},
		{
			expr: "prometheus_build_info[2m:1m]",
			output: []utils.Source{
				{
					Type:     utils.SelectorSource,
					Returns:  promParser.ValueTypeVector,
					Selector: mustParse[*promParser.VectorSelector](t, "prometheus_build_info", 0),
				},
			},
		},
		{
			expr: "deriv(rate(distance_covered_meters_total[1m])[5m:1m])",
			output: []utils.Source{
				{
					Type:      utils.FuncSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "deriv",
					Selector:  mustParse[*promParser.VectorSelector](t, "distance_covered_meters_total", 11),
					Call:      mustParse[*promParser.Call](t, "deriv(rate(distance_covered_meters_total[1m])[5m:1m])", 0),
				},
			},
		},
		{
			expr: "foo - 1",
			output: []utils.Source{
				{
					Type:     utils.SelectorSource,
					Returns:  promParser.ValueTypeVector,
					Selector: mustParse[*promParser.VectorSelector](t, "foo", 0),
				},
			},
		},
		{
			expr: "foo / 5",
			output: []utils.Source{
				{
					Type:     utils.SelectorSource,
					Returns:  promParser.ValueTypeVector,
					Selector: mustParse[*promParser.VectorSelector](t, "foo", 0),
				},
			},
		},
		{
			expr: "-foo",
			output: []utils.Source{
				{
					Type:     utils.SelectorSource,
					Returns:  promParser.ValueTypeVector,
					Selector: mustParse[*promParser.VectorSelector](t, "foo", 1),
				},
			},
		},
		{
			expr: `sum(foo{job="myjob"})`,
			output: []utils.Source{
				{
					Type:        utils.AggregateSource,
					Returns:     promParser.ValueTypeVector,
					Operation:   "sum",
					Selector:    mustParse[*promParser.VectorSelector](t, `foo{job="myjob"}`, 4),
					Aggregation: mustParse[*promParser.AggregateExpr](t, `sum(foo{job="myjob"})`, 0),
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Query is using aggregation that removes all labels.",
						},
					},
				},
			},
		},
		{
			expr: `sum(count(foo{job="myjob"}) by(instance))`,
			output: []utils.Source{
				{
					Type:        utils.AggregateSource,
					Returns:     promParser.ValueTypeVector,
					Operation:   "sum",
					Selector:    mustParse[*promParser.VectorSelector](t, `foo{job="myjob"}`, 10),
					Aggregation: mustParse[*promParser.AggregateExpr](t, `sum(count(foo{job="myjob"}) by(instance))`, 0),
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Query is using aggregation that removes all labels.",
						},
					},
				},
			},
		},
		{
			expr: `sum(foo{job="myjob"}) > 20`,
			output: []utils.Source{
				{
					Type:        utils.AggregateSource,
					Returns:     promParser.ValueTypeVector,
					Operation:   "sum",
					Selector:    mustParse[*promParser.VectorSelector](t, `foo{job="myjob"}`, 4),
					Aggregation: mustParse[*promParser.AggregateExpr](t, `sum(foo{job="myjob"})`, 0),
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Query is using aggregation that removes all labels.",
						},
					},
					IsConditional: true,
				},
			},
		},
		{
			expr: `sum(foo{job="myjob"}) without(job)`,
			output: []utils.Source{
				{
					Type:           utils.AggregateSource,
					Returns:        promParser.ValueTypeVector,
					Operation:      "sum",
					Selector:       mustParse[*promParser.VectorSelector](t, `foo{job="myjob"}`, 4),
					Aggregation:    mustParse[*promParser.AggregateExpr](t, `sum(foo{job="myjob"}) without(job)`, 0),
					ExcludedLabels: []string{"job"},
					ExcludeReason: map[string]utils.ExcludedLabel{
						"job": {
							Reason: "Query is using aggregation with `without(job)`, all labels included inside `without(...)` will be removed from the results.",
						},
					},
				},
			},
		},
		{
			expr: `sum(foo) by(job)`,
			output: []utils.Source{
				{
					Type:           utils.AggregateSource,
					Returns:        promParser.ValueTypeVector,
					Operation:      "sum",
					Selector:       mustParse[*promParser.VectorSelector](t, `foo`, 4),
					Aggregation:    mustParse[*promParser.AggregateExpr](t, `sum(foo) by(job)`, 0),
					IncludedLabels: []string{"job"},
					FixedLabels:    true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Query is using aggregation with `by(job)`, only labels included inside `by(...)` will be present on the results.",
						},
					},
				},
			},
		},
		{
			expr: `sum(foo{job="myjob"}) by(job)`,
			output: []utils.Source{
				{
					Type:             utils.AggregateSource,
					Returns:          promParser.ValueTypeVector,
					Operation:        "sum",
					Selector:         mustParse[*promParser.VectorSelector](t, `foo{job="myjob"}`, 4),
					Aggregation:      mustParse[*promParser.AggregateExpr](t, `sum(foo{job="myjob"}) by(job)`, 0),
					IncludedLabels:   []string{"job"},
					GuaranteedLabels: []string{"job"},
					FixedLabels:      true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Query is using aggregation with `by(job)`, only labels included inside `by(...)` will be present on the results.",
						},
					},
				},
			},
		},
		{
			expr: `abs(foo{job="myjob"} offset 5m)`,
			output: []utils.Source{
				{
					Type:             utils.FuncSource,
					Returns:          promParser.ValueTypeVector,
					Operation:        "abs",
					Selector:         mustParse[*promParser.VectorSelector](t, `foo{job="myjob"} offset 5m`, 4),
					Call:             mustParse[*promParser.Call](t, `abs(foo{job="myjob"} offset 5m)`, 0),
					GuaranteedLabels: []string{"job"},
				},
			},
		},
		{
			expr: `abs(foo{job="myjob"} or bar{cluster="dev"})`,
			output: []utils.Source{
				{
					Type:             utils.FuncSource,
					Returns:          promParser.ValueTypeVector,
					Operation:        "abs",
					Selector:         mustParse[*promParser.VectorSelector](t, `foo{job="myjob"}`, 4),
					GuaranteedLabels: []string{"job"},
					Call:             mustParse[*promParser.Call](t, `abs(foo{job="myjob"} or bar{cluster="dev"})`, 0),
				},
				{
					Type:             utils.FuncSource,
					Returns:          promParser.ValueTypeVector,
					Operation:        "abs",
					Selector:         mustParse[*promParser.VectorSelector](t, `bar{cluster="dev"}`, 24),
					Call:             mustParse[*promParser.Call](t, `abs(foo{job="myjob"} or bar{cluster="dev"})`, 0),
					GuaranteedLabels: []string{"cluster"},
				},
			},
		},
		{
			expr: `sum(foo{job="myjob"} or bar{cluster="dev"}) without(instance)`,
			output: []utils.Source{
				{
					Type:             utils.AggregateSource,
					Returns:          promParser.ValueTypeVector,
					Operation:        "sum",
					Selector:         mustParse[*promParser.VectorSelector](t, `foo{job="myjob"}`, 4),
					Aggregation:      mustParse[*promParser.AggregateExpr](t, `sum(foo{job="myjob"} or bar{cluster="dev"}) without(instance)`, 0),
					GuaranteedLabels: []string{"job"},
					ExcludedLabels:   []string{"instance"},
					ExcludeReason: map[string]utils.ExcludedLabel{
						"instance": {
							Reason: "Query is using aggregation with `without(instance)`, all labels included inside `without(...)` will be removed from the results.",
						},
					},
				},
				{
					Type:             utils.AggregateSource,
					Returns:          promParser.ValueTypeVector,
					Operation:        "sum",
					Selector:         mustParse[*promParser.VectorSelector](t, `bar{cluster="dev"}`, 24),
					Aggregation:      mustParse[*promParser.AggregateExpr](t, `sum(foo{job="myjob"} or bar{cluster="dev"}) without(instance)`, 0),
					GuaranteedLabels: []string{"cluster"},
					ExcludedLabels:   []string{"instance"},
					ExcludeReason: map[string]utils.ExcludedLabel{
						"instance": {
							Reason: "Query is using aggregation with `without(instance)`, all labels included inside `without(...)` will be removed from the results.",
						},
					},
				},
			},
		},
		{
			expr: `sum(foo{job="myjob"}) without(instance)`,
			output: []utils.Source{
				{
					Type:             utils.AggregateSource,
					Returns:          promParser.ValueTypeVector,
					Operation:        "sum",
					Selector:         mustParse[*promParser.VectorSelector](t, `foo{job="myjob"}`, 4),
					Aggregation:      mustParse[*promParser.AggregateExpr](t, `sum(foo{job="myjob"}) without(instance)`, 0),
					GuaranteedLabels: []string{"job"},
					ExcludedLabels:   []string{"instance"},
					ExcludeReason: map[string]utils.ExcludedLabel{
						"instance": {
							Reason: "Query is using aggregation with `without(instance)`, all labels included inside `without(...)` will be removed from the results.",
						},
					},
				},
			},
		},
		{
			expr: `min(foo{job="myjob"}) / max(foo{job="myjob"})`,
			output: []utils.Source{
				{
					Type:        utils.AggregateSource,
					Returns:     promParser.ValueTypeVector,
					Operation:   "min",
					Selector:    mustParse[*promParser.VectorSelector](t, `foo{job="myjob"}`, 4),
					Aggregation: mustParse[*promParser.AggregateExpr](t, `min(foo{job="myjob"})`, 0),
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Query is using aggregation that removes all labels.",
						},
					},
					Joins: []utils.Join{
						{
							Src: utils.Source{
								Type:        utils.AggregateSource,
								Operation:   "max",
								Returns:     promParser.ValueTypeVector,
								Selector:    mustParse[*promParser.VectorSelector](t, `foo{job="myjob"}`, 28),
								Aggregation: mustParse[*promParser.AggregateExpr](t, `max(foo{job="myjob"})`, 24),
								FixedLabels: true,
								ExcludeReason: map[string]utils.ExcludedLabel{
									"": {
										Reason: "Query is using aggregation that removes all labels.",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			expr: `max(foo{job="myjob"}) / min(foo{job="myjob"})`,
			output: []utils.Source{
				{
					Type:        utils.AggregateSource,
					Returns:     promParser.ValueTypeVector,
					Operation:   "max",
					Selector:    mustParse[*promParser.VectorSelector](t, `foo{job="myjob"}`, 4),
					Aggregation: mustParse[*promParser.AggregateExpr](t, `max(foo{job="myjob"})`, 0),
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Query is using aggregation that removes all labels.",
						},
					},
					Joins: []utils.Join{
						{
							Src: utils.Source{
								Type:        utils.AggregateSource,
								Operation:   "min",
								Returns:     promParser.ValueTypeVector,
								Selector:    mustParse[*promParser.VectorSelector](t, `foo{job="myjob"}`, 28),
								Aggregation: mustParse[*promParser.AggregateExpr](t, `min(foo{job="myjob"})`, 24),
								FixedLabels: true,
								ExcludeReason: map[string]utils.ExcludedLabel{
									"": {
										Reason: "Query is using aggregation that removes all labels.",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			expr: `avg(foo{job="myjob"}) by(job)`,
			output: []utils.Source{
				{
					Type:             utils.AggregateSource,
					Returns:          promParser.ValueTypeVector,
					Operation:        "avg",
					Selector:         mustParse[*promParser.VectorSelector](t, `foo{job="myjob"}`, 4),
					Aggregation:      mustParse[*promParser.AggregateExpr](t, `avg(foo{job="myjob"}) by(job)`, 0),
					GuaranteedLabels: []string{"job"},
					IncludedLabels:   []string{"job"},
					FixedLabels:      true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Query is using aggregation with `by(job)`, only labels included inside `by(...)` will be present on the results.",
						},
					},
				},
			},
		},
		{
			expr: `group(foo) by(job)`,
			output: []utils.Source{
				{
					Type:           utils.AggregateSource,
					Returns:        promParser.ValueTypeVector,
					Operation:      "group",
					Selector:       mustParse[*promParser.VectorSelector](t, `foo`, 6),
					Aggregation:    mustParse[*promParser.AggregateExpr](t, `group(foo) by(job)`, 0),
					IncludedLabels: []string{"job"},
					FixedLabels:    true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Query is using aggregation with `by(job)`, only labels included inside `by(...)` will be present on the results.",
						},
					},
				},
			},
		},
		{
			expr: `stddev(rate(foo[5m]))`,
			output: []utils.Source{
				{
					Type:        utils.AggregateSource,
					Returns:     promParser.ValueTypeVector,
					Operation:   "stddev",
					Selector:    mustParse[*promParser.VectorSelector](t, `foo`, 12),
					Call:        mustParse[*promParser.Call](t, "rate(foo[5m])", 7),
					Aggregation: mustParse[*promParser.AggregateExpr](t, `stddev(rate(foo[5m]))`, 0),
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Query is using aggregation that removes all labels.",
						},
					},
				},
			},
		},
		{
			expr: `stdvar(rate(foo[5m]))`,
			output: []utils.Source{
				{
					Type:        utils.AggregateSource,
					Returns:     promParser.ValueTypeVector,
					Operation:   "stdvar",
					Selector:    mustParse[*promParser.VectorSelector](t, `foo`, 12),
					Call:        mustParse[*promParser.Call](t, "rate(foo[5m])", 7),
					Aggregation: mustParse[*promParser.AggregateExpr](t, `stdvar(rate(foo[5m]))`, 0),
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Query is using aggregation that removes all labels.",
						},
					},
				},
			},
		},
		{
			expr: `stddev_over_time(foo[5m])`,
			output: []utils.Source{
				{
					Type:      utils.FuncSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "stddev_over_time",
					Selector:  mustParse[*promParser.VectorSelector](t, `foo`, 17),
					Call:      mustParse[*promParser.Call](t, `stddev_over_time(foo[5m])`, 0),
				},
			},
		},
		{
			expr: `stdvar_over_time(foo[5m])`,
			output: []utils.Source{
				{
					Type:      utils.FuncSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "stdvar_over_time",
					Selector:  mustParse[*promParser.VectorSelector](t, `foo`, 17),
					Call:      mustParse[*promParser.Call](t, `stdvar_over_time(foo[5m])`, 0),
				},
			},
		},
		{
			expr: `quantile(0.9, rate(foo[5m]))`,
			output: []utils.Source{
				{
					Type:        utils.AggregateSource,
					Returns:     promParser.ValueTypeVector,
					Operation:   "quantile",
					Selector:    mustParse[*promParser.VectorSelector](t, `foo`, 19),
					Call:        mustParse[*promParser.Call](t, "rate(foo[5m])", 14),
					Aggregation: mustParse[*promParser.AggregateExpr](t, `quantile(0.9, rate(foo[5m]))`, 0),
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Query is using aggregation that removes all labels.",
						},
					},
				},
			},
		},
		{
			expr: `count_values("version", build_version)`,
			output: []utils.Source{
				{
					Type:             utils.AggregateSource,
					Returns:          promParser.ValueTypeVector,
					Operation:        "count_values",
					Selector:         mustParse[*promParser.VectorSelector](t, `build_version`, 24),
					Aggregation:      mustParse[*promParser.AggregateExpr](t, `count_values("version", build_version)`, 0),
					GuaranteedLabels: []string{"version"},
					IncludedLabels:   []string{"version"},
					FixedLabels:      true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Query is using aggregation that removes all labels.",
						},
					},
				},
			},
		},
		{
			expr: `count_values("version", build_version) without(job)`,
			output: []utils.Source{
				{
					Type:             utils.AggregateSource,
					Returns:          promParser.ValueTypeVector,
					Operation:        "count_values",
					Selector:         mustParse[*promParser.VectorSelector](t, `build_version`, 24),
					Aggregation:      mustParse[*promParser.AggregateExpr](t, `count_values("version", build_version) without(job)`, 0),
					IncludedLabels:   []string{"version"},
					GuaranteedLabels: []string{"version"},
					ExcludedLabels:   []string{"job"},
					ExcludeReason: map[string]utils.ExcludedLabel{
						"job": {
							Reason: "Query is using aggregation with `without(job)`, all labels included inside `without(...)` will be removed from the results.",
						},
					},
				},
			},
		},
		{
			expr: `count_values("version", build_version{job="foo"}) without(job)`,
			output: []utils.Source{
				{
					Type:             utils.AggregateSource,
					Returns:          promParser.ValueTypeVector,
					Operation:        "count_values",
					Selector:         mustParse[*promParser.VectorSelector](t, `build_version{job="foo"}`, 24),
					Aggregation:      mustParse[*promParser.AggregateExpr](t, `count_values("version", build_version{job="foo"}) without(job)`, 0),
					IncludedLabels:   []string{"version"},
					GuaranteedLabels: []string{"version"},
					ExcludedLabels:   []string{"job"},
					ExcludeReason: map[string]utils.ExcludedLabel{
						"job": {
							Reason: "Query is using aggregation with `without(job)`, all labels included inside `without(...)` will be removed from the results.",
						},
					},
				},
			},
		},
		{
			expr: `count_values("version", build_version) by(job)`,
			output: []utils.Source{
				{
					Type:             utils.AggregateSource,
					Returns:          promParser.ValueTypeVector,
					Operation:        "count_values",
					Selector:         mustParse[*promParser.VectorSelector](t, `build_version`, 24),
					Aggregation:      mustParse[*promParser.AggregateExpr](t, `count_values("version", build_version) by(job)`, 0),
					GuaranteedLabels: []string{"version"},
					IncludedLabels:   []string{"job", "version"},
					FixedLabels:      true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Query is using aggregation with `by(job)`, only labels included inside `by(...)` will be present on the results.",
						},
					},
				},
			},
		},
		{
			expr: `topk(10, foo{job="myjob"}) > 10`,
			output: []utils.Source{
				{
					Type:             utils.AggregateSource,
					Returns:          promParser.ValueTypeVector,
					Operation:        "topk",
					Selector:         mustParse[*promParser.VectorSelector](t, `foo{job="myjob"}`, 9),
					Aggregation:      mustParse[*promParser.AggregateExpr](t, `topk(10, foo{job="myjob"})`, 0),
					GuaranteedLabels: []string{"job"},
					IsConditional:    true,
				},
			},
		},
		{
			expr: `topk(10, foo or bar)`,
			output: []utils.Source{
				{
					Type:        utils.AggregateSource,
					Returns:     promParser.ValueTypeVector,
					Operation:   "topk",
					Selector:    mustParse[*promParser.VectorSelector](t, `foo`, 9),
					Aggregation: mustParse[*promParser.AggregateExpr](t, `topk(10, foo or bar)`, 0),
				},
				{
					Type:        utils.AggregateSource,
					Operation:   "topk",
					Returns:     promParser.ValueTypeVector,
					Selector:    mustParse[*promParser.VectorSelector](t, `bar`, 16),
					Aggregation: mustParse[*promParser.AggregateExpr](t, `topk(10, foo or bar)`, 0),
				},
			},
		},
		{
			expr: `rate(foo[10m])`,
			output: []utils.Source{
				{
					Type:      utils.FuncSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "rate",
					Selector:  mustParse[*promParser.VectorSelector](t, `foo`, 5),
					Call:      mustParse[*promParser.Call](t, `rate(foo[10m])`, 0),
				},
			},
		},
		{
			expr: `sum(rate(foo[10m])) without(instance)`,
			output: []utils.Source{
				{
					Type:           utils.AggregateSource,
					Returns:        promParser.ValueTypeVector,
					Operation:      "sum",
					Selector:       mustParse[*promParser.VectorSelector](t, `foo`, 9),
					Call:           mustParse[*promParser.Call](t, "rate(foo[10m])", 4),
					Aggregation:    mustParse[*promParser.AggregateExpr](t, `sum(rate(foo[10m])) without(instance)`, 0),
					ExcludedLabels: []string{"instance"},
					ExcludeReason: map[string]utils.ExcludedLabel{
						"instance": {
							Reason: "Query is using aggregation with `without(instance)`, all labels included inside `without(...)` will be removed from the results.",
						},
					},
				},
			},
		},
		{
			expr: `foo{job="foo"} / bar`,
			output: []utils.Source{
				{
					Type:             utils.SelectorSource,
					Returns:          promParser.ValueTypeVector,
					Operation:        promParser.CardOneToOne.String(),
					Selector:         mustParse[*promParser.VectorSelector](t, `foo{job="foo"}`, 0),
					GuaranteedLabels: []string{"job"},
					Joins: []utils.Join{
						{
							Src: utils.Source{
								Type:     utils.SelectorSource,
								Returns:  promParser.ValueTypeVector,
								Selector: mustParse[*promParser.VectorSelector](t, `bar`, 17),
							},
						},
					},
				},
			},
		},
		{
			expr: `foo{job="foo"} * on(instance) bar`,
			output: []utils.Source{
				{
					Type:           utils.SelectorSource,
					Returns:        promParser.ValueTypeVector,
					Operation:      promParser.CardOneToOne.String(),
					Selector:       mustParse[*promParser.VectorSelector](t, `foo{job="foo"}`, 0),
					IncludedLabels: []string{"instance"},
					FixedLabels:    true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Query is using one-to-one vector matching with `on(instance)`, only labels included inside `on(...)` will be present on the results.",
						},
					},
					Joins: []utils.Join{
						{
							Src: utils.Source{
								Type:     utils.SelectorSource,
								Returns:  promParser.ValueTypeVector,
								Selector: mustParse[*promParser.VectorSelector](t, `bar`, 30),
							},
						},
					},
				},
			},
		},
		{
			expr: `foo{job="foo"} * on(instance) group_left(bar) bar`,
			output: []utils.Source{
				{
					Type:             utils.SelectorSource,
					Returns:          promParser.ValueTypeVector,
					Operation:        promParser.CardManyToOne.String(),
					Selector:         mustParse[*promParser.VectorSelector](t, `foo{job="foo"}`, 0),
					IncludedLabels:   []string{"bar", "instance"},
					GuaranteedLabels: []string{"job"},
					Joins: []utils.Join{
						{
							Src: utils.Source{
								Type:     utils.SelectorSource,
								Returns:  promParser.ValueTypeVector,
								Selector: mustParse[*promParser.VectorSelector](t, `bar`, 46),
							},
						},
					},
				},
			},
		},
		{
			expr: `foo{job="foo"} * on(instance) group_left(cluster) bar{cluster="bar", ignored="true"}`,
			output: []utils.Source{
				{
					Type:             utils.SelectorSource,
					Returns:          promParser.ValueTypeVector,
					Operation:        promParser.CardManyToOne.String(),
					Selector:         mustParse[*promParser.VectorSelector](t, `foo{job="foo"}`, 0),
					IncludedLabels:   []string{"cluster", "instance"},
					GuaranteedLabels: []string{"job"},
					Joins: []utils.Join{
						{
							Src: utils.Source{
								Type:             utils.SelectorSource,
								Returns:          promParser.ValueTypeVector,
								Selector:         mustParse[*promParser.VectorSelector](t, `bar{cluster="bar", ignored="true"}`, 50),
								GuaranteedLabels: []string{"cluster"},
							},
						},
					},
				},
			},
		},
		{
			expr: `foo{job="foo", ignored="true"} * on(instance) group_right(job) bar{cluster="bar"}`,
			output: []utils.Source{
				{
					Type:             utils.SelectorSource,
					Returns:          promParser.ValueTypeVector,
					Operation:        promParser.CardOneToMany.String(),
					Selector:         mustParse[*promParser.VectorSelector](t, `bar{cluster="bar"}`, 63),
					IncludedLabels:   []string{"job", "instance"},
					GuaranteedLabels: []string{"cluster"},
					Joins: []utils.Join{
						{
							Src: utils.Source{
								Type:             utils.SelectorSource,
								Returns:          promParser.ValueTypeVector,
								Selector:         mustParse[*promParser.VectorSelector](t, `foo{job="foo", ignored="true"}`, 0),
								GuaranteedLabels: []string{"job"},
							},
						},
					},
				},
			},
		},
		{
			expr: `count(foo / bar)`,
			output: []utils.Source{
				{
					Type:        utils.AggregateSource,
					Returns:     promParser.ValueTypeVector,
					Operation:   "count",
					Selector:    mustParse[*promParser.VectorSelector](t, `foo`, 6),
					Aggregation: mustParse[*promParser.AggregateExpr](t, `count(foo / bar)`, 0),
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Query is using aggregation that removes all labels.",
						},
					},
					Joins: []utils.Join{
						{
							Src: utils.Source{
								Type:     utils.SelectorSource,
								Returns:  promParser.ValueTypeVector,
								Selector: mustParse[*promParser.VectorSelector](t, `bar`, 12),
							},
						},
					},
				},
			},
		},
		{
			expr: `count(up{job="a"} / on () up{job="b"})`,
			output: []utils.Source{
				{
					Type:        utils.AggregateSource,
					Returns:     promParser.ValueTypeVector,
					Operation:   "count",
					Selector:    mustParse[*promParser.VectorSelector](t, `up{job="a"}`, 6),
					Aggregation: mustParse[*promParser.AggregateExpr](t, `count(up{job="a"} / on () up{job="b"})`, 0),
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Query is using aggregation that removes all labels.",
						},
					},
					Joins: []utils.Join{
						{
							Src: utils.Source{
								Type:             utils.SelectorSource,
								Returns:          promParser.ValueTypeVector,
								Selector:         mustParse[*promParser.VectorSelector](t, `up{job="b"}`, 26),
								GuaranteedLabels: []string{"job"},
							},
						},
					},
				},
			},
		},
		{
			expr: `count(up{job="a"} / on (env) up{job="b"})`,
			output: []utils.Source{
				{
					Type:        utils.AggregateSource,
					Returns:     promParser.ValueTypeVector,
					Operation:   "count",
					Selector:    mustParse[*promParser.VectorSelector](t, `up{job="a"}`, 6),
					Aggregation: mustParse[*promParser.AggregateExpr](t, `count(up{job="a"} / on (env) up{job="b"})`, 0),
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Query is using aggregation that removes all labels.",
						},
					},
					Joins: []utils.Join{
						{
							Src: utils.Source{
								Type:             utils.SelectorSource,
								Returns:          promParser.ValueTypeVector,
								Selector:         mustParse[*promParser.VectorSelector](t, `up{job="b"}`, 29),
								GuaranteedLabels: []string{"job"},
							},
						},
					},
				},
			},
		},
		{
			expr: `foo{job="foo", instance="1"} and bar`,
			output: []utils.Source{
				{
					Type:             utils.SelectorSource,
					Returns:          promParser.ValueTypeVector,
					Operation:        promParser.CardManyToMany.String(),
					Selector:         mustParse[*promParser.VectorSelector](t, `foo{job="foo", instance="1"}`, 0),
					GuaranteedLabels: []string{"job", "instance"},
					Joins: []utils.Join{
						{
							Src: utils.Source{
								Type:     utils.SelectorSource,
								Returns:  promParser.ValueTypeVector,
								Selector: mustParse[*promParser.VectorSelector](t, `bar`, 33),
							},
						},
					},
				},
			},
		},
		{
			expr: `foo{job="foo", instance="1"} and on(cluster) bar`,
			output: []utils.Source{
				{
					Type:             utils.SelectorSource,
					Returns:          promParser.ValueTypeVector,
					Operation:        promParser.CardManyToMany.String(),
					Selector:         mustParse[*promParser.VectorSelector](t, `foo{job="foo", instance="1"}`, 0),
					IncludedLabels:   []string{"cluster"},
					GuaranteedLabels: []string{"job", "instance"},
					Joins: []utils.Join{
						{
							Src: utils.Source{
								Type:     utils.SelectorSource,
								Returns:  promParser.ValueTypeVector,
								Selector: mustParse[*promParser.VectorSelector](t, `bar`, 45),
							},
						},
					},
				},
			},
		},
		{
			expr: `topk(10, foo)`,
			output: []utils.Source{
				{
					Type:        utils.AggregateSource,
					Returns:     promParser.ValueTypeVector,
					Operation:   "topk",
					Selector:    mustParse[*promParser.VectorSelector](t, `foo`, 9),
					Aggregation: mustParse[*promParser.AggregateExpr](t, `topk(10, foo)`, 0),
				},
			},
		},
		{
			expr: `topk(10, foo) without(cluster)`,
			output: []utils.Source{
				{
					Type:        utils.AggregateSource,
					Returns:     promParser.ValueTypeVector,
					Operation:   "topk",
					Selector:    mustParse[*promParser.VectorSelector](t, `foo`, 9),
					Aggregation: mustParse[*promParser.AggregateExpr](t, `topk(10, foo) without(cluster)`, 0),
				},
			},
		},
		{
			expr: `topk(10, foo) by(cluster)`,
			output: []utils.Source{
				{
					Type:        utils.AggregateSource,
					Returns:     promParser.ValueTypeVector,
					Operation:   "topk",
					Selector:    mustParse[*promParser.VectorSelector](t, `foo`, 9),
					Aggregation: mustParse[*promParser.AggregateExpr](t, `topk(10, foo) by(cluster)`, 0),
				},
			},
		},
		{
			expr: `bottomk(10, sum(rate(foo[5m])) without(job))`,
			output: []utils.Source{
				{
					Type:           utils.AggregateSource,
					Returns:        promParser.ValueTypeVector,
					Operation:      "bottomk",
					Selector:       mustParse[*promParser.VectorSelector](t, `foo`, 21),
					Call:           mustParse[*promParser.Call](t, "rate(foo[5m])", 16),
					Aggregation:    mustParse[*promParser.AggregateExpr](t, `bottomk(10, sum(rate(foo[5m])) without(job))`, 0),
					ExcludedLabels: []string{"job"},
					ExcludeReason: map[string]utils.ExcludedLabel{
						"job": {
							Reason: "Query is using aggregation with `without(job)`, all labels included inside `without(...)` will be removed from the results.",
						},
					},
				},
			},
		},
		{
			expr: `foo or bar`,
			output: []utils.Source{
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardManyToMany.String(),
					Selector:  mustParse[*promParser.VectorSelector](t, `foo`, 0),
				},
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardManyToMany.String(),
					Selector:  mustParse[*promParser.VectorSelector](t, `bar`, 7),
				},
			},
		},
		{
			expr: `foo or bar or baz`,
			output: []utils.Source{
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardManyToMany.String(),
					Selector:  mustParse[*promParser.VectorSelector](t, `foo`, 0),
				},
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardManyToMany.String(),
					Selector:  mustParse[*promParser.VectorSelector](t, `bar`, 7),
				},
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardManyToMany.String(),
					Selector:  mustParse[*promParser.VectorSelector](t, `baz`, 14),
				},
			},
		},
		{
			expr: `(foo or bar) or baz`,
			output: []utils.Source{
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardManyToMany.String(),
					Selector:  mustParse[*promParser.VectorSelector](t, `foo`, 1),
				},
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardManyToMany.String(),
					Selector:  mustParse[*promParser.VectorSelector](t, `bar`, 8),
				},
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardManyToMany.String(),
					Selector:  mustParse[*promParser.VectorSelector](t, `baz`, 16),
				},
			},
		},
		{
			expr: `foo unless bar`,
			output: []utils.Source{
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardManyToMany.String(),
					Selector:  mustParse[*promParser.VectorSelector](t, `foo`, 0),
					Unless: []utils.Join{
						{
							Src: utils.Source{
								Type:     utils.SelectorSource,
								Returns:  promParser.ValueTypeVector,
								Selector: mustParse[*promParser.VectorSelector](t, `bar`, 11),
							},
						},
					},
				},
			},
		},
		{
			expr: `foo unless bar > 5`,
			output: []utils.Source{
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardManyToMany.String(),
					Selector:  mustParse[*promParser.VectorSelector](t, `foo`, 0),
					Unless: []utils.Join{
						{
							Src: utils.Source{
								Type:          utils.SelectorSource,
								Returns:       promParser.ValueTypeVector,
								Selector:      mustParse[*promParser.VectorSelector](t, `bar`, 11),
								IsConditional: true,
							},
						},
					},
				},
			},
		},
		{
			expr: `foo unless bar unless baz`,
			output: []utils.Source{
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardManyToMany.String(),
					Selector:  mustParse[*promParser.VectorSelector](t, `foo`, 0),
					Unless: []utils.Join{
						{
							Src: utils.Source{
								Type:     utils.SelectorSource,
								Returns:  promParser.ValueTypeVector,
								Selector: mustParse[*promParser.VectorSelector](t, `bar`, 11),
							},
						},
						{
							Src: utils.Source{
								Type:     utils.SelectorSource,
								Returns:  promParser.ValueTypeVector,
								Selector: mustParse[*promParser.VectorSelector](t, `baz`, 22),
							},
						},
					},
				},
			},
		},
		{
			expr: `count(sum(up{job="foo", cluster="dev"}) by(job, cluster) == 0) without(job, cluster)`,
			output: []utils.Source{
				{
					Type:           utils.AggregateSource,
					Returns:        promParser.ValueTypeVector,
					Operation:      "count",
					Selector:       mustParse[*promParser.VectorSelector](t, `up{job="foo", cluster="dev"}`, 10),
					Aggregation:    mustParse[*promParser.AggregateExpr](t, `count(sum(up{job="foo", cluster="dev"}) by(job, cluster) == 0) without(job, cluster)`, 0),
					ExcludedLabels: []string{"job", "cluster"}, // FIXME empty
					FixedLabels:    true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Query is using aggregation with `by(job, cluster)`, only labels included inside `by(...)` will be present on the results.",
						},
						"job": {
							Reason: "Query is using aggregation with `without(job, cluster)`, all labels included inside `without(...)` will be removed from the results.",
						},
						"cluster": {
							Reason: "Query is using aggregation with `without(job, cluster)`, all labels included inside `without(...)` will be removed from the results.",
						},
					},
					IsConditional: true,
				},
			},
		},
		{
			expr: "year()",
			output: []utils.Source{
				{
					Type:          utils.FuncSource,
					Returns:       promParser.ValueTypeVector,
					Operation:     "year",
					FixedLabels:   true,
					AlwaysReturns: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Calling `year()` with no arguments will return an empty time series with no labels.",
						},
					},
					Call: mustParse[*promParser.Call](t, `year()`, 0),
				},
			},
		},
		{
			expr: "year(foo)",
			output: []utils.Source{
				{
					Type:      utils.FuncSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "year",
					Selector:  mustParse[*promParser.VectorSelector](t, "foo", 5),
					Call:      mustParse[*promParser.Call](t, `year(foo)`, 0),
				},
			},
		},
		{
			expr: `label_join(up{job="api-server",src1="a",src2="b",src3="c"}, "foo", ",", "src1", "src2", "src3")`,
			output: []utils.Source{
				{
					Type:             utils.FuncSource,
					Returns:          promParser.ValueTypeVector,
					Operation:        "label_join",
					Selector:         mustParse[*promParser.VectorSelector](t, `up{job="api-server",src1="a",src2="b",src3="c"}`, 11),
					GuaranteedLabels: []string{"job", "src1", "src2", "src3", "foo"},
					Call:             mustParse[*promParser.Call](t, `label_join(up{job="api-server",src1="a",src2="b",src3="c"}, "foo", ",", "src1", "src2", "src3")`, 0),
				},
			},
		},
		{
			expr: `
(
	sum(foo:sum > 0) without(notify)
	* on(job) group_left(notify)
	job:notify
)
and on(job)
sum(foo:count) by(job) > 20`,
			output: []utils.Source{
				{
					Type:           utils.AggregateSource,
					Returns:        promParser.ValueTypeVector,
					Operation:      "sum",
					Selector:       mustParse[*promParser.VectorSelector](t, `foo:sum`, 8),
					Aggregation:    mustParse[*promParser.AggregateExpr](t, `sum(foo:sum > 0) without(notify)`, 4),
					IncludedLabels: []string{"notify", "job"},
					ExcludeReason:  map[string]utils.ExcludedLabel{},
					IsConditional:  false, // FIXME should be true
					Joins: []utils.Join{
						{
							Src: utils.Source{
								Type:     utils.SelectorSource,
								Returns:  promParser.ValueTypeVector,
								Selector: mustParse[*promParser.VectorSelector](t, `job:notify`, 68),
							},
						},
						{
							Src: utils.Source{
								Type:           utils.AggregateSource,
								Returns:        promParser.ValueTypeVector,
								Operation:      "sum",
								Selector:       mustParse[*promParser.VectorSelector](t, `foo:count`, 97),
								Aggregation:    mustParse[*promParser.AggregateExpr](t, `sum(foo:count) by(job)`, 93),
								IncludedLabels: []string{"job"},
								FixedLabels:    true,
								ExcludeReason: map[string]utils.ExcludedLabel{
									"": {
										Reason: "Query is using aggregation with `by(job)`, only labels included inside `by(...)` will be present on the results.",
									},
								},
								IsConditional: true,
							},
						},
					},
				},
			},
		},
		{
			expr: `container_file_descriptors / on (instance, app_name) container_ulimits_soft{ulimit="max_open_files"}`,
			output: []utils.Source{
				{
					Type:           utils.SelectorSource,
					Returns:        promParser.ValueTypeVector,
					Operation:      promParser.CardOneToOne.String(),
					Selector:       mustParse[*promParser.VectorSelector](t, `container_file_descriptors`, 0),
					IncludedLabels: []string{"instance", "app_name"},
					FixedLabels:    true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Query is using one-to-one vector matching with `on(instance, app_name)`, only labels included inside `on(...)` will be present on the results.",
						},
					},
					Joins: []utils.Join{
						{
							Src: utils.Source{
								Type:             utils.SelectorSource,
								Returns:          promParser.ValueTypeVector,
								Selector:         mustParse[*promParser.VectorSelector](t, `container_ulimits_soft{ulimit="max_open_files"}`, 53),
								GuaranteedLabels: []string{"ulimit"},
							},
						},
					},
				},
			},
		},
		{
			expr: `container_file_descriptors / on (instance, app_name) group_left() container_ulimits_soft{ulimit="max_open_files"}`,
			output: []utils.Source{
				{
					Type:           utils.SelectorSource,
					Returns:        promParser.ValueTypeVector,
					Operation:      promParser.CardManyToOne.String(),
					Selector:       mustParse[*promParser.VectorSelector](t, `container_file_descriptors`, 0),
					IncludedLabels: []string{"instance", "app_name"},
					Joins: []utils.Join{
						{
							Src: utils.Source{
								Type:     utils.SelectorSource,
								Returns:  promParser.ValueTypeVector,
								Selector: mustParse[*promParser.VectorSelector](t, `container_ulimits_soft{ulimit="max_open_files"}`, 66),
							},
						},
					},
				},
			},
		},
		{
			expr: `absent(foo{job="bar"})`,
			output: []utils.Source{
				{
					Type:             utils.FuncSource,
					Returns:          promParser.ValueTypeVector,
					Operation:        "absent",
					Selector:         mustParse[*promParser.VectorSelector](t, `foo{job="bar"}`, 7),
					IncludedLabels:   []string{"job"},
					GuaranteedLabels: []string{"job"},
					FixedLabels:      true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "The [absent()](https://prometheus.io/docs/prometheus/latest/querying/functions/#absent) function is used to check if provided query doesn't match any time series.\nYou will only get any results back if the metric selector you pass doesn't match anything.\nSince there are no matching time series there are also no labels. If some time series is missing you cannot read its labels.\nThis means that the only labels you can get back from absent call are the ones you pass to it.\nIf you're hoping to get instance specific labels this way and alert when some target is down then that won't work, use the `up` metric instead.",
						},
					},
					Call: mustParse[*promParser.Call](t, `absent(foo{job="bar"})`, 0),
				},
			},
		},
		{
			expr: `absent(foo{job="bar", cluster!="dev", instance=~".+", env="prod"})`,
			output: []utils.Source{
				{
					Type:             utils.FuncSource,
					Returns:          promParser.ValueTypeVector,
					Operation:        "absent",
					Selector:         mustParse[*promParser.VectorSelector](t, `foo{job="bar", cluster!="dev", instance=~".+", env="prod"}`, 7),
					IncludedLabels:   []string{"job", "env"},
					GuaranteedLabels: []string{"job", "env"},
					FixedLabels:      true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "The [absent()](https://prometheus.io/docs/prometheus/latest/querying/functions/#absent) function is used to check if provided query doesn't match any time series.\nYou will only get any results back if the metric selector you pass doesn't match anything.\nSince there are no matching time series there are also no labels. If some time series is missing you cannot read its labels.\nThis means that the only labels you can get back from absent call are the ones you pass to it.\nIf you're hoping to get instance specific labels this way and alert when some target is down then that won't work, use the `up` metric instead.",
						},
					},
					Call: mustParse[*promParser.Call](t, `absent(foo{job="bar", cluster!="dev", instance=~".+", env="prod"})`, 0),
				},
			},
		},
		{
			expr: `absent(sum(foo) by(job, instance))`,
			output: []utils.Source{
				{
					Type:        utils.FuncSource,
					Returns:     promParser.ValueTypeVector,
					Operation:   "absent",
					Selector:    mustParse[*promParser.VectorSelector](t, `foo`, 11),
					Aggregation: mustParse[*promParser.AggregateExpr](t, `sum(foo) by(job, instance)`, 7),
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "The [absent()](https://prometheus.io/docs/prometheus/latest/querying/functions/#absent) function is used to check if provided query doesn't match any time series.\nYou will only get any results back if the metric selector you pass doesn't match anything.\nSince there are no matching time series there are also no labels. If some time series is missing you cannot read its labels.\nThis means that the only labels you can get back from absent call are the ones you pass to it.\nIf you're hoping to get instance specific labels this way and alert when some target is down then that won't work, use the `up` metric instead.",
						},
					},
					Call: mustParse[*promParser.Call](t, `absent(sum(foo) by(job, instance))`, 0),
				},
			},
		},
		{
			expr: `absent(foo{job="prometheus", xxx="1"}) AND on(job) prometheus_build_info`,
			output: []utils.Source{
				{
					Type:             utils.FuncSource,
					Returns:          promParser.ValueTypeVector,
					Operation:        "absent",
					Selector:         mustParse[*promParser.VectorSelector](t, `foo{job="prometheus", xxx="1"}`, 7),
					IncludedLabels:   []string{"job", "xxx"},
					GuaranteedLabels: []string{"job", "xxx"},
					FixedLabels:      true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "The [absent()](https://prometheus.io/docs/prometheus/latest/querying/functions/#absent) function is used to check if provided query doesn't match any time series.\nYou will only get any results back if the metric selector you pass doesn't match anything.\nSince there are no matching time series there are also no labels. If some time series is missing you cannot read its labels.\nThis means that the only labels you can get back from absent call are the ones you pass to it.\nIf you're hoping to get instance specific labels this way and alert when some target is down then that won't work, use the `up` metric instead.",
						},
					},
					Call: mustParse[*promParser.Call](t, `absent(foo{job="prometheus", xxx="1"})`, 0),
					Joins: []utils.Join{
						{
							Src: utils.Source{
								Type:     utils.SelectorSource,
								Returns:  promParser.ValueTypeVector,
								Selector: mustParse[*promParser.VectorSelector](t, "prometheus_build_info", 51),
							},
						},
					},
				},
			},
		},
		{
			expr: `1 + sum(foo) by(notjob)`,
			output: []utils.Source{
				{
					Type:           utils.AggregateSource,
					Returns:        promParser.ValueTypeVector,
					Operation:      "sum",
					Selector:       mustParse[*promParser.VectorSelector](t, `foo`, 8),
					Aggregation:    mustParse[*promParser.AggregateExpr](t, `sum(foo) by(notjob)`, 4),
					IncludedLabels: []string{"notjob"},
					FixedLabels:    true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Query is using aggregation with `by(notjob)`, only labels included inside `by(...)` will be present on the results.",
						},
					},
				},
			},
		},
		{
			expr: `count(node_exporter_build_info) by (instance, version) != ignoring(package,version) group_left(foo) count(deb_package_version) by (instance, version, package)`,
			output: []utils.Source{
				{
					Type:           utils.AggregateSource,
					Returns:        promParser.ValueTypeVector,
					Operation:      "count",
					Selector:       mustParse[*promParser.VectorSelector](t, `node_exporter_build_info`, 6),
					Aggregation:    mustParse[*promParser.AggregateExpr](t, `count(node_exporter_build_info) by (instance, version)`, 0),
					IncludedLabels: []string{"instance", "version", "foo"},
					FixedLabels:    true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Query is using aggregation with `by(instance, version)`, only labels included inside `by(...)` will be present on the results.",
						},
					},
					IsConditional: true,
					Joins: []utils.Join{
						{
							Src: utils.Source{
								Type:           utils.AggregateSource,
								Returns:        promParser.ValueTypeVector,
								Operation:      "count",
								Selector:       mustParse[*promParser.VectorSelector](t, "deb_package_version", 106),
								Aggregation:    mustParse[*promParser.AggregateExpr](t, `count(deb_package_version) by (instance, version, package)`, 100),
								IncludedLabels: []string{"instance", "version", "package"},
								FixedLabels:    true,
								ExcludeReason: map[string]utils.ExcludedLabel{
									"": {
										Reason: "Query is using aggregation with `by(instance, version, package)`, only labels included inside `by(...)` will be present on the results.",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			expr: `absent(foo) or absent(bar)`,
			output: []utils.Source{
				{
					Type:        utils.FuncSource,
					Returns:     promParser.ValueTypeVector,
					Operation:   "absent",
					Selector:    mustParse[*promParser.VectorSelector](t, `foo`, 7),
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "The [absent()](https://prometheus.io/docs/prometheus/latest/querying/functions/#absent) function is used to check if provided query doesn't match any time series.\nYou will only get any results back if the metric selector you pass doesn't match anything.\nSince there are no matching time series there are also no labels. If some time series is missing you cannot read its labels.\nThis means that the only labels you can get back from absent call are the ones you pass to it.\nIf you're hoping to get instance specific labels this way and alert when some target is down then that won't work, use the `up` metric instead.",
						},
					},
					Call: mustParse[*promParser.Call](t, `absent(foo)`, 0),
				},
				{
					Type:        utils.FuncSource,
					Returns:     promParser.ValueTypeVector,
					Operation:   "absent",
					Selector:    mustParse[*promParser.VectorSelector](t, `bar`, 22),
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "The [absent()](https://prometheus.io/docs/prometheus/latest/querying/functions/#absent) function is used to check if provided query doesn't match any time series.\nYou will only get any results back if the metric selector you pass doesn't match anything.\nSince there are no matching time series there are also no labels. If some time series is missing you cannot read its labels.\nThis means that the only labels you can get back from absent call are the ones you pass to it.\nIf you're hoping to get instance specific labels this way and alert when some target is down then that won't work, use the `up` metric instead.",
						},
					},
					Call: mustParse[*promParser.Call](t, `absent(bar)`, 15),
				},
			},
		},
		{
			expr: `absent_over_time(foo[5m]) or absent(bar)`,
			output: []utils.Source{
				{
					Type:        utils.FuncSource,
					Returns:     promParser.ValueTypeVector,
					Operation:   "absent_over_time",
					Selector:    mustParse[*promParser.VectorSelector](t, `foo`, 17),
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "The [absent_over_time()](https://prometheus.io/docs/prometheus/latest/querying/functions/#absent_over_time) function is used to check if provided query doesn't match any time series.\nYou will only get any results back if the metric selector you pass doesn't match anything.\nSince there are no matching time series there are also no labels. If some time series is missing you cannot read its labels.\nThis means that the only labels you can get back from absent call are the ones you pass to it.\nIf you're hoping to get instance specific labels this way and alert when some target is down then that won't work, use the `up` metric instead.",
						},
					},
					Call: mustParse[*promParser.Call](t, `absent_over_time(foo[5m])`, 0),
				},
				{
					Type:        utils.FuncSource,
					Returns:     promParser.ValueTypeVector,
					Operation:   "absent",
					Selector:    mustParse[*promParser.VectorSelector](t, `bar`, 36),
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "The [absent()](https://prometheus.io/docs/prometheus/latest/querying/functions/#absent) function is used to check if provided query doesn't match any time series.\nYou will only get any results back if the metric selector you pass doesn't match anything.\nSince there are no matching time series there are also no labels. If some time series is missing you cannot read its labels.\nThis means that the only labels you can get back from absent call are the ones you pass to it.\nIf you're hoping to get instance specific labels this way and alert when some target is down then that won't work, use the `up` metric instead.",
						},
					},
					Call: mustParse[*promParser.Call](t, `absent(bar)`, 29),
				},
			},
		},
		{
			expr: `bar * on() group_right(cluster, env) absent(foo{job="xxx"})`,
			output: []utils.Source{
				{
					Type:             utils.FuncSource,
					Returns:          promParser.ValueTypeVector,
					Operation:        "absent",
					Selector:         mustParse[*promParser.VectorSelector](t, `foo{job="xxx"}`, 44),
					IncludedLabels:   []string{"job", "cluster", "env"},
					GuaranteedLabels: []string{"job"},
					FixedLabels:      true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "The [absent()](https://prometheus.io/docs/prometheus/latest/querying/functions/#absent) function is used to check if provided query doesn't match any time series.\nYou will only get any results back if the metric selector you pass doesn't match anything.\nSince there are no matching time series there are also no labels. If some time series is missing you cannot read its labels.\nThis means that the only labels you can get back from absent call are the ones you pass to it.\nIf you're hoping to get instance specific labels this way and alert when some target is down then that won't work, use the `up` metric instead.",
						},
					},
					Call: mustParse[*promParser.Call](t, `absent(foo{job="xxx"})`, 37),
					Joins: []utils.Join{
						{
							Src: utils.Source{
								Type:     utils.SelectorSource,
								Returns:  promParser.ValueTypeVector,
								Selector: mustParse[*promParser.VectorSelector](t, "bar", 0),
							},
						},
					},
				},
			},
		},
		{
			expr: `bar * on() group_right() absent(foo{job="xxx"})`,
			output: []utils.Source{
				{
					Type:             utils.FuncSource,
					Returns:          promParser.ValueTypeVector,
					Operation:        "absent",
					Selector:         mustParse[*promParser.VectorSelector](t, `foo{job="xxx"}`, 32),
					IncludedLabels:   []string{"job"},
					GuaranteedLabels: []string{"job"},
					FixedLabels:      true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "The [absent()](https://prometheus.io/docs/prometheus/latest/querying/functions/#absent) function is used to check if provided query doesn't match any time series.\nYou will only get any results back if the metric selector you pass doesn't match anything.\nSince there are no matching time series there are also no labels. If some time series is missing you cannot read its labels.\nThis means that the only labels you can get back from absent call are the ones you pass to it.\nIf you're hoping to get instance specific labels this way and alert when some target is down then that won't work, use the `up` metric instead.",
						},
					},
					Call: mustParse[*promParser.Call](t, `absent(foo{job="xxx"})`, 25),
					Joins: []utils.Join{
						{
							Src: utils.Source{
								Type:     utils.SelectorSource,
								Returns:  promParser.ValueTypeVector,
								Selector: mustParse[*promParser.VectorSelector](t, "bar", 0),
							},
						},
					},
				},
			},
		},
		{
			expr: "vector(1)",
			output: []utils.Source{
				{
					Type:           utils.FuncSource,
					Returns:        promParser.ValueTypeVector,
					Operation:      "vector",
					FixedLabels:    true,
					AlwaysReturns:  true,
					ReturnedNumber: 1,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Calling `vector()` will return a vector value with no labels.",
						},
					},
					Call: mustParse[*promParser.Call](t, `vector(1)`, 0),
				},
			},
		},
		{
			expr: "vector(scalar(foo))",
			output: []utils.Source{
				{
					Type:          utils.FuncSource,
					Returns:       promParser.ValueTypeVector,
					Operation:     "vector",
					FixedLabels:   true,
					AlwaysReturns: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Calling `vector()` will return a vector value with no labels.",
						},
					},
					Call: mustParse[*promParser.Call](t, `vector(scalar(foo))`, 0),
				},
			},
		},
		{
			expr: "vector(0.0  >= bool 0.5) == 1",
			output: []utils.Source{
				{
					Type:          utils.FuncSource,
					Returns:       promParser.ValueTypeVector,
					Operation:     "vector",
					FixedLabels:   true,
					AlwaysReturns: true,
					IsDead:        true,
					IsDeadReason:  "this query always evaluates to `0 == 1` which is not possible, so it will never return anything",
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Calling `vector()` will return a vector value with no labels.",
						},
					},
					Call:          mustParse[*promParser.Call](t, `vector(0.0  >= bool 0.5)`, 0),
					IsConditional: true,
				},
			},
		},
		{
			expr: `sum_over_time(foo{job="myjob"}[5m])`,
			output: []utils.Source{
				{
					Type:             utils.FuncSource,
					Returns:          promParser.ValueTypeVector,
					Operation:        "sum_over_time",
					Selector:         mustParse[*promParser.VectorSelector](t, `foo{job="myjob"}`, 14),
					GuaranteedLabels: []string{"job"},
					Call:             mustParse[*promParser.Call](t, `sum_over_time(foo{job="myjob"}[5m])`, 0),
				},
			},
		},
		{
			expr: `days_in_month()`,
			output: []utils.Source{
				{
					Type:          utils.FuncSource,
					Returns:       promParser.ValueTypeVector,
					Operation:     "days_in_month",
					FixedLabels:   true,
					AlwaysReturns: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Calling `days_in_month()` with no arguments will return an empty time series with no labels.",
						},
					},
					Call: mustParse[*promParser.Call](t, `days_in_month()`, 0),
				},
			},
		},
		{
			expr: `days_in_month(foo{job="foo"})`,
			output: []utils.Source{
				{
					Type:             utils.FuncSource,
					Returns:          promParser.ValueTypeVector,
					Operation:        "days_in_month",
					Selector:         mustParse[*promParser.VectorSelector](t, `foo{job="foo"}`, 14),
					GuaranteedLabels: []string{"job"},
					Call:             mustParse[*promParser.Call](t, `days_in_month(foo{job="foo"})`, 0),
				},
			},
		},
		{
			expr: `label_replace(up{job="api-server",service="a:c"}, "foo", "$1", "service", "(.*):.*")`,
			output: []utils.Source{
				{
					Type:             utils.FuncSource,
					Returns:          promParser.ValueTypeVector,
					Operation:        "label_replace",
					Selector:         mustParse[*promParser.VectorSelector](t, `up{job="api-server",service="a:c"}`, 14),
					GuaranteedLabels: []string{"job", "service", "foo"},
					Call:             mustParse[*promParser.Call](t, `label_replace(up{job="api-server",service="a:c"}, "foo", "$1", "service", "(.*):.*")`, 0),
				},
			},
		},
		{
			expr: `label_replace(sum by (pod) (pod_status) > 0, "cluster", "$1", "pod", "(.*)")`,
			output: []utils.Source{
				{
					Type:             utils.FuncSource,
					Returns:          promParser.ValueTypeVector,
					Operation:        "label_replace",
					Selector:         mustParse[*promParser.VectorSelector](t, `pod_status`, 28),
					Aggregation:      mustParse[*promParser.AggregateExpr](t, `sum by (pod) (pod_status)`, 14),
					FixedLabels:      true,
					IsConditional:    true,
					IncludedLabels:   []string{"pod"},
					GuaranteedLabels: []string{"cluster"},
					Call:             mustParse[*promParser.Call](t, `label_replace(sum by (pod) (pod_status) > 0, "cluster", "$1", "pod", "(.*)")`, 0),
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Query is using aggregation with `by(pod)`, only labels included inside `by(...)` will be present on the results.",
						},
					},
				},
			},
		},
		{
			expr: `(time() - my_metric) > 5*3600`,
			output: []utils.Source{
				{
					Type:          utils.SelectorSource,
					Returns:       promParser.ValueTypeVector,
					Selector:      mustParse[*promParser.VectorSelector](t, "my_metric", 10),
					IsConditional: true,
				},
			},
		},
		{
			expr: `up{instance="a", job="prometheus"} * ignoring(job) up{instance="a", job="pint"}`,
			output: []utils.Source{
				{
					Type:             utils.SelectorSource,
					Returns:          promParser.ValueTypeVector,
					Operation:        promParser.CardOneToOne.String(),
					Selector:         mustParse[*promParser.VectorSelector](t, `up{instance="a", job="prometheus"}`, 0),
					GuaranteedLabels: []string{"instance"},
					ExcludedLabels:   []string{"job"},
					ExcludeReason: map[string]utils.ExcludedLabel{
						"job": {
							Reason: "Query is using one-to-one vector matching with `ignoring(job)`, all labels included inside `ignoring(...)` will be removed on the results.",
						},
					},
					Joins: []utils.Join{
						{
							Src: utils.Source{
								Type:             utils.SelectorSource,
								Returns:          promParser.ValueTypeVector,
								Selector:         mustParse[*promParser.VectorSelector](t, `up{instance="a", job="pint"}`, 51),
								GuaranteedLabels: []string{"instance", "job"},
							},
						},
					},
				},
			},
		},
		{
			expr: `
avg without(router, colo_id, instance) (router_anycast_prefix_enabled{cidr_use_case!~".*offpeak.*"})
< 0.5 > 0
or sum without(router, colo_id, instance) (router_anycast_prefix_enabled{cidr_use_case=~".*tier1.*"})
< on() count(colo_router_tier:disabled_pops:max{tier="1",router=~"edge.*"}) * 0.4 > 0
or avg without(router, colo_id, instance) (router_anycast_prefix_enabled{cidr_use_case=~".*regional.*"})
< 0.1 > 0
`,
			output: []utils.Source{
				{
					Type:        utils.AggregateSource,
					Returns:     promParser.ValueTypeVector,
					Operation:   "avg",
					Selector:    mustParse[*promParser.VectorSelector](t, `router_anycast_prefix_enabled{cidr_use_case!~".*offpeak.*"}`, 41),
					Aggregation: mustParse[*promParser.AggregateExpr](t, `avg without(router, colo_id, instance) (router_anycast_prefix_enabled{cidr_use_case!~".*offpeak.*"})`, 1),

					ExcludedLabels: []string{"router", "colo_id", "instance"},
					ExcludeReason: map[string]utils.ExcludedLabel{
						"router": {
							Reason: "Query is using aggregation with `without(router, colo_id, instance)`, all labels included inside `without(...)` will be removed from the results.",
						},
						"colo_id": {
							Reason: "Query is using aggregation with `without(router, colo_id, instance)`, all labels included inside `without(...)` will be removed from the results.",
						},
						"instance": {
							Reason: "Query is using aggregation with `without(router, colo_id, instance)`, all labels included inside `without(...)` will be removed from the results.",
						},
					},
					IsConditional: true,
				},
				{
					Type:           utils.AggregateSource,
					Returns:        promParser.ValueTypeVector,
					Operation:      "sum",
					Selector:       mustParse[*promParser.VectorSelector](t, `router_anycast_prefix_enabled{cidr_use_case=~".*tier1.*"}`, 155),
					Aggregation:    mustParse[*promParser.AggregateExpr](t, `sum without(router, colo_id, instance) (router_anycast_prefix_enabled{cidr_use_case=~".*tier1.*"})`, 115),
					ExcludedLabels: []string{"router", "colo_id", "instance"},
					FixedLabels:    true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"router": {
							Reason: "Query is using aggregation with `without(router, colo_id, instance)`, all labels included inside `without(...)` will be removed from the results.",
						},
						"colo_id": {
							Reason: "Query is using aggregation with `without(router, colo_id, instance)`, all labels included inside `without(...)` will be removed from the results.",
						},
						"instance": {
							Reason: "Query is using aggregation with `without(router, colo_id, instance)`, all labels included inside `without(...)` will be removed from the results.",
						},
						"": {
							Reason: "Query is using one-to-one vector matching with `on()`, only labels included inside `on(...)` will be present on the results.",
						},
					},
					IsConditional: true,
					Joins: []utils.Join{
						{
							Src: utils.Source{
								Type:        utils.AggregateSource,
								Returns:     promParser.ValueTypeVector,
								Operation:   "count",
								Selector:    mustParse[*promParser.VectorSelector](t, `colo_router_tier:disabled_pops:max{tier="1",router=~"edge.*"}`, 227),
								Aggregation: mustParse[*promParser.AggregateExpr](t, `count(colo_router_tier:disabled_pops:max{tier="1",router=~"edge.*"})`, 221),
								FixedLabels: true,
								ExcludeReason: map[string]utils.ExcludedLabel{
									"": {
										Reason: "Query is using aggregation that removes all labels.",
									},
								},
							},
						},
					},
				},
				{
					Type:             utils.AggregateSource,
					Returns:          promParser.ValueTypeVector,
					Operation:        "avg",
					Selector:         mustParse[*promParser.VectorSelector](t, `router_anycast_prefix_enabled{cidr_use_case=~".*regional.*"}`, 343),
					Aggregation:      mustParse[*promParser.AggregateExpr](t, `avg without(router, colo_id, instance) (router_anycast_prefix_enabled{cidr_use_case=~".*regional.*"})`, 303),
					GuaranteedLabels: []string{"cidr_use_case"},
					ExcludedLabels:   []string{"router", "colo_id", "instance"},
					ExcludeReason: map[string]utils.ExcludedLabel{
						"router": {
							Reason: "Query is using aggregation with `without(router, colo_id, instance)`, all labels included inside `without(...)` will be removed from the results.",
						},
						"colo_id": {
							Reason: "Query is using aggregation with `without(router, colo_id, instance)`, all labels included inside `without(...)` will be removed from the results.",
						},
						"instance": {
							Reason: "Query is using aggregation with `without(router, colo_id, instance)`, all labels included inside `without(...)` will be removed from the results.",
						},
					},
					IsConditional: true,
				},
			},
		},
		{
			expr: `label_replace(sum(foo) without(instance), "instance", "none", "", "")`,
			output: []utils.Source{
				{
					Type:             utils.FuncSource,
					Returns:          promParser.ValueTypeVector,
					Operation:        "label_replace",
					Selector:         mustParse[*promParser.VectorSelector](t, `foo`, 18),
					Aggregation:      mustParse[*promParser.AggregateExpr](t, `sum(foo) without(instance)`, 14),
					GuaranteedLabels: []string{"instance"},
					ExcludeReason:    map[string]utils.ExcludedLabel{},
					Call:             mustParse[*promParser.Call](t, `label_replace(sum(foo) without(instance), "instance", "none", "", "")`, 0),
				},
			},
		},
		{
			expr: `
sum by (region, target, colo_name) (
    sum_over_time(probe_success{job="abc"}[5m])
	or
	vector(1)
) == 0`,
			output: []utils.Source{
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "sum",
					Selector:  mustParse[*promParser.VectorSelector](t, `probe_success{job="abc"}`, 56),
					Call:      mustParse[*promParser.Call](t, `sum_over_time(probe_success{job="abc"}[5m])`, 42),
					Aggregation: mustParse[*promParser.AggregateExpr](t, `
sum by (region, target, colo_name) (
    sum_over_time(probe_success{job="abc"}[5m])
	or
	vector(1)
)`, 0), // FIXME 0? should be 1
					IncludedLabels: []string{"region", "target", "colo_name"},
					FixedLabels:    true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Query is using aggregation with `by(region, target, colo_name)`, only labels included inside `by(...)` will be present on the results.",
						},
					},
					IsConditional: true,
				},
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "sum",
					Call:      mustParse[*promParser.Call](t, `vector(1)`, 91),
					Aggregation: mustParse[*promParser.AggregateExpr](t, `
sum by (region, target, colo_name) (
    sum_over_time(probe_success{job="abc"}[5m])
	or
	vector(1)
)`, 0), // FIXME 0? should be 1
					FixedLabels:    true,
					AlwaysReturns:  true,
					IsDead:         true,
					IsDeadReason:   "this query always evaluates to `1 == 0` which is not possible, so it will never return anything",
					ReturnedNumber: 1,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Calling `vector()` will return a vector value with no labels.",
						},
					},
					IsConditional: true,
				},
			},
		},
		{
			expr: `vector(1) or foo`,
			output: []utils.Source{
				{
					Type:           utils.FuncSource,
					Returns:        promParser.ValueTypeVector,
					Operation:      "vector",
					FixedLabels:    true,
					AlwaysReturns:  true,
					ReturnedNumber: 1,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Calling `vector()` will return a vector value with no labels.",
						},
					},
					Call: mustParse[*promParser.Call](t, "vector(1)", 0),
				},
				{
					Type:         utils.SelectorSource,
					Operation:    promParser.CardManyToMany.String(),
					Returns:      promParser.ValueTypeVector,
					Selector:     mustParse[*promParser.VectorSelector](t, "foo", 13),
					IsDead:       true,
					IsDeadReason: "the left hand side always returs something and so the right hand side is never used",
				},
			},
		},
		{
			expr: `vector(0) > 0`,
			output: []utils.Source{
				{
					Type:           utils.FuncSource,
					Returns:        promParser.ValueTypeVector,
					Operation:      "vector",
					FixedLabels:    true,
					AlwaysReturns:  true,
					ReturnedNumber: 0,
					IsDead:         true,
					IsDeadReason:   "this query always evaluates to `0 > 0` which is not possible, so it will never return anything",
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Calling `vector()` will return a vector value with no labels.",
						},
					},
					Call:          mustParse[*promParser.Call](t, "vector(0)", 0),
					IsConditional: true,
				},
			},
		},
		{
			expr: `vector(0) > vector(1)`,
			output: []utils.Source{
				{
					Type:           utils.FuncSource,
					Returns:        promParser.ValueTypeVector,
					Operation:      "vector",
					FixedLabels:    true,
					AlwaysReturns:  true,
					ReturnedNumber: 0,
					IsDead:         true,
					IsDeadReason:   "`vector(0) > vector(1)` always evaluates to `0 > 1` which is not possible, so it will never return anything",
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Calling `vector()` will return a vector value with no labels.",
						},
					},
					Call:          mustParse[*promParser.Call](t, "vector(0)", 0),
					IsConditional: true,
					Joins: []utils.Join{
						{
							Src: utils.Source{
								Type:           utils.FuncSource,
								Returns:        promParser.ValueTypeVector,
								Operation:      "vector",
								FixedLabels:    true,
								AlwaysReturns:  true,
								ReturnedNumber: 1,
								ExcludeReason: map[string]utils.ExcludedLabel{
									"": {
										Reason: "Calling `vector()` will return a vector value with no labels.",
									},
								},
								Call: mustParse[*promParser.Call](t, "vector(1)", 12),
							},
						},
					},
				},
			},
		},
		{
			expr: `sum(foo or vector(0)) > 0`,
			output: []utils.Source{
				{
					Type:        utils.AggregateSource,
					Returns:     promParser.ValueTypeVector,
					Operation:   "sum",
					Selector:    mustParse[*promParser.VectorSelector](t, `foo`, 4),
					Aggregation: mustParse[*promParser.AggregateExpr](t, `sum(foo or vector(0))`, 0),
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Query is using aggregation that removes all labels.",
						},
					},
					IsConditional: true,
				},
				{
					Type:           utils.AggregateSource,
					Returns:        promParser.ValueTypeVector,
					Operation:      "sum",
					Call:           mustParse[*promParser.Call](t, `vector(0)`, 11),
					Aggregation:    mustParse[*promParser.AggregateExpr](t, `sum(foo or vector(0))`, 0),
					FixedLabels:    true,
					AlwaysReturns:  true,
					ReturnedNumber: 0,
					IsDead:         true,
					IsDeadReason:   "this query always evaluates to `0 > 0` which is not possible, so it will never return anything",
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Query is using aggregation that removes all labels.",
						},
					},
					IsConditional: true,
				},
			},
		},
		{
			expr: `(sum(foo or vector(1)) > 0) == 2`,
			output: []utils.Source{
				{
					Type:        utils.AggregateSource,
					Returns:     promParser.ValueTypeVector,
					Operation:   "sum",
					Selector:    mustParse[*promParser.VectorSelector](t, `foo`, 5),
					Aggregation: mustParse[*promParser.AggregateExpr](t, `sum(foo or vector(1))`, 1),
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Query is using aggregation that removes all labels.",
						},
					},
					IsConditional: true,
				},
				{
					Type:           utils.AggregateSource,
					Returns:        promParser.ValueTypeVector,
					Operation:      "sum",
					Call:           mustParse[*promParser.Call](t, `vector(1)`, 12),
					Aggregation:    mustParse[*promParser.AggregateExpr](t, `sum(foo or vector(1))`, 1),
					FixedLabels:    true,
					AlwaysReturns:  true,
					ReturnedNumber: 1,
					IsDead:         true,
					IsDeadReason:   "this query always evaluates to `1 == 2` which is not possible, so it will never return anything",
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Query is using aggregation that removes all labels.",
						},
					},
					IsConditional: true,
				},
			},
		},
		{
			expr: `(sum(foo or vector(1)) > 0) != 2`,
			output: []utils.Source{
				{
					Type:        utils.AggregateSource,
					Returns:     promParser.ValueTypeVector,
					Operation:   "sum",
					Selector:    mustParse[*promParser.VectorSelector](t, `foo`, 5),
					Aggregation: mustParse[*promParser.AggregateExpr](t, `sum(foo or vector(1))`, 1),
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Query is using aggregation that removes all labels.",
						},
					},
					IsConditional: true,
				},
				{
					Type:           utils.AggregateSource,
					Returns:        promParser.ValueTypeVector,
					Operation:      "sum",
					Call:           mustParse[*promParser.Call](t, `vector(1)`, 12),
					Aggregation:    mustParse[*promParser.AggregateExpr](t, `sum(foo or vector(1))`, 1),
					FixedLabels:    true,
					AlwaysReturns:  true,
					ReturnedNumber: 1,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Query is using aggregation that removes all labels.",
						},
					},
					IsConditional: true,
				},
			},
		},
		{
			expr: `(sum(foo or vector(2)) > 0) != 2`,
			output: []utils.Source{
				{
					Type:        utils.AggregateSource,
					Returns:     promParser.ValueTypeVector,
					Operation:   "sum",
					Selector:    mustParse[*promParser.VectorSelector](t, `foo`, 5),
					Aggregation: mustParse[*promParser.AggregateExpr](t, `sum(foo or vector(2))`, 1),
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Query is using aggregation that removes all labels.",
						},
					},
					IsConditional: true,
				},
				{
					Type:           utils.AggregateSource,
					Returns:        promParser.ValueTypeVector,
					Operation:      "sum",
					Call:           mustParse[*promParser.Call](t, `vector(2)`, 12),
					Aggregation:    mustParse[*promParser.AggregateExpr](t, `sum(foo or vector(2))`, 1),
					FixedLabels:    true,
					AlwaysReturns:  true,
					ReturnedNumber: 2,
					IsDead:         true,
					IsDeadReason:   "this query always evaluates to `2 != 2` which is not possible, so it will never return anything",
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Query is using aggregation that removes all labels.",
						},
					},
					IsConditional: true,
				},
			},
		},
		{
			expr: `(sum(sometimes{foo!="bar"} or vector(0)))
or
((bob > 10) or sum(foo) or vector(1))`,
			output: []utils.Source{
				{
					Type:        utils.AggregateSource,
					Returns:     promParser.ValueTypeVector,
					Operation:   "sum",
					Selector:    mustParse[*promParser.VectorSelector](t, `sometimes{foo!="bar"}`, 5),
					Aggregation: mustParse[*promParser.AggregateExpr](t, `sum(sometimes{foo!="bar"} or vector(0) )`, 1), // FIXME extra end

					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation that removes all labels.",
							Fragment: posrange.PositionRange{Start: 1, End: 1}, // FIXME bogus )
						},
					},
				},
				{
					Type:           utils.AggregateSource,
					Returns:        promParser.ValueTypeVector,
					Operation:      "sum",
					Call:           mustParse[*promParser.Call](t, `vector(0)`, 30),
					Aggregation:    mustParse[*promParser.AggregateExpr](t, `sum(sometimes{foo!="bar"} or vector(0) )`, 1), // FIXME extra end
					AlwaysReturns:  true,
					ReturnedNumber: 0,
					FixedLabels:    true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation that removes all labels.",
							Fragment: posrange.PositionRange{Start: 1, End: 1}, // FIXME bogus )
						},
					},
				},
				{
					Type:          utils.SelectorSource,
					Operation:     promParser.CardManyToMany.String(),
					Returns:       promParser.ValueTypeVector,
					Selector:      mustParse[*promParser.VectorSelector](t, `bob`, 47),
					IsConditional: true,
				},
				{
					Type:        utils.AggregateSource,
					Returns:     promParser.ValueTypeVector,
					Operation:   "sum",
					Selector:    mustParse[*promParser.VectorSelector](t, `foo`, 64),
					Aggregation: mustParse[*promParser.AggregateExpr](t, `sum(foo)`, 60),
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Query is using aggregation that removes all labels.",
						},
					},
				},
				{
					Type:           utils.FuncSource,
					Returns:        promParser.ValueTypeVector,
					Operation:      "vector",
					AlwaysReturns:  true,
					ReturnedNumber: 1,
					FixedLabels:    true,
					Call:           mustParse[*promParser.Call](t, "vector(1)", 72),
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Calling `vector()` will return a vector value with no labels.",
						},
					},
				},
			},
		},
		{
			expr: `
(
	sum(sometimes{foo!="bar"})
	or
	vector(1)
) and (
	((bob > 10) or sum(bar))
	or
	notfound > 0
)`,
			output: []utils.Source{
				{
					Type:        utils.AggregateSource,
					Returns:     promParser.ValueTypeVector,
					Operation:   "sum",
					Selector:    mustParse[*promParser.VectorSelector](t, `sometimes{foo!="bar"}`, 8),
					Aggregation: mustParse[*promParser.AggregateExpr](t, `sum(sometimes{foo!="bar"})`, 4),
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Query is using aggregation that removes all labels.",
						},
					},
					Joins: []utils.Join{
						{
							Src: utils.Source{
								Type:          utils.SelectorSource,
								Operation:     promParser.CardManyToMany.String(),
								Returns:       promParser.ValueTypeVector,
								Selector:      mustParse[*promParser.VectorSelector](t, `bob`, 57),
								IsConditional: true,
							},
						},
						{
							Src: utils.Source{
								Type:        utils.AggregateSource,
								Returns:     promParser.ValueTypeVector,
								Operation:   "sum",
								Selector:    mustParse[*promParser.VectorSelector](t, `bar`, 74),
								Aggregation: mustParse[*promParser.AggregateExpr](t, `sum(bar )`, 70),
								FixedLabels: true,
								ExcludeReason: map[string]utils.ExcludedLabel{
									"": {
										Reason:   "Query is using aggregation that removes all labels.",
										Fragment: posrange.PositionRange{Start: 1, End: 1}, // FIXME bogus )
									},
								},
							},
						},
						{
							Src: utils.Source{
								Type:          utils.SelectorSource,
								Operation:     promParser.CardManyToMany.String(),
								Returns:       promParser.ValueTypeVector,
								Selector:      mustParse[*promParser.VectorSelector](t, `notfound`, 85),
								IsConditional: true,
							},
						},
					},
				},
				{
					Type:           utils.FuncSource,
					Returns:        promParser.ValueTypeVector,
					Operation:      "vector",
					AlwaysReturns:  true,
					ReturnedNumber: 1,
					FixedLabels:    true,
					Call:           mustParse[*promParser.Call](t, "vector(1)", 36),
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Calling `vector()` will return a vector value with no labels.",
						},
					},
					Joins: []utils.Join{
						{
							Src: utils.Source{
								Type:          utils.SelectorSource,
								Operation:     promParser.CardManyToMany.String(),
								Returns:       promParser.ValueTypeVector,
								Selector:      mustParse[*promParser.VectorSelector](t, `bob`, 57),
								IsConditional: true,
							},
						},
						{
							Src: utils.Source{
								Type:        utils.AggregateSource,
								Returns:     promParser.ValueTypeVector,
								Operation:   "sum",
								Selector:    mustParse[*promParser.VectorSelector](t, `bar`, 74),
								Aggregation: mustParse[*promParser.AggregateExpr](t, `sum(bar )`, 70), // FIXME extra end
								FixedLabels: true,
								ExcludeReason: map[string]utils.ExcludedLabel{
									"": {
										Reason:   "Query is using aggregation that removes all labels.",
										Fragment: posrange.PositionRange{Start: 1, End: 1}, // FIXME bogus )
									},
								},
							},
						},
						{
							Src: utils.Source{
								Type:          utils.SelectorSource,
								Operation:     promParser.CardManyToMany.String(),
								Returns:       promParser.ValueTypeVector,
								Selector:      mustParse[*promParser.VectorSelector](t, `notfound`, 85),
								IsConditional: true,
							},
						},
					},
				},
			},
		},
		{
			expr: "foo offset 5m > 5",
			output: []utils.Source{
				{
					Type:          utils.SelectorSource,
					Returns:       promParser.ValueTypeVector,
					Selector:      mustParse[*promParser.VectorSelector](t, "foo offset 5m", 0),
					IsConditional: true,
				},
			},
		},
		{
			expr: `
(rate(metric2[5m]) or vector(0)) +
(rate(metric1[5m]) or vector(1)) +
(rate(metric3{log_name="samplerd"}[5m]) or vector(2)) > 0
`,
			output: []utils.Source{
				{
					Type:          utils.FuncSource,
					Operation:     "rate",
					Returns:       promParser.ValueTypeVector,
					Selector:      mustParse[*promParser.VectorSelector](t, "metric2", 7),
					Call:          mustParse[*promParser.Call](t, "rate(metric2[5m])", 2),
					IsConditional: true,
					Joins: []utils.Join{
						{
							Src: utils.Source{
								Type:      utils.FuncSource,
								Operation: "rate",
								Returns:   promParser.ValueTypeVector,
								Selector:  mustParse[*promParser.VectorSelector](t, "metric1", 42),
								Call:      mustParse[*promParser.Call](t, "rate(metric1[5m])", 37),
							},
						},
						{
							Src: utils.Source{
								Type:           utils.FuncSource,
								Returns:        promParser.ValueTypeVector,
								Operation:      "vector",
								AlwaysReturns:  true,
								ReturnedNumber: 1,
								FixedLabels:    true,
								Call:           mustParse[*promParser.Call](t, "vector(1)", 58),
								ExcludeReason: map[string]utils.ExcludedLabel{
									"": {
										Reason: "Calling `vector()` will return a vector value with no labels.",
									},
								},
							},
						},
						{
							Src: utils.Source{
								Type:             utils.FuncSource,
								Operation:        "rate",
								Returns:          promParser.ValueTypeVector,
								Selector:         mustParse[*promParser.VectorSelector](t, `metric3{log_name="samplerd"}`, 77),
								Call:             mustParse[*promParser.Call](t, `rate(metric3{log_name="samplerd"}[5m])`, 72),
								GuaranteedLabels: []string{"log_name"},
							},
						},
						{
							Src: utils.Source{
								Type:           utils.FuncSource,
								Returns:        promParser.ValueTypeVector,
								Operation:      "vector",
								AlwaysReturns:  true,
								ReturnedNumber: 2,
								FixedLabels:    true,
								Call:           mustParse[*promParser.Call](t, "vector(2)", 114),
								ExcludeReason: map[string]utils.ExcludedLabel{
									"": {
										Reason: "Calling `vector()` will return a vector value with no labels.",
									},
								},
							},
						},
					},
				},
				{
					Type:           utils.FuncSource,
					Returns:        promParser.ValueTypeVector,
					Operation:      "vector",
					AlwaysReturns:  true,
					ReturnedNumber: 3,
					FixedLabels:    true,
					Call:           mustParse[*promParser.Call](t, "vector(0)", 23),
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Calling `vector()` will return a vector value with no labels.",
						},
					},
					IsConditional: true,
					Joins: []utils.Join{
						{
							Src: utils.Source{
								Type:      utils.FuncSource,
								Operation: "rate",
								Returns:   promParser.ValueTypeVector,
								Selector:  mustParse[*promParser.VectorSelector](t, "metric1", 42),
								Call:      mustParse[*promParser.Call](t, "rate(metric1[5m])", 37),
							},
						},
						{
							Src: utils.Source{
								Type:           utils.FuncSource,
								Returns:        promParser.ValueTypeVector,
								Operation:      "vector",
								AlwaysReturns:  true,
								ReturnedNumber: 1,
								FixedLabels:    true,
								Call:           mustParse[*promParser.Call](t, "vector(1)", 58),
								ExcludeReason: map[string]utils.ExcludedLabel{
									"": {
										Reason: "Calling `vector()` will return a vector value with no labels.",
									},
								},
							},
						},
						{
							Src: utils.Source{
								Type:             utils.FuncSource,
								Operation:        "rate",
								Returns:          promParser.ValueTypeVector,
								Selector:         mustParse[*promParser.VectorSelector](t, `metric3{log_name="samplerd"}`, 77),
								Call:             mustParse[*promParser.Call](t, `rate(metric3{log_name="samplerd"}[5m])`, 72),
								GuaranteedLabels: []string{"log_name"},
							},
						},
						{
							Src: utils.Source{
								Type:           utils.FuncSource,
								Returns:        promParser.ValueTypeVector,
								Operation:      "vector",
								AlwaysReturns:  true,
								ReturnedNumber: 2,
								FixedLabels:    true,
								Call:           mustParse[*promParser.Call](t, "vector(2)", 114),
								ExcludeReason: map[string]utils.ExcludedLabel{
									"": {
										Reason: "Calling `vector()` will return a vector value with no labels.",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			expr: `label_replace(vector(1), "nexthop_tag", "$1", "nexthop", "(.+)")`,
			output: []utils.Source{
				{
					Type:             utils.FuncSource,
					Returns:          promParser.ValueTypeVector,
					Operation:        "label_replace",
					AlwaysReturns:    true,
					FixedLabels:      true,
					ReturnedNumber:   1,
					GuaranteedLabels: []string{"nexthop_tag"},
					Call:             mustParse[*promParser.Call](t, `label_replace(vector(1), "nexthop_tag", "$1", "nexthop", "(.+)")`, 0),
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Calling `vector()` will return a vector value with no labels.",
						},
					},
				},
			},
		},
		{
			expr: `(sum(foo{job="myjob"}))`,
			output: []utils.Source{
				{
					Type:        utils.AggregateSource,
					Returns:     promParser.ValueTypeVector,
					Operation:   "sum",
					Selector:    mustParse[*promParser.VectorSelector](t, `foo{job="myjob"}`, 5),
					Aggregation: mustParse[*promParser.AggregateExpr](t, `sum(foo{job="myjob"} )`, 1),
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation that removes all labels.",
							Fragment: posrange.PositionRange{Start: 1, End: 1}, // FIXME bogus )
						},
					},
				},
			},
		},
		{
			expr: `(-foo{job="myjob"})`,
			output: []utils.Source{
				{
					Type:             utils.SelectorSource,
					Returns:          promParser.ValueTypeVector,
					Selector:         mustParse[*promParser.VectorSelector](t, `foo{job="myjob"}`, 2),
					GuaranteedLabels: []string{"job"},
				},
			},
		},
		{
			expr: "\n((( group(vector(0)) ))) > 0",
			output: []utils.Source{
				{
					Type:           utils.AggregateSource,
					Operation:      "group",
					Returns:        promParser.ValueTypeVector,
					FixedLabels:    true,
					AlwaysReturns:  true,
					IsConditional:  true,
					IsDead:         true,
					IsDeadReason:   "this query always evaluates to `0 > 0` which is not possible, so it will never return anything",
					ReturnedNumber: 0,
					Call:           mustParse[*promParser.Call](t, `vector(0)`, 11),
					Aggregation:    mustParse[*promParser.AggregateExpr](t, "group(vector(0)  )", 5), // FIXME
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation that removes all labels.",
							Fragment: posrange.PositionRange{Start: 1, End: 1}, // FIXME bogus )
						},
					},
				},
			},
		},
		{
			expr: "1 > bool 5",
			output: []utils.Source{
				{
					Type:           utils.NumberSource,
					Returns:        promParser.ValueTypeScalar,
					FixedLabels:    true,
					AlwaysReturns:  true,
					IsConditional:  true,
					IsDead:         true,
					IsDeadReason:   "this query always evaluates to `1 > 5` which is not possible, so it will never return anything",
					ReturnedNumber: 1,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "This query returns a number value with no labels.",
						},
					},
				},
			},
		},
		{
			expr: `prometheus_ready{job="prometheus"} unless vector(0)`,
			output: []utils.Source{
				{
					Type:             utils.SelectorSource,
					Operation:        promParser.CardManyToMany.String(),
					Returns:          promParser.ValueTypeVector,
					Selector:         mustParse[*promParser.VectorSelector](t, `prometheus_ready{job="prometheus"}`, 0),
					GuaranteedLabels: []string{"job"},
					Unless: []utils.Join{
						{
							Src: utils.Source{
								Type:           utils.FuncSource,
								Returns:        promParser.ValueTypeVector,
								Operation:      "vector",
								AlwaysReturns:  true,
								ReturnedNumber: 0,
								FixedLabels:    true,
								Call:           mustParse[*promParser.Call](t, "vector(0)", 42),
								ExcludeReason: map[string]utils.ExcludedLabel{
									"": {
										Reason: "Calling `vector()` will return a vector value with no labels.",
									},
								},
								IsDead:       true,
								IsDeadReason: "the right hand side will never be matched because it doesn't have the `job` label while the left hand side will",
							},
						},
					},
				},
			},
		},
		{
			expr: `prometheus_ready{job="prometheus"} unless on() vector(0)`,
			output: []utils.Source{
				{
					Type:             utils.SelectorSource,
					Operation:        promParser.CardManyToMany.String(),
					Returns:          promParser.ValueTypeVector,
					IsDead:           true,
					IsDeadReason:     "this query will never return anything because the `unless` query always returns something",
					Selector:         mustParse[*promParser.VectorSelector](t, `prometheus_ready{job="prometheus"}`, 0),
					GuaranteedLabels: []string{"job"},
					Unless: []utils.Join{
						{
							Src: utils.Source{
								Type:           utils.FuncSource,
								Returns:        promParser.ValueTypeVector,
								Operation:      "vector",
								AlwaysReturns:  true,
								ReturnedNumber: 0,
								FixedLabels:    true,
								Call:           mustParse[*promParser.Call](t, "vector(0)", 47),
								ExcludeReason: map[string]utils.ExcludedLabel{
									"": {
										Reason: "Calling `vector()` will return a vector value with no labels.",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			expr: `prometheus_ready{job="prometheus"} unless on(job) vector(0)`,
			output: []utils.Source{
				{
					Type:             utils.SelectorSource,
					Operation:        promParser.CardManyToMany.String(),
					Returns:          promParser.ValueTypeVector,
					Selector:         mustParse[*promParser.VectorSelector](t, `prometheus_ready{job="prometheus"}`, 0),
					IncludedLabels:   []string{"job"},
					GuaranteedLabels: []string{"job"},
					Unless: []utils.Join{
						{
							Src: utils.Source{
								Type:           utils.FuncSource,
								Returns:        promParser.ValueTypeVector,
								Operation:      "vector",
								AlwaysReturns:  true,
								IsDead:         true,
								IsDeadReason:   "the right hand side will never be matched because it doesn't have the `job` label from `on(...)`",
								ReturnedNumber: 0,
								FixedLabels:    true,
								Call:           mustParse[*promParser.Call](t, "vector(0)", 50),
								ExcludeReason: map[string]utils.ExcludedLabel{
									"": {
										Reason: "Calling `vector()` will return a vector value with no labels.",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			expr: `
max by (instance, cluster) (cf_node_role{kubernetes_role="master",role="kubernetes"})
unless
	sum by (instance, cluster) (time() - node_systemd_timer_last_trigger_seconds{name=~"etcd-defrag-.*.timer"})
  	* on (instance) group_left (cluster)
    cf_node_role{kubernetes_role="master",role="kubernetes"}
`,
			output: []utils.Source{
				{
					Type:           utils.AggregateSource,
					Operation:      "max",
					Returns:        promParser.ValueTypeVector,
					Selector:       mustParse[*promParser.VectorSelector](t, `cf_node_role{kubernetes_role="master",role="kubernetes"}`, 29),
					Aggregation:    mustParse[*promParser.AggregateExpr](t, `max by (instance, cluster) (cf_node_role{kubernetes_role="master",role="kubernetes"})`, 1),
					FixedLabels:    true,
					IncludedLabels: []string{"instance", "cluster"},
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Query is using aggregation with `by(instance, cluster)`, only labels included inside `by(...)` will be present on the results.",
						},
					},
					Unless: []utils.Join{
						{
							Src: utils.Source{
								Type:           utils.AggregateSource,
								Operation:      "sum",
								Returns:        promParser.ValueTypeVector,
								Selector:       mustParse[*promParser.VectorSelector](t, `node_systemd_timer_last_trigger_seconds{name=~"etcd-defrag-.*.timer"}`, 132),
								Aggregation:    mustParse[*promParser.AggregateExpr](t, `sum by (instance, cluster) (time() - node_systemd_timer_last_trigger_seconds{name=~"etcd-defrag-.*.timer"})`, 95),
								FixedLabels:    true,
								IncludedLabels: []string{"instance", "cluster"},
								ExcludeReason: map[string]utils.ExcludedLabel{
									"": {
										Reason: "Query is using aggregation with `by(instance, cluster)`, only labels included inside `by(...)` will be present on the results.",
									},
								},
								Joins: []utils.Join{
									{
										Src: utils.Source{
											Type:     utils.SelectorSource,
											Returns:  promParser.ValueTypeVector,
											Selector: mustParse[*promParser.VectorSelector](t, `cf_node_role{kubernetes_role="master",role="kubernetes"}`, 247),
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			expr: `foo{a="1"} * on() bar{b="2"}`,
			output: []utils.Source{
				{
					Type:        utils.SelectorSource,
					Returns:     promParser.ValueTypeVector,
					Operation:   promParser.CardOneToOne.String(),
					Selector:    mustParse[*promParser.VectorSelector](t, `foo{a="1"}`, 0),
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Query is using one-to-one vector matching with `on()`, only labels included inside `on(...)` will be present on the results.",
						},
					},
					Joins: []utils.Join{
						{
							Src: utils.Source{
								Type:             utils.SelectorSource,
								Returns:          promParser.ValueTypeVector,
								GuaranteedLabels: []string{"b"},
								Selector:         mustParse[*promParser.VectorSelector](t, `bar{b="2"}`, 18),
							},
						},
					},
				},
			},
		},
		{
			expr: `foo{a="1"} * on(instance) group_left(c,d) bar{b="2"}`,
			output: []utils.Source{
				{
					Type:             utils.SelectorSource,
					Returns:          promParser.ValueTypeVector,
					Operation:        promParser.CardManyToOne.String(),
					IncludedLabels:   []string{"c", "d", "instance"},
					GuaranteedLabels: []string{"a"},
					Selector:         mustParse[*promParser.VectorSelector](t, `foo{a="1"}`, 0),
					Joins: []utils.Join{
						{
							Src: utils.Source{
								Type:     utils.SelectorSource,
								Returns:  promParser.ValueTypeVector,
								Selector: mustParse[*promParser.VectorSelector](t, `bar{b="2"}`, 42),
							},
						},
					},
				},
			},
		},
		{
			expr: `foo{a="1"} * on(instance) group_right(c,d) bar{b="2"}`,
			output: []utils.Source{
				{
					Type:             utils.SelectorSource,
					Returns:          promParser.ValueTypeVector,
					Operation:        promParser.CardOneToMany.String(),
					IncludedLabels:   []string{"c", "d", "instance"},
					GuaranteedLabels: []string{"b"},
					Selector:         mustParse[*promParser.VectorSelector](t, `bar{b="2"}`, 43),
					Joins: []utils.Join{
						{
							Src: utils.Source{
								Type:     utils.SelectorSource,
								Returns:  promParser.ValueTypeVector,
								Selector: mustParse[*promParser.VectorSelector](t, `foo{a="1"}`, 0),
							},
						},
					},
				},
			},
		},
		{
			expr: `foo{a="1"} * on(instance) sum(bar{b="2"})`,
			output: []utils.Source{
				{
					Type:           utils.SelectorSource,
					Returns:        promParser.ValueTypeVector,
					Operation:      promParser.CardOneToOne.String(),
					IncludedLabels: []string{"instance"},
					Selector:       mustParse[*promParser.VectorSelector](t, `foo{a="1"}`, 0),
					FixedLabels:    true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Query is using one-to-one vector matching with `on(instance)`, only labels included inside `on(...)` will be present on the results.",
						},
					},
					Joins: []utils.Join{
						{
							Src: utils.Source{
								Type:        utils.AggregateSource,
								Operation:   "sum",
								Returns:     promParser.ValueTypeVector,
								FixedLabels: true,
								Selector:    mustParse[*promParser.VectorSelector](t, `bar{b="2"}`, 30),
								Aggregation: mustParse[*promParser.AggregateExpr](t, `sum(bar{b="2"})`, 26),
								ExcludeReason: map[string]utils.ExcludedLabel{
									"": {
										Reason: "Query is using aggregation that removes all labels.",
									},
								},
								IsDead:       true,
								IsDeadReason: "the right hand side will never be matched because it doesn't have the `instance` label from `on(...)`",
							},
						},
					},
				},
			},
		},
		{
			expr: `foo{a="1"} * on(instance) group_left(c,d) sum(bar{b="2"})`,
			output: []utils.Source{
				{
					Type:             utils.SelectorSource,
					Returns:          promParser.ValueTypeVector,
					Operation:        promParser.CardManyToOne.String(),
					IncludedLabels:   []string{"c", "d", "instance"},
					GuaranteedLabels: []string{"a"},
					Selector:         mustParse[*promParser.VectorSelector](t, `foo{a="1"}`, 0),
					Joins: []utils.Join{
						{
							Src: utils.Source{
								Type:        utils.AggregateSource,
								Operation:   "sum",
								Returns:     promParser.ValueTypeVector,
								FixedLabels: true,
								Selector:    mustParse[*promParser.VectorSelector](t, `bar{b="2"}`, 46),
								Aggregation: mustParse[*promParser.AggregateExpr](t, `sum(bar{b="2"})`, 42),
								ExcludeReason: map[string]utils.ExcludedLabel{
									"": {
										Reason: "Query is using aggregation that removes all labels.",
									},
								},
								IsDead:       true,
								IsDeadReason: "the right hand side will never be matched because it doesn't have the `instance` label from `on(...)`",
							},
						},
					},
				},
			},
		},
		{
			expr: `sum(foo{a="1"}) * on(instance) group_right(c,d) bar{b="2"}`,
			output: []utils.Source{
				{
					Type:             utils.SelectorSource,
					Returns:          promParser.ValueTypeVector,
					Operation:        promParser.CardOneToMany.String(),
					IncludedLabels:   []string{"c", "d", "instance"},
					GuaranteedLabels: []string{"b"},
					Selector:         mustParse[*promParser.VectorSelector](t, `bar{b="2"}`, 48),
					Joins: []utils.Join{
						{
							Src: utils.Source{
								Type:        utils.AggregateSource,
								Operation:   "sum",
								Returns:     promParser.ValueTypeVector,
								FixedLabels: true,
								Selector:    mustParse[*promParser.VectorSelector](t, `foo{a="1"}`, 4),
								Aggregation: mustParse[*promParser.AggregateExpr](t, `sum(foo{a="1"})`, 0),
								ExcludeReason: map[string]utils.ExcludedLabel{
									"": {
										Reason: "Query is using aggregation that removes all labels.",
									},
								},
								IsDead:       true,
								IsDeadReason: "the left hand side will never be matched because it doesn't have the `instance` label from `on(...)`",
							},
						},
					},
				},
			},
		},
		{
			expr: `foo{a="1"} * on(instance) group_left(c,d) sum(bar{b="2"}) without(instance)`,
			output: []utils.Source{
				{
					Type:             utils.SelectorSource,
					Returns:          promParser.ValueTypeVector,
					Operation:        promParser.CardManyToOne.String(),
					IncludedLabels:   []string{"c", "d", "instance"},
					GuaranteedLabels: []string{"a"},
					Selector:         mustParse[*promParser.VectorSelector](t, `foo{a="1"}`, 0),
					Joins: []utils.Join{
						{
							Src: utils.Source{
								Type:           utils.AggregateSource,
								Operation:      "sum",
								Returns:        promParser.ValueTypeVector,
								Selector:       mustParse[*promParser.VectorSelector](t, `bar{b="2"}`, 46),
								Aggregation:    mustParse[*promParser.AggregateExpr](t, `sum(bar{b="2"}) without(instance)`, 42),
								ExcludedLabels: []string{"instance"},
								ExcludeReason: map[string]utils.ExcludedLabel{
									"instance": {
										Reason: "Query is using aggregation with `without(instance)`, all labels included inside `without(...)` will be removed from the results.",
									},
								},
								IsDead:       true,
								IsDeadReason: "the right hand side will never be matched because it doesn't have the `instance` label from `on(...)`",
							},
						},
					},
				},
			},
		},
		{
			expr: `sum(foo{a="1"}) without(instance) * on(instance) group_right(c,d) bar{b="2"}`,
			output: []utils.Source{
				{
					Type:             utils.SelectorSource,
					Returns:          promParser.ValueTypeVector,
					Operation:        promParser.CardOneToMany.String(),
					IncludedLabels:   []string{"c", "d", "instance"},
					GuaranteedLabels: []string{"b"},
					Selector:         mustParse[*promParser.VectorSelector](t, `bar{b="2"}`, 66),
					Joins: []utils.Join{
						{
							Src: utils.Source{
								Type:           utils.AggregateSource,
								Operation:      "sum",
								Returns:        promParser.ValueTypeVector,
								Selector:       mustParse[*promParser.VectorSelector](t, `foo{a="1"}`, 4),
								Aggregation:    mustParse[*promParser.AggregateExpr](t, `sum(foo{a="1"}) without(instance)`, 0),
								ExcludedLabels: []string{"instance"},
								ExcludeReason: map[string]utils.ExcludedLabel{
									"instance": {
										Reason: "Query is using aggregation with `without(instance)`, all labels included inside `without(...)` will be removed from the results.",
									},
								},
								IsDead:       true,
								IsDeadReason: "the left hand side will never be matched because it doesn't have the `instance` label from `on(...)`",
							},
						},
					},
				},
			},
		},
		{
			expr: `
 max without (source_instance) (
   increase(kernel_device_io_errors_total{device!~"loop.+"}[120m]) > 3 unless on(instance, device) (
     increase(kernel_device_io_soft_errors_total{device!~"loop.+"}[125m])*2 > increase(kernel_device_io_errors_total[120m])
   )
   and on(device, instance) absent(node_disk_info)
 ) * on(instance) group_left(group) label_replace(salt_highstate_runner_configured_minions, "instance", "$1", "minion", "(.+)")
`,
			output: []utils.Source{
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "max",
					Selector:  mustParse[*promParser.VectorSelector](t, `kernel_device_io_errors_total{device!~"loop.+"}`, 46),
					Call:      mustParse[*promParser.Call](t, `increase(kernel_device_io_errors_total{device!~"loop.+"}[120m])`, 37),
					Aggregation: mustParse[*promParser.AggregateExpr](t,
						`max without (source_instance) (
   increase(kernel_device_io_errors_total{device!~"loop.+"}[120m]) > 3 unless on(instance, device) (
     increase(kernel_device_io_soft_errors_total{device!~"loop.+"}[125m])*2 > increase(kernel_device_io_errors_total[120m])
   )
   and on(device, instance) absent(node_disk_info)
 )`, 2),
					IncludedLabels: []string{"instance", "device", "group"},
					ExcludedLabels: []string{"source_instance"},
					ExcludeReason: map[string]utils.ExcludedLabel{
						"source_instance": {
							Reason: "Query is using aggregation with `without(source_instance)`, all labels included inside `without(...)` will be removed from the results.",
						},
					},
					Joins: []utils.Join{
						{
							Src: utils.Source{
								Type:         utils.FuncSource,
								Returns:      promParser.ValueTypeVector,
								Operation:    "absent",
								FixedLabels:  true,
								Selector:     mustParse[*promParser.VectorSelector](t, `node_disk_info`, 299),
								Call:         mustParse[*promParser.Call](t, `absent(node_disk_info)`, 292),
								IsDead:       true,
								IsDeadReason: "the right hand side will never be matched because it doesn't have the `device` label from `on(...)`",
								ExcludeReason: map[string]utils.ExcludedLabel{
									"": {
										Reason: "The [absent()](https://prometheus.io/docs/prometheus/latest/querying/functions/#absent) function is used to check if provided query doesn't match any time series.\nYou will only get any results back if the metric selector you pass doesn't match anything.\nSince there are no matching time series there are also no labels. If some time series is missing you cannot read its labels.\nThis means that the only labels you can get back from absent call are the ones you pass to it.\nIf you're hoping to get instance specific labels this way and alert when some target is down then that won't work, use the `up` metric instead.",
									},
								},
							},
						},
						{
							Src: utils.Source{
								Type:      utils.FuncSource,
								Returns:   promParser.ValueTypeVector,
								Operation: "label_replace",
								Selector:  mustParse[*promParser.VectorSelector](t, `salt_highstate_runner_configured_minions`, 365),
								Call:      mustParse[*promParser.Call](t, `label_replace(salt_highstate_runner_configured_minions, "instance", "$1", "minion", "(.+)")`, 351),
							},
						},
					},
					Unless: []utils.Join{
						{
							Src: utils.Source{
								Type:          utils.FuncSource,
								Returns:       promParser.ValueTypeVector,
								Operation:     "increase",
								IsConditional: true,
								Selector:      mustParse[*promParser.VectorSelector](t, `kernel_device_io_soft_errors_total{device!~"loop.+"}`, 149),
								Call:          mustParse[*promParser.Call](t, `increase(kernel_device_io_soft_errors_total{device!~"loop.+"}[125m])`, 140),
								Joins: []utils.Join{
									{
										Src: utils.Source{
											Type:      utils.FuncSource,
											Returns:   promParser.ValueTypeVector,
											Operation: "increase",
											Selector:  mustParse[*promParser.VectorSelector](t, `kernel_device_io_errors_total`, 222),
											Call:      mustParse[*promParser.Call](t, `increase(kernel_device_io_errors_total[120m])`, 213),
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			expr: `sum(foo{a="1"}) by(job) * on() bar{b="2"}`,
			output: []utils.Source{
				{
					Type:        utils.AggregateSource,
					Returns:     promParser.ValueTypeVector,
					Operation:   "sum",
					Selector:    mustParse[*promParser.VectorSelector](t, `foo{a="1"}`, 4),
					Aggregation: mustParse[*promParser.AggregateExpr](t, `sum(foo{a="1"}) by(job)`, 0),
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason: "Query is using one-to-one vector matching with `on()`, only labels included inside `on(...)` will be present on the results.",
						},
					},
					Joins: []utils.Join{
						{
							Src: utils.Source{
								Type:             utils.SelectorSource,
								Returns:          promParser.ValueTypeVector,
								GuaranteedLabels: []string{"b"},
								Selector:         mustParse[*promParser.VectorSelector](t, `bar{b="2"}`, 31),
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.expr, func(t *testing.T) {
			n, err := parser.DecodeExpr(tc.expr)
			if err != nil {
				t.Error(err)
				t.FailNow()
			}
			output := utils.LabelsSource(tc.expr, n.Expr)
			require.Len(t, output, len(tc.output))
			if diff := cmp.Diff(tc.output, output,
				cmpopts.EquateNaNs(),
				cmpopts.IgnoreUnexported(labels.Matcher{}),
				cmpopts.IgnoreFields(utils.ExcludedLabel{}, "Fragment"),
				cmpopts.IgnoreFields(utils.Source{}, "Position"),
			); diff != "" {
				t.Errorf("utils.LabelsSource() returned wrong output (-want +got):\n%s", diff)
				return
			}
		})
	}
}

func TestLabelsSourceCallCoverage(t *testing.T) {
	for name, def := range promParser.Functions {
		t.Run(name, func(t *testing.T) {
			if def.Experimental {
				t.SkipNow()
			}

			var b strings.Builder
			b.WriteString(name)
			b.WriteRune('(')
			for i, at := range def.ArgTypes {
				if i > 0 {
					b.WriteString(", ")
				}
				switch at {
				case promParser.ValueTypeNone:
				case promParser.ValueTypeScalar:
					b.WriteRune('1')
				case promParser.ValueTypeVector:
					b.WriteString("http_requests_total")
				case promParser.ValueTypeMatrix:
					b.WriteString("http_requests_total[2m]")
				case promParser.ValueTypeString:
					b.WriteString(`"foo"`)
				}
			}
			b.WriteRune(')')

			n, err := parser.DecodeExpr(b.String())
			if err != nil {
				t.Error(err)
				t.FailNow()
			}
			output := utils.LabelsSource(b.String(), n.Expr)
			require.Len(t, output, 1)
			require.NotNil(t, output[0].Call, "no call detected in: %q ~> %+v", b.String(), output)
			require.Equal(t, name, output[0].Operation)
			require.Equal(t, def.ReturnType, output[0].Returns, "incorrect return type on Source{}")
		})
	}
}

func TestLabelsSourceCallCoverageFail(t *testing.T) {
	n := &parser.PromQLNode{
		Expr: &promParser.Call{
			Func: &promParser.Function{
				Name: "fake_call",
			},
		},
	}
	output := utils.LabelsSource("fake_call()", n.Expr)
	require.Len(t, output, 1)
	require.Nil(t, output[0].Call, "no call should have been detected in fake function")
}

func TestSourceFragment(t *testing.T) {
	type testCaseT struct {
		expr      string
		fragments []string
	}

	testCases := []testCaseT{
		{
			expr:      "1",
			fragments: []string{""},
		},
		{
			expr:      `"foo"`,
			fragments: []string{""},
		},
		{
			expr:      `foo`,
			fragments: []string{"foo"},
		},
		{
			expr:      `foo{job="bar"}`,
			fragments: []string{`foo{job="bar"}`},
		},
		{
			expr:      `rate(foo{job="bar"}[5m])`,
			fragments: []string{`rate(foo{job="bar"}[5m])`},
		},
		{
			expr:      `sum(foo{job="bar"})`,
			fragments: []string{`sum(foo{job="bar"})`},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.expr, func(t *testing.T) {
			n, err := parser.DecodeExpr(tc.expr)
			if err != nil {
				t.Error(err)
				t.FailNow()
			}
			var got []string
			for _, src := range utils.LabelsSource(tc.expr, n.Expr) {
				got = append(got, src.Fragment(tc.expr))
			}
			require.Equal(t, tc.fragments, got)
		})
	}
}
